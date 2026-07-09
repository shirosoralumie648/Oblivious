# Relay Files Current Runtime Proof

Date: 2026-07-02

## Scope

Refreshes the Relay Files commercial gap assessment after verifying the active runtime path. This evidence does not claim target-environment provider proof or full delete/tombstone lifecycle completion.

## Findings

- Active Relay construction wires `handler.NewFilesHandler(...).WithMappingStore(cfg.FilesMappingStore)` in `src/server/internal/relay/relay.go`.
- Active Files upload/list/get/delete/content routes use `src/server/internal/relay/handler/files.go`, not the stale alternate `handler_new/files.go` TODO path.
- Upload requires a configured mapping store and trusted tenant identity, routes through Relay billing, stores local/provider file IDs, and returns a local file ID plus `provider_file_id`.
- List/get/delete/content require tenant-scoped mapping lookup/list capability before upstream dispatch. List responses are filtered to current-tenant provider IDs and rewritten to local file IDs.
- SQL Relay store implements save/get/list tenant ownership for `relay_file_mappings`.

## Verification

```text
command: go test ./internal/relay/handler -run "TestFiles" -count=1
result: pass

command: go test ./internal/relay -run "TestRelayStore.*FileMapping|TestNewRelay.*File|TestRelay.*File" -count=1
result: pass
```

## Remaining Boundary

- Target-runtime provider reachability and tenant-isolation proof are still required.
- Delete/tombstone lifecycle audit and reconciliation evidence remain open before claiming complete Files API lifecycle readiness.
- Stale `handler_new/files.go` TODOs must not be counted as active runtime blockers or active runtime evidence.
