# Target Release Evidence Workdir Initializer

## Scope

This pass closes an operator workflow gap in the final commercial release path:
target/live evidence already had strict collectors, assembly, digest, and
verification gates, but the release operator still had to manually create the
external working directory and remember which files are raw proof, downloaded
artifact bodies, logs, or environment inputs.

## Changes

- Added `scripts/init-target-release-evidence-workdir.sh`.
- Added `pnpm init:target-release:evidence` as a package script alias.
- Updated `docs/release/rc-checklist.md` to make the initializer the first
  external evidence workspace step.
- Updated `scripts/verify-quality-gates.sh` so the initializer, package alias,
  and runbook language remain guarded.

The initializer creates only:

- `raw/`
- `artifacts/`
- `logs/`
- `.env.example`
- `README.md`
- `collect-target-evidence.todo.md`

It refuses repository-internal workdirs and does not create target manifests,
artifact bodies, proof JSON, secrets, or digest values.

## Verification

- `bash -n scripts/init-target-release-evidence-workdir.sh scripts/verify-quality-gates.sh`
- `bash scripts/init-target-release-evidence-workdir.sh --help`
- `bash scripts/init-target-release-evidence-workdir.sh --workdir /tmp/oblivious-target-evidence-init.XXXXXX/release`
- Verified the generated workspace contains `raw/`, `artifacts/`, `logs/`,
  `.env.example`, `README.md`, and `collect-target-evidence.todo.md`.
- Verified the generated workspace does not contain
  `target-release-evidence.json`.
- Verified the generated `artifacts/` directory is empty.
- Verified a repository-internal workdir path is rejected.
- `git diff --check`

## Boundary

This is release-operations hardening only. It makes the target evidence
collection workflow reproducible, but it does not provide production target
proof, provider live rails, Kubernetes proof, target artifact bodies, canonical
digest refresh, or the final no-skip `scripts/verify-commercial-completion.sh`
run required for a commercial release claim.
