# Relay Files Delete Tombstone Lifecycle

Date: 2026-07-04

## Scope

Closes the local Relay Files delete/tombstone lifecycle gap identified in `2026-07-02-relay-files-current-runtime-proof.md`.

## Findings

- `DELETE /v1/files/:id` now performs tenant-scoped local mapping lookup before upstream deletion.
- Successful upstream delete responses tombstone the local mapping through `TombstoneFileMapping` instead of leaving stale local IDs visible.
- SQL get/list mapping reads exclude `deleted_at IS NOT NULL`, so deleted files are hidden from subsequent tenant list/get/content passthrough.
- Tombstone writes are scoped by `local_file_id`, `user_id`, and `organization_id`, preserving tenant isolation and failing closed on mismatched ownership.
- Schema change is appended as migration `0095_relay_file_mapping_tombstones.sql` to avoid mutating the checksum of already-applied migration `0061_relay_file_mappings.sql`.

## Verification

```text
command: cd src/server && go test ./internal/relay/handler -run TestFilesDeleteTombstonesTenantMappingAfterUpstreamSuccess -count=1 -v
result: pass
evidence: regression failed before implementation with empty tombstone evidence; passed after handler tombstone path was added.

command: cd src/server && go test ./internal/relay -run 'TestRelayStore(TombstoneFileMappingHidesTenantMapping|GetFileMappingRequiresTenantOwnership|ListFileMappingsRequiresTenantOwnership)' -count=1 -v
result: pass with DB-backed cases skipped in this environment because TEST_DATABASE_URL is not set.

command: cd src/server && go test ./internal/relay/handler ./internal/relay -count=1 -timeout 300s
result: pass
```

## Remaining Boundary

- Target-runtime provider reachability and tenant-isolation proof are still required before claiming complete Files API lifecycle readiness.
- DB-backed tombstone behavior should be re-run with `TEST_DATABASE_URL` in the release evidence environment.
