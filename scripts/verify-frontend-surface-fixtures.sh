#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
mode="all"
case ${1:-} in
  --transport) mode="transport"; shift ;;
  --exposure) mode="exposure"; shift ;;
esac
[[ $# -eq 0 ]] || {
  printf 'frontend_surface_fixture_argument_invalid\n' >&2
  exit 2
}
tmp_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-frontend-surface-fixtures.XXXXXX")
cleanup() { rm -rf -- "$tmp_root"; }
trap cleanup EXIT

sidecar="$tmp_root/sidecar.json"
extractor_invocations=0
node "$repo_root/scripts/frontend_surface_sidecar.mjs" \
  --root "$repo_root/scripts/testdata/frontend-surface/production" \
  --tsconfig "$repo_root/scripts/testdata/frontend-surface/production/tsconfig.json" \
  --generated-file "$repo_root/scripts/testdata/frontend-surface/production/generated/client.generated.ts" \
  --output "$sidecar"
extractor_invocations=$((extractor_invocations + 1))

if [[ "$mode" == "exposure" || "$mode" == "all" ]]; then
  contract="$tmp_root/contract.json"
  app_projection="$tmp_root/app-projection.json"
  server_catalog="$tmp_root/server-catalog.json"
  exposure_observation="$tmp_root/exposure-observation.json"

  python3 - "$contract" "$app_projection" "$server_catalog" <<'PY'
import hashlib
import json
from pathlib import Path
import sys

contract_path, app_path, catalog_path = map(Path, sys.argv[1:])
identity = {
    "sourceTree": "a" * 40,
    "contractDigest": "sha256:" + "b" * 64,
    "deploymentProfile": "monolith",
}
capabilities = [
    {"id": "fixture.conditional", "commitment": "conditional", "reasonCode": "dependency_unproven"},
    {"id": "fixture.excluded", "commitment": "excluded", "reasonCode": "capability_excluded"},
    {"id": "fixture.users", "commitment": "committed"},
]
bindings = [
    {"id": "model.fixture", "subjectKind": "model", "subjectId": "fixture-model", "runtimeClass": "server_model", "capabilityId": "fixture.users"},
    {"id": "tool.fixture", "subjectKind": "tool", "subjectId": "fixture-tool", "runtimeClass": "builtin", "capabilityId": "fixture.conditional"},
]
contract = {
    "schemaVersion": "contract/v1",
    "defaultProfile": "monolith",
    "profiles": [{"id": "monolith", "commitment": "committed", "catalogBindingIds": [row["id"] for row in bindings]}],
    "capabilities": capabilities,
    "catalogBindings": bindings,
    "surfaceReferences": [{
        "id": "frontend",
        "canonicalSource": "scripts/testdata/frontend-surface/production",
        "consumer": "frontend-transport-inventory",
        "capabilityIds": [row["id"] for row in capabilities],
    }],
}
app_capabilities = [
    {"capabilityId": "fixture.conditional", "disposition": "conditional", "availability": "enabled", "enabled": True},
    {"capabilityId": "fixture.users", "disposition": "committed", "availability": "enabled", "enabled": True},
]
payload = {"identity": identity, "generation": 1, "capabilities": app_capabilities}
digest = "sha256:" + hashlib.sha256(json.dumps(payload, separators=(",", ":")).encode()).hexdigest()
app = {
    "schemaVersion": "frontend-app-projection-observation/v1",
    "provenance": {"source": "authenticated-api", "provider": "ReleaseProjectionProvider", "operationId": "getAppReadinessCapabilities"},
    "releaseIdentity": identity,
    "generation": 1,
    "projectionDigest": digest,
    "capabilities": app_capabilities,
}
catalog = {
    "schemaVersion": "frontend-server-catalog-observation/v1",
    "releaseIdentity": identity,
    "subjects": bindings,
}
for path, value in ((contract_path, contract), (app_path, app), (catalog_path, catalog)):
    path.write_text(json.dumps(value, sort_keys=True) + "\n", encoding="utf-8")
PY

  run_exposure() {
    python3 "$repo_root/scripts/verify_frontend_surface.py" exposure \
      --sidecar "$1" \
      --contract "$2" \
      --app-projection "$3" \
      --server-catalog "$4" \
      --output "$5"
  }

  run_exposure "$sidecar" "$contract" "$app_projection" "$server_catalog" "$exposure_observation"

  python3 - "$exposure_observation" <<'PY'
import json
from pathlib import Path
import sys

value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
assert value["schemaVersion"] == "frontend-exposure-observation/v1"
assert set(value) == {
    "schemaVersion", "sidecarDigest", "sourceDigest", "configDigest",
    "exposureCount", "catalogCount", "navigationCount", "generatedConsumerCount",
    "projectionDigest", "unresolvedCount", "errorCodes", "skippedChecks",
}
assert value["navigationCount"] > 0
assert value["catalogCount"] == 2
assert value["generatedConsumerCount"] > 0
assert value["projectionDigest"].startswith("sha256:")
assert value["errorCodes"] == []
assert value["skippedChecks"] == []
PY

  expect_exposure_failure() {
    local label="$1"
    local expected="$2"
    local mutated_sidecar="$tmp_root/${label}-sidecar.json"
    local mutated_contract="$tmp_root/${label}-contract.json"
    local mutated_app="$tmp_root/${label}-app.json"
    local mutated_catalog="$tmp_root/${label}-catalog.json"
    cp "$sidecar" "$mutated_sidecar"
    cp "$contract" "$mutated_contract"
    cp "$app_projection" "$mutated_app"
    cp "$server_catalog" "$mutated_catalog"
    python3 - "$mutated_sidecar" "$mutated_contract" "$mutated_app" "$mutated_catalog" "$label" <<'PY'
import hashlib
import json
from pathlib import Path
import sys

sidecar_path, contract_path, app_path, catalog_path, label_path = map(Path, sys.argv[1:])
label = label_path.name
sidecar = json.loads(sidecar_path.read_text(encoding="utf-8"))
contract = json.loads(contract_path.read_text(encoding="utf-8"))
app = json.loads(app_path.read_text(encoding="utf-8"))
catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
selectors = [row for row in sidecar["exposures"] if row["kind"] == "selector"]
mutations = sidecar["mutationContracts"]
if label == "selector-dto":
    selectors[0]["catalogSubject"] = "UnknownOption.capabilityId"
elif label == "selector-missing":
    sidecar["exposures"] = [row for row in sidecar["exposures"] if row.get("catalogSubject") != "ModelOption.capabilityId"]
elif label == "mutation-capability":
    mutations[0]["fields"].append("capabilityId")
elif label == "mutation-omission-flag":
    mutations[0]["capabilityIdOmitted"] = False
elif label == "provider-prop":
    sidecar["projectionProvider"]["props"].append("projection")
elif label == "provider-auth":
    sidecar["projectionProvider"]["authenticatedStatus"] = "anonymous"
elif label == "identity-splice":
    catalog["releaseIdentity"]["sourceTree"] = "c" * 40
elif label == "runtime-digest":
    app["projectionDigest"] = "sha256:" + "0" * 64
elif label == "excluded-app":
    app["capabilities"].append({"capabilityId": "fixture.excluded", "disposition": "excluded", "availability": "enabled", "enabled": True})
elif label == "excluded-navigation":
    next(row for row in sidecar["exposures"] if row["kind"] == "navigation")["capabilityId"] = "fixture.excluded"
elif label == "catalog-missing":
    catalog["subjects"].pop()
elif label == "catalog-unknown":
    catalog["subjects"][0]["capabilityId"] = "fixture.unknown"
elif label == "catalog-disabled":
    for row in app["capabilities"]:
        row["availability"] = "disabled"
        row["enabled"] = False
elif label == "projection-digest":
    sidecar["releaseProjection"]["digest"] = "sha256:" + "0" * 64
elif label == "client-map":
    sidecar["policyViolations"] = ["client_capability_map"]
elif label == "admin-inventory":
    selectors[0]["catalogSubject"] = "AdminModelOption.capabilityId"
elif label == "zero-exposures":
    sidecar["exposures"] = []
elif label == "zero-navigation":
    sidecar["exposures"] = [row for row in sidecar["exposures"] if row["kind"] != "navigation"]
elif label == "conditional-unguarded":
    sidecar["exposures"] = [row for row in sidecar["exposures"] if row["kind"] != "availability-guard"]
elif label == "generated-consumer-drift":
    sidecar["generatedConsumers"] = 0
else:
    raise SystemExit(f"unknown exposure fixture mutation: {label}")

if label not in {"runtime-digest", "identity-splice"}:
    identity = app["releaseIdentity"]
    ordered_capabilities = [
        {
            "capabilityId": row["capabilityId"],
            "disposition": row["disposition"],
            "availability": row["availability"],
            "enabled": row["enabled"],
        }
        for row in app["capabilities"]
    ]
    payload = {
        "identity": {
            "sourceTree": identity["sourceTree"],
            "contractDigest": identity["contractDigest"],
            "deploymentProfile": identity["deploymentProfile"],
        },
        "generation": app["generation"],
        "capabilities": ordered_capabilities,
    }
    app["projectionDigest"] = "sha256:" + hashlib.sha256(json.dumps(payload, separators=(",", ":")).encode()).hexdigest()

for path, value in ((sidecar_path, sidecar), (contract_path, contract), (app_path, app), (catalog_path, catalog)):
    path.write_text(json.dumps(value, sort_keys=True) + "\n", encoding="utf-8")
PY
    local output
    if output=$(run_exposure "$mutated_sidecar" "$mutated_contract" "$mutated_app" "$mutated_catalog" "$tmp_root/${label}-observation.json" 2>&1); then
      printf 'frontend_exposure_fixture_failed: %s unexpectedly passed\n' "$label" >&2
      exit 1
    fi
    [[ "$output" == *"$expected"* ]] || {
      printf 'frontend_exposure_fixture_failed: %s failed for wrong reason: %s\n' "$label" "$output" >&2
      exit 1
    }
  }

  expect_exposure_failure selector-dto frontend_catalog_selector_identity_invalid
  expect_exposure_failure selector-missing frontend_catalog_selector_identity_invalid
  expect_exposure_failure mutation-capability frontend_mutation_capability_identity
  expect_exposure_failure mutation-omission-flag frontend_mutation_capability_identity
  expect_exposure_failure provider-prop frontend_projection_provider_invalid
  expect_exposure_failure provider-auth frontend_projection_provider_invalid
  expect_exposure_failure identity-splice frontend_exposure_identity_splice
  expect_exposure_failure runtime-digest frontend_app_projection_digest_mismatch
  expect_exposure_failure excluded-app frontend_excluded_capability_exposed
  expect_exposure_failure excluded-navigation frontend_excluded_capability_exposed
  expect_exposure_failure catalog-missing frontend_catalog_inventory_mismatch
  expect_exposure_failure catalog-unknown frontend_catalog_capability_unknown
  expect_exposure_failure catalog-disabled frontend_catalog_selectable_inventory_empty
  expect_exposure_failure projection-digest frontend_release_projection_mismatch
  expect_exposure_failure client-map frontend_client_capability_authority
  expect_exposure_failure admin-inventory frontend_admin_inventory_exposed
  expect_exposure_failure zero-exposures frontend_exposure_inventory_empty
  expect_exposure_failure zero-navigation frontend_navigation_inventory_empty
  expect_exposure_failure conditional-unguarded frontend_conditional_exposure_unguarded
  expect_exposure_failure generated-consumer-drift frontend_generated_consumer_mismatch

  printf '[frontend-surface-fixtures] exposure baseline and 20 rejected mutations verified\n'
  [[ "$mode" == "all" ]] || exit 0
fi

[[ "$mode" == "transport" || "$mode" == "all" ]] || exit 0

manifest="$tmp_root/manifest.json"
transport_observation="$tmp_root/transport-observation.json"

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

run_transport "$sidecar" "$manifest" "$transport_observation"

python3 - "$transport_observation" <<'PY'
import json
from pathlib import Path
import sys

value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
assert value["schemaVersion"] == "frontend-transport-observation/v1"
assert value["operationCount"] == value["coreCount"] == value["compatibleCount"] == 12
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

if [[ "$mode" == "all" ]]; then
  [[ "$extractor_invocations" -eq 1 ]] || {
    printf 'frontend_surface_fixture_failed: extractor invoked %s times\n' "$extractor_invocations" >&2
    exit 1
  }
  identity_repo="$tmp_root/repository"
  git clone --quiet --no-hardlinks "$repo_root" "$identity_repo"
  [[ "$(git -C "$identity_repo" rev-parse 'HEAD^{tree}')" == "$(git -C "$repo_root" rev-parse 'HEAD^{tree}')" ]] || {
    printf 'frontend_surface_fixture_failed: identity tree splice\n' >&2
    exit 1
  }
  sidecar_digest=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["sidecarDigest"])' "$transport_observation")
  exposure_sidecar_digest=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["sidecarDigest"])' "$exposure_observation")
  [[ "$sidecar_digest" == "$exposure_sidecar_digest" ]] || {
    printf 'frontend_surface_fixture_failed: observation sidecar splice\n' >&2
    exit 1
  }

  run_report() {
    local command="$1"
    local observation_path="$2"
    local digest="$3"
    local report_path="$4"
    shift 4
    (
      cd "$repo_root/src/server"
      go run ./cmd/release-contract "$command" \
        --repo "$identity_repo" \
        --contract config/release/contract.v1.json \
        --schema config/release/contract.schema.json \
        --profile monolith \
        --observation "$observation_path" \
        --sidecar-digest "$digest" \
        --output "$report_path" \
        "$@"
    )
  }

  transport_report="$tmp_root/frontend-transport-report.json"
  exposure_report="$tmp_root/frontend-exposure-report.json"
  run_report report-frontend-transport "$transport_observation" "$sidecar_digest" "$transport_report"
  run_report report-frontend-exposure "$exposure_observation" "$sidecar_digest" "$exposure_report"

  python3 - "$sidecar" "$transport_report" "$exposure_report" <<'PY'
import hashlib
import json
from pathlib import Path
import sys

sidecar_path, transport_path, exposure_path = map(Path, sys.argv[1:])
transport = json.loads(transport_path.read_text(encoding="utf-8"))
exposure = json.loads(exposure_path.read_text(encoding="utf-8"))
digest = "sha256:" + hashlib.sha256(sidecar_path.read_bytes()).hexdigest()
assert transport["surfaceIdentity"]["surface"] == "frontend-transport"
assert exposure["surfaceIdentity"]["surface"] == "frontend-exposure"
assert transport["surfaceIdentity"] != exposure["surfaceIdentity"]
assert transport["releaseIdentity"] == exposure["releaseIdentity"]
assert transport["evidence"]["details"]["sidecarDigest"] == digest
assert exposure["evidence"]["details"]["sidecarDigest"] == digest
PY

  expect_report_failure() {
    local label="$1"
    shift
    local output
    if output=$("$@" 2>&1); then
      printf 'frontend_surface_fixture_failed: %s unexpectedly passed\n' "$label" >&2
      exit 1
    fi
  }
  expect_report_failure sidecar-digest-splice run_report report-frontend-transport "$transport_observation" "sha256:$(printf 'f%.0s' {1..64})" "$tmp_root/spliced-report.json"
  expect_report_failure identity-override run_report report-frontend-transport "$transport_observation" "$sidecar_digest" "$tmp_root/identity-report.json" --source-tree "$(printf 'f%.0s' {1..40})"
  mkdir "$tmp_root/unwritable-report"
  expect_report_failure output-failure run_report report-frontend-transport "$transport_observation" "$sidecar_digest" "$tmp_root/unwritable-report"

  python3 - "$transport_report" "$tmp_root/folded-report.json" "$tmp_root/drift-report.json" "$tmp_root/skip-report.json" <<'PY'
import copy
import json
from pathlib import Path
import sys

source, folded_path, drift_path, skip_path = map(Path, sys.argv[1:])
report = json.loads(source.read_text(encoding="utf-8"))
folded = copy.deepcopy(report)
folded["surfaceIdentity"]["surface"] = "frontend-exposure"
drift = copy.deepcopy(report)
drift["drift"]["missing"] = ["frontend.fixture"]
skipped = copy.deepcopy(report)
skipped["outcome"]["skippedChecks"] = ["frontend.fixture"]
for path, value in ((folded_path, folded), (drift_path, drift), (skip_path, skipped)):
    path.write_text(json.dumps(value, sort_keys=True) + "\n", encoding="utf-8")
PY
  for mutation in folded drift skip; do
    expect_report_failure "report-$mutation" bash -c "cd '$repo_root/src/server' && go run ./cmd/release-contract verify-report --input '$tmp_root/${mutation}-report.json'"
  done

  printf '[frontend-surface-fixtures] combined one-sidecar report pair and report mutations verified\n'
fi
