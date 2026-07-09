#!/usr/bin/env python3
import argparse
import copy
import hashlib
import json
import pathlib
import sys


ZERO_SHA256 = "0" * 64
DIGEST_FIELDS = ("targetEvidenceSha256", "artifactBundleSha256")
RESULT_SCHEMA = "oblivious-target-release-digests-v1"
BUNDLE_SCHEMA = "oblivious-target-artifact-bundle-digest-v1"


def fail(message):
    raise SystemExit(f"[target-release-digests] {message}")


def canonical_json_bytes(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n").encode("utf-8")


def sha256_bytes(value):
    return hashlib.sha256(value).hexdigest()


def load_json(path):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        fail(f"file not found: {path}")
    except json.JSONDecodeError as exc:
        fail(f"{path} is not valid JSON: {exc}")


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def artifacts_by_id(manifest):
    artifacts = manifest.get("artifacts")
    if not isinstance(artifacts, list) or not artifacts:
        fail("manifest artifacts must be a non-empty array")

    by_id = {}
    for index, artifact in enumerate(artifacts):
        if not isinstance(artifact, dict):
            fail(f"artifacts[{index}] must be an object")
        artifact_id = artifact.get("id")
        if not isinstance(artifact_id, str) or not artifact_id.strip():
            fail(f"artifacts[{index}].id must be a non-empty string")
        if artifact_id in by_id:
            fail(f"duplicate artifact id: {artifact_id}")
        by_id[artifact_id] = artifact
    return by_id


def strict_artifact_id(manifest, by_id):
    strict = manifest.get("strictVerifier")
    if not isinstance(strict, dict):
        fail("strictVerifier must be an object")

    evidence_ref = strict.get("evidenceRef")
    if isinstance(evidence_ref, str) and evidence_ref.strip():
        artifact = by_id.get(evidence_ref)
        if artifact is None:
            fail(f"strictVerifier.evidenceRef does not resolve to an artifact: {evidence_ref}")
        if artifact.get("kind") != "strict-verifier-log":
            fail("strictVerifier.evidenceRef must reference artifact kind strict-verifier-log")
        return evidence_ref

    strict_ids = [artifact_id for artifact_id, artifact in by_id.items() if artifact.get("kind") == "strict-verifier-log"]
    if len(strict_ids) != 1:
        fail("strictVerifier.evidenceRef must identify exactly one strict-verifier-log artifact")
    return strict_ids[0]


def normalize_strict_digest_fields(value):
    normalized = copy.deepcopy(value)
    if not isinstance(normalized, dict):
        return normalized
    for field in DIGEST_FIELDS:
        normalized[field] = ZERO_SHA256
    return normalized


def read_artifact_bodies(manifest, artifact_dir, strict_id):
    artifact_dir = pathlib.Path(artifact_dir)
    if not artifact_dir.is_dir():
        fail(f"artifact directory not found: {artifact_dir}")

    by_id = artifacts_by_id(manifest)
    body_infos = {}
    for artifact_id in sorted(by_id):
        artifact = by_id[artifact_id]
        path = artifact_dir / f"{artifact_id}.json"
        if not path.is_file():
            fail(f"artifact body not found: {path}")
        raw_body = path.read_bytes()
        raw_sha256 = sha256_bytes(raw_body)
        try:
            body = json.loads(raw_body.decode("utf-8"))
        except UnicodeDecodeError:
            fail(f"artifact body is not UTF-8 JSON: {path}")
        except json.JSONDecodeError as exc:
            fail(f"artifact body is not valid JSON: {path}: {exc}")

        digest_body = body
        if artifact_id == strict_id:
            digest_body = normalize_strict_digest_fields(body)
        body_infos[artifact_id] = {
            "artifact": artifact,
            "body": body,
            "path": path,
            "rawSha256": raw_sha256,
            "canonicalBodySha256": sha256_bytes(canonical_json_bytes(digest_body)),
        }
    return body_infos


def normalized_manifest(manifest, body_infos, strict_id):
    normalized = copy.deepcopy(manifest)
    strict = normalized.get("strictVerifier")
    if not isinstance(strict, dict):
        fail("strictVerifier must be an object")
    for field in DIGEST_FIELDS:
        strict[field] = ZERO_SHA256

    artifacts = normalized.get("artifacts")
    if not isinstance(artifacts, list):
        fail("manifest artifacts must be an array")
    for artifact in artifacts:
        if not isinstance(artifact, dict):
            continue
        artifact_id = artifact.get("id")
        if artifact_id == strict_id:
            artifact["sha256"] = ZERO_SHA256
        elif artifact_id in body_infos:
            artifact["sha256"] = body_infos[artifact_id]["rawSha256"]
    return normalized


def artifact_bundle_payload(body_infos):
    artifacts = []
    for artifact_id in sorted(body_infos):
        artifact = body_infos[artifact_id]["artifact"]
        entry = {
            "id": artifact_id,
            "kind": artifact.get("kind"),
            "bodySha256": body_infos[artifact_id]["canonicalBodySha256"],
        }
        if "provider" in artifact:
            entry["provider"] = artifact.get("provider")
        artifacts.append(entry)
    return {"schema": BUNDLE_SCHEMA, "artifacts": artifacts}


def compute(manifest, artifact_dir):
    by_id = artifacts_by_id(manifest)
    strict_id = strict_artifact_id(manifest, by_id)
    body_infos = read_artifact_bodies(manifest, artifact_dir, strict_id)
    manifest_digest_body = normalized_manifest(manifest, body_infos, strict_id)
    bundle_payload = artifact_bundle_payload(body_infos)
    return {
        "strictArtifactId": strict_id,
        "bodyInfos": body_infos,
        "targetEvidenceSha256": sha256_bytes(canonical_json_bytes(manifest_digest_body)),
        "artifactBundleSha256": sha256_bytes(canonical_json_bytes(bundle_payload)),
    }


def write_refreshed_manifest(manifest_path, manifest, computation):
    refreshed = copy.deepcopy(manifest)
    strict = refreshed.get("strictVerifier")
    if not isinstance(strict, dict):
        fail("strictVerifier must be an object")
    strict["targetEvidenceSha256"] = computation["targetEvidenceSha256"]
    strict["artifactBundleSha256"] = computation["artifactBundleSha256"]

    strict_id = computation["strictArtifactId"]
    strict_info = computation["bodyInfos"][strict_id]
    strict_body = copy.deepcopy(strict_info["body"])
    strict_body["targetEvidenceSha256"] = computation["targetEvidenceSha256"]
    strict_body["artifactBundleSha256"] = computation["artifactBundleSha256"]
    strict_body_bytes = canonical_json_bytes(strict_body)
    strict_info["path"].write_bytes(strict_body_bytes)
    strict_raw_sha256 = sha256_bytes(strict_body_bytes)

    for artifact in refreshed.get("artifacts", []):
        if not isinstance(artifact, dict):
            continue
        artifact_id = artifact.get("id")
        if artifact_id == strict_id:
            artifact["sha256"] = strict_raw_sha256
        elif artifact_id in computation["bodyInfos"]:
            artifact["sha256"] = computation["bodyInfos"][artifact_id]["rawSha256"]

    write_json(manifest_path, refreshed)


def result_payload(computation, write_enabled):
    return {
        "schema": RESULT_SCHEMA,
        "targetEvidenceSha256": computation["targetEvidenceSha256"],
        "artifactBundleSha256": computation["artifactBundleSha256"],
        "strictArtifactId": computation["strictArtifactId"],
        "artifactCount": len(computation["bodyInfos"]),
        "updated": write_enabled,
        "targetEvidenceDigest": {
            "normalizedFields": [
                "strictVerifier.targetEvidenceSha256",
                "strictVerifier.artifactBundleSha256",
                "artifacts[<strict-verifier-log>].sha256",
            ],
            "nonStrictArtifactSha256Source": "artifact body raw SHA-256 from --artifact-dir",
        },
        "artifactBundleDigest": {
            "artifactOrder": "sorted by artifact id",
            "strictVerifierBodyNormalizedFields": list(DIGEST_FIELDS),
            "bodyDigestFormat": "canonical JSON with sorted object keys and trailing newline",
        },
    }


def parse_args():
    parser = argparse.ArgumentParser(description="Compute canonical target release evidence digest fields.")
    parser.add_argument("manifest_arg", nargs="?", help="target release evidence manifest")
    parser.add_argument("--manifest", help="target release evidence manifest")
    parser.add_argument("--artifact-dir", required=True, help="directory containing <artifact-id>.json bodies")
    parser.add_argument("--write", action="store_true", help="write digest fields back to manifest and strict artifact body")
    parser.add_argument("--output", help="write digest result JSON to this file instead of stdout")
    args = parser.parse_args()
    manifest = args.manifest or args.manifest_arg
    if not manifest:
        fail("--manifest or manifest argument is required")
    args.manifest_path = pathlib.Path(manifest)
    args.artifact_dir_path = pathlib.Path(args.artifact_dir)
    return args


def main():
    args = parse_args()
    manifest = load_json(args.manifest_path)
    computation = compute(manifest, args.artifact_dir_path)
    if args.write:
        write_refreshed_manifest(args.manifest_path, manifest, computation)
    payload = result_payload(computation, args.write)
    output = json.dumps(payload, indent=2, ensure_ascii=False) + "\n"
    if args.output:
        pathlib.Path(args.output).write_text(output, encoding="utf-8")
    else:
        sys.stdout.write(output)


if __name__ == "__main__":
    main()
