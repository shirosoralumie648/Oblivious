#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
mode="all"
if [[ ${1:-} == "--transport" ]]; then
  mode="transport"
  shift
fi
[[ $# -eq 0 ]] || {
  printf 'frontend_surface_fixture_argument_invalid\n' >&2
  exit 2
}
[[ "$mode" == "transport" || "$mode" == "all" ]] || exit 0

tmp_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-frontend-surface-fixtures.XXXXXX")
cleanup() { rm -rf -- "$tmp_root"; }
trap cleanup EXIT

sidecar="$tmp_root/sidecar.json"
manifest="$tmp_root/manifest.json"
observation="$tmp_root/transport-observation.json"

node "$repo_root/scripts/frontend_surface_sidecar.mjs" \
  --root "$repo_root/scripts/testdata/frontend-surface/production" \
  --tsconfig "$repo_root/scripts/testdata/frontend-surface/production/tsconfig.json" \
  --generated-file "$repo_root/scripts/testdata/frontend-surface/production/generated/client.generated.ts" \
  --output "$sidecar"

python3 - "$sidecar" "$manifest" <<'PY'
import hashlib
import json
from pathlib import Path
import sys

sidecar = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
operations_by_id = {}
browser_events = {}
for row in sidecar["operations"]:
    operation = row["operation"]
    operations_by_id.setdefault(operation["operationId"], operation)
    if row["events"]:
        browser_events.setdefault(operation["operationId"], {
            "operationId": operation["operationId"],
            "transport": "websocket" if row["transport"]["protocol"] == "websocket" else "sse",
            "events": row["events"],
        })
operations = sorted(operations_by_id.values(), key=lambda item: (item["normalizedPath"], item["method"], item["operationId"]))
events = sorted(browser_events.values(), key=lambda item: item["operationId"])
scope = {
    "schemaVersion": "public-operation-scope/v1",
    "mandatoryPrefixes": ["/api/", "/v1/"],
    "dispositions": [
        {
            "method": item["method"],
            "normalizedPath": item["normalizedPath"],
            "disposition": "included",
            "reason": "frontend transport fixture",
        }
        for item in operations
    ],
}
canonical = lambda value: json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
digest = lambda value: "sha256:" + hashlib.sha256(canonical(value)).hexdigest()
manifest = {
    "schemaVersion": "route-surface-manifest/v2",
    "generatedFrom": "docs/api/openapi.yaml",
    "projectionDigest": digest({"scope": scope, "operations": operations}),
    "browserEventDigest": digest(events),
    "scope": scope,
    "operations": operations,
    "browserEvents": events,
    "routeSamples": [],
}
Path(sys.argv[2]).write_text(json.dumps(manifest, sort_keys=True) + "\n", encoding="utf-8")
PY

run_transport() {
  python3 "$repo_root/scripts/verify_frontend_surface.py" transport \
    --sidecar "$1" \
    --manifest "$2" \
    --output "$3"
}

run_transport "$sidecar" "$manifest" "$observation"

python3 - "$observation" <<'PY'
import json
from pathlib import Path
import sys

value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
assert value["schemaVersion"] == "frontend-transport-observation/v1"
assert value["operationCount"] == value["coreCount"] == value["compatibleCount"] == 11
assert value["unresolvedCount"] == 0
assert value["errorCodes"] == []
assert value["skippedChecks"] == []
PY

expect_failure() {
  local label="$1"
  local expected="$2"
  local mutated_sidecar="$tmp_root/${label}-sidecar.json"
  local mutated_manifest="$tmp_root/${label}-manifest.json"
  cp "$sidecar" "$mutated_sidecar"
  cp "$manifest" "$mutated_manifest"
  python3 - "$mutated_sidecar" "$mutated_manifest" "$label" <<'PY'
import json
from pathlib import Path
import sys

sidecar_path, manifest_path, label = map(Path, sys.argv[1:])
sidecar = json.loads(sidecar_path.read_text(encoding="utf-8"))
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
row = sidecar["operations"][0]
stream = next(item for item in sidecar["operations"] if item["transport"]["kind"] == "sse-stream")
socket = next(item for item in sidecar["operations"] if item["transport"]["kind"] == "websocket")
upload = next(item for item in sidecar["operations"] if item["operation"]["operationId"] == "uploadFixture")
raw = next(item for item in sidecar["operations"] if item["operation"]["operationId"] == "rawFixture")
text = next(item for item in sidecar["operations"] if item["operation"]["operationId"] == "textFixture")
no_content = next(item for item in sidecar["operations"] if item["operation"]["operationId"] == "deleteFixture")
if label.name == "core-mismatch":
    row["contract"]["capabilityId"] = "fixture.wrong"
elif label.name == "encoder-mismatch":
    row["requestEncoder"]["id"] = "json"
elif label.name == "form-data-encoder-mismatch":
    upload["requestEncoder"]["id"] = "raw"
elif label.name == "raw-encoder-mismatch":
    raw["requestEncoder"]["id"] = "json"
elif label.name == "json-encoder-mismatch":
    stream["requestEncoder"]["id"] = "raw"
elif label.name == "decoder-mismatch":
    row["responseDecoder"]["id"] = "text"
elif label.name == "text-decoder-mismatch":
    text["responseDecoder"]["id"] = "json-envelope"
elif label.name == "raw-decoder-mismatch":
    raw["responseDecoder"]["id"] = "none"
elif label.name == "none-decoder-mismatch":
    no_content["responseDecoder"]["id"] = "json-envelope"
elif label.name == "success-alternative-mismatch":
    text["responseDecoder"]["status"] = 201
elif label.name == "event-mismatch":
    socket["events"][0]["direction"] = "server"
elif label.name == "media-charset-mismatch":
    stream["responseDecoder"]["mediaType"] = "text/event-stream; charset=latin1"
elif label.name == "unresolved":
    sidecar["unresolved"] = [{"source": row["source"], "code": "fixture_unresolved"}]
elif label.name == "unknown-taxonomy":
    row["transport"]["kind"] = "mystery"
elif label.name == "generated-source":
    row["source"]["file"] = "src/web/src/generated/forged.generated.ts"
elif label.name == "zero-operations":
    sidecar["operations"] = []
elif label.name == "duplicate-source":
    sidecar["operations"].append(row)
elif label.name == "generated-consumer-drift":
    sidecar["generatedConsumers"] -= 1
elif label.name == "manifest-digest":
    manifest["projectionDigest"] = "sha256:" + "0" * 64
else:
    raise SystemExit(f"unknown fixture mutation: {label.name}")
sidecar_path.write_text(json.dumps(sidecar, sort_keys=True) + "\n", encoding="utf-8")
manifest_path.write_text(json.dumps(manifest, sort_keys=True) + "\n", encoding="utf-8")
PY
  local output
  if output=$(run_transport "$mutated_sidecar" "$mutated_manifest" "$tmp_root/${label}-observation.json" 2>&1); then
    printf 'frontend_surface_fixture_failed: %s unexpectedly passed\n' "$label" >&2
    exit 1
  fi
  [[ "$output" == *"$expected"* ]] || {
    printf 'frontend_surface_fixture_failed: %s failed for wrong reason: %s\n' "$label" "$output" >&2
    exit 1
  }
}

expect_failure core-mismatch frontend_core_mismatch
expect_failure encoder-mismatch frontend_request_encoder_incompatible
expect_failure form-data-encoder-mismatch frontend_request_encoder_incompatible
expect_failure raw-encoder-mismatch frontend_request_encoder_incompatible
expect_failure json-encoder-mismatch frontend_request_encoder_incompatible
expect_failure decoder-mismatch frontend_response_decoder_incompatible
expect_failure text-decoder-mismatch frontend_response_decoder_incompatible
expect_failure raw-decoder-mismatch frontend_response_decoder_incompatible
expect_failure none-decoder-mismatch frontend_response_decoder_incompatible
expect_failure success-alternative-mismatch frontend_response_status_incompatible
expect_failure event-mismatch frontend_event_identity_mismatch
expect_failure media-charset-mismatch frontend_response_media_incompatible
expect_failure unresolved frontend_sidecar_unresolved
expect_failure unknown-taxonomy frontend_transport_taxonomy_unknown
expect_failure generated-source frontend_generated_call_classified
expect_failure zero-operations frontend_operation_inventory_empty
expect_failure duplicate-source frontend_source_classification_duplicate
expect_failure generated-consumer-drift frontend_generated_consumer_mismatch
expect_failure manifest-digest frontend_manifest_digest_mismatch

printf '[frontend-surface-fixtures] transport baseline and 20 rejected mutations verified\n'
