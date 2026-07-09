# Commercial Preflight Blocker Handoff

## Scope

The strict commercial preflight already reported final release blockers, but the
machine-readable JSON was still a raw failure list. This made the final target
release handoff less actionable because release, platform, database, and target
evidence owners had to infer their next command and acceptance artifact from
each failure label.

## Changes

- Extended `scripts/verify-commercial-preflight.mjs` JSON output with a
  `blockers[]` handoff section while preserving existing `checks[]`,
  `failures[]`, and `warnings[]`.
- Each blocker now carries:
  - `owner`
  - `ownerStatus`
  - `severity`
  - `acceptanceArtifacts`
  - `nextCommands`
  - `handoffNotes`
- Added fixture coverage for successful target-only reports having no blockers.
- Added fixture coverage for corrupt artifact body reports carrying artifact
  collection and digest next commands.
- Added fixture coverage for missing-input reports assigning database, platform,
  release operator, and release evidence owners.
- Updated quality gates so the blocker handoff contract remains guarded.

## Boundary

This is release-handoff hardening only. It does not supply the production
database URL, Kubernetes secret file, target evidence manifest, target artifact
body directory, or final no-skip verifier pass. Those remain external release
inputs.
