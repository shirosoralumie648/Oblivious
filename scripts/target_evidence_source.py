import json
import os
import pathlib
import re
from datetime import datetime
from urllib import parse, request


SECRET_QUERY_RE = re.compile(r"(token|password|passwd|pass|secret|api[_-]?key|signature|session)", re.IGNORECASE)


def unquote_nested(value):
    previous = value
    for _ in range(3):
        current = parse.unquote_plus(previous)
        if current == previous:
            return current
        previous = current
    return previous


def add_proof_source_args(parser):
    proof_source = parser.add_mutually_exclusive_group(required=True)
    proof_source.add_argument("--proof-file")
    proof_source.add_argument("--proof-url")
    parser.add_argument("--bearer-token-env", default="OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN")
    parser.add_argument("--bearer-token-file")
    parser.add_argument("--cookie-file")
    parser.add_argument("--timeout-seconds", type=float, default=10.0)


def require_url(value, name, fail):
    if not isinstance(value, str) or value.strip() == "":
        fail(f"{name} is required")
    value = value.strip()
    parsed = parse.urlsplit(value)
    if parsed.scheme not in ("http", "https") or not parsed.netloc:
        fail(f"{name} must be an HTTP(S) URL")
    if parsed.username or parsed.password:
        fail(f"{name} must not embed credentials in URI userinfo")
    for component in (parsed.query, parsed.fragment):
        for key, _ in parse.parse_qsl(component, keep_blank_values=True):
            if SECRET_QUERY_RE.search(unquote_nested(key)):
                if name == "proof-url":
                    fail("proof-url must not carry secret-like query parameters")
                fail(f"{name} must not carry secret-like query parameters")
        if component and "=" not in component and SECRET_QUERY_RE.search(unquote_nested(component)):
            if name == "proof-url":
                fail("proof-url must not carry secret-like query parameters")
            fail(f"{name} must not carry secret-like query parameters")
    return value


def sanitized_url(value, name, fail):
    value = require_url(value, name, fail)
    parsed = parse.urlsplit(value)
    return parse.urlunsplit((parsed.scheme, parsed.netloc, parsed.path, parsed.query, ""))


def require_collected_at(value, fail):
    if not isinstance(value, str) or value.strip() == "":
        fail("recorded-at is required")
    value = value.strip()
    try:
        datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        fail("recorded-at must be ISO-8601")
    return value


def proof_collection_source(args, fail):
    collected_at = require_collected_at(args.recorded_at, fail)
    if args.proof_file:
        return {"type": "file", "collectedAt": collected_at}
    return {
        "type": "target-url",
        "url": sanitized_url(args.proof_url, "proof-url", fail),
        "collectedAt": collected_at,
    }


def read_secret_file(path_label, path, fail):
    try:
        return pathlib.Path(path).read_text(encoding="utf-8").strip()
    except FileNotFoundError:
        fail(f"{path_label} file is required: {path}")


def request_headers(args, fail):
    headers = {"Accept": "application/json"}
    bearer_token = ""
    if args.bearer_token_file:
        bearer_token = read_secret_file("bearer-token", args.bearer_token_file, fail)
    elif args.bearer_token_env:
        bearer_token = os.environ.get(args.bearer_token_env, "").strip()
    if bearer_token:
        headers["Authorization"] = f"Bearer {bearer_token}"
    if args.cookie_file:
        cookie_value = read_secret_file("cookie", args.cookie_file, fail)
        if cookie_value:
            headers["Cookie"] = cookie_value
    return headers


def read_json_url(label, url, args, fail, unwrap_keys=()):
    if args.timeout_seconds <= 0:
        fail("timeout-seconds must be positive")
    req = request.Request(url, headers=request_headers(args, fail), method="GET")
    try:
        with request.urlopen(req, timeout=args.timeout_seconds) as response:
            status = getattr(response, "status", response.getcode())
            body = response.read(2 * 1024 * 1024 + 1)
    except Exception as error:
        fail(f"{label} fetch failed: {error}")
    if status < 200 or status >= 300:
        fail(f"{label} fetch returned HTTP {status}")
    if len(body) > 2 * 1024 * 1024:
        fail(f"{label} response exceeded 2MiB")
    try:
        payload = json.loads(body.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        fail(f"{label} response must be JSON: {error}")
    if not isinstance(payload, dict):
        fail(f"{label} response must be a JSON object")
    data = payload.get("data")
    if isinstance(data, dict) and any(key in data for key in unwrap_keys):
        return data
    return payload


def read_proof(args, read_json, fail, unwrap_keys=()):
    if args.proof_file:
        return read_json("proof", args.proof_file)
    proof_url = require_url(args.proof_url, "proof-url", fail)
    return read_json_url("proof", proof_url, args, fail, unwrap_keys)
