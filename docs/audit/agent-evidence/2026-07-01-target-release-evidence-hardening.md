# Target Release Evidence Hardening

Date: 2026-07-01

## Scope

This slice tightens the final commercial-release evidence contract so target readiness cannot be claimed from generic Kubernetes/provider pass rows alone.

## Changes

- Added required target evidence sections in `scripts/verify-target-release-evidence.sh`:
  - `requestLogObservability`
  - `ragIndexing`
  - `marketplacePayouts`
  - `providerRuntimeConfig`
  - `microserviceDatabases`
- Added matching artifact-kind checks for:
  - `request-log-observability`
  - `rag-indexing-proof`
  - `marketplace-payout-proof`
  - `provider-runtime-config`
  - `microservice-database-proof`
- Extended target evidence fixtures with positive manifest coverage and negative cases for missing/failed ClickHouse, RAG worker, marketplace payout webhook, provider runtime config, and microservice database proof.
- Expanded service database URL coverage across local env examples, Kubernetes Secret templates, deployment contract checks, target manifest proof, and `pkg/config` service loaders.
- Added quality-gate tracked-file assertions for release-owned new files so a release checkout cannot silently miss ClickHouse, RAG index jobs, payout webhook/provider, or provider-scoped webhook migration files.

## Verification

- `git diff --check` passed.
- `go test ./pkg/config -count=1` could not run because `go` is not installed in this environment.
- `bash -n scripts/verify-target-release-evidence.sh scripts/verify-target-release-evidence-fixtures.sh` passed under Git Bash.
- `python -m py_compile scripts/verify_target_release_evidence.py scripts/target_release_fixture_mutations.py scripts/verify_deployment_operations_contract.py` passed.
- `bash scripts/verify-target-release-evidence-fixtures.sh` passed after moving the target evidence verifier and fixture mutations off Ruby.
- `bash scripts/verify-deployment-operations-contract.sh` passed after moving the deployment operations contract off Ruby.
- `bash scripts/verify-fusion-evidence-pack.sh` passed.

## Remaining Release Blockers

- Release-owned new files must be added to version control before `scripts/verify-quality-gates.sh` can pass with the new tracked-file assertions.
- Strict final readiness still requires target Kubernetes, live provider rails, target secret audit, ClickHouse request-log proof, RAG worker proof, marketplace payout webhook proof, microservice database proof, workflow telemetry, backup/restore, and strict commercial verifier artifacts from the target environment.
