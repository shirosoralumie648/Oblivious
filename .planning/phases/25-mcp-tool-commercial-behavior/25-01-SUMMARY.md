# Phase 25 Summary: MCP Tool Commercial Behavior

## Status

Complete.

## Closed Requirement

- `PROD-01`: Built-in MCP tools either use real providers/parsers or are disabled from default commercial use.

## What Changed

- Added a default commercial builtin policy in `src/server/internal/mcp/builtin.go`.
- Replaced `calculator` placeholder behavior with a bounded arithmetic parser.
- Kept `datetime` as an enabled real builtin with RFC3339 output tests.
- Disabled default `web_search` until a real search provider is configured.
- Disabled default `http_request` until a tenant-safe outbound HTTP policy is configured, with tests proving disabled default mode performs no outbound network I/O.
- Filtered disabled built-ins from Agent tool definitions and `ListAvailableTools`.
- Rejected disabled built-in execution before calling tool implementation.
- Updated API/commercial-gate docs and quality gates for the Phase 25 policy.

## Verification

- `cd src/server && go test ./internal/mcp ./internal/agent -run 'Builtin|Commercial|Tool|Calculator|WebSearch|HTTPRequest|Disabled' -count=1`
- `bash scripts/check.sh docs`
- `git diff --check`

All passed on 2026-05-28.

## Boundary

This does not complete v08 Product Completeness or final commercial readiness. Phase 26 Durable Agent Workflows is the next required phase.
