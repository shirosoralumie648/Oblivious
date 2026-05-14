---
phase: 06-backend-mainline-integration-hardening
plan: 03
subsystem: relay-chat-agent
tags: [relay, chat, agent, metadata, tool-calls]

requires:
  - phase: 06-01-route-surface-and-auth-guard
    provides: HTTP request/session boundary coverage
  - phase: 06-02-notification-ownership
    provides: User-owned mutation hardening pattern
provides:
  - Relay request metadata propagation through Chat and Agent paths
  - Request ID preservation from HTTP middleware to Relay calls
  - Production Relay fallback policy that fails closed instead of bypassing Relay
  - Structured tool-call preservation tests
affects: [CHAT-06, Phase 7, Phase 8]

tech-stack:
  added: []
  patterns:
    - Context-carried Relay metadata
    - Production fail-closed Relay gateway composition
    - Agent structured tool-call regression coverage

key-files:
  created:
    - src/server/internal/http/agent_handler.go
  modified:
    - src/server/internal/chat/gateway.go
    - src/server/internal/chat/service.go
    - src/server/internal/chat/relay_gateway.go
    - src/server/internal/chat/relay_gateway_test.go
    - src/server/internal/chat/service_test.go
    - src/server/internal/agent/runner.go
    - src/server/internal/agent/service_test.go
    - src/server/internal/http/chat_handler.go
    - src/server/internal/http/middleware.go
    - src/server/internal/http/middleware_test.go
    - src/server/internal/http/router.go

key-decisions:
  - "Chat and Agent calls preserve `UserID`, `WorkspaceID`, and request ID in Relay metadata."
  - "When Relay is enabled in production, the local direct provider fallback is not used."
  - "Structured tool-call execution remains non-streamed for tool loops with final-answer streaming preserved."

patterns-established:
  - "HTTP handlers seed Relay metadata from session and request ID before entering service methods."
  - "Service/runner layers merge session identity into existing metadata instead of replacing upstream request attribution."

requirements-completed: [CHAT-06]
requirements-blocked: []
status: complete

duration: 25min
completed: 2026-05-14
---

# Phase 06 Plan 03: Relay Metadata And Tool-Call Summary

**Chat and Agent model calls now preserve Relay attribution metadata, keep structured tool loops covered, and fail closed in production when Relay has no fallback.**

## Accomplishments

- Added `RelayRequestMetadataFromContext` and metadata merge behavior for Chat and Agent call paths.
- Added HTTP request ID extraction and seeded Relay metadata in Chat and Agent handlers.
- Kept usage recording separate from Relay quota/billing settlement.
- Preserved Agent `RunWithTools`, assistant tool-call persistence, `tool` role messages, and final-answer streaming behavior in tests.
- Updated router composition so production Relay-enabled mode does not silently fall back to direct model provider calls.

## Task Commits

- Relay metadata and tool-call hardening: `ef81374` (`feat(06): harden backend mainline integration`)

## Verification

```bash
cd src/server && go test ./internal/http ./internal/chat ./internal/agent ./internal/relay -count=1
env TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32768/oblivious_test?sslmode=disable' go test ./... -count=1
```

Both commands passed. The final DB-backed full backend run covered `internal/chat`, `internal/agent`, `internal/relay`, and the HTTP request metadata tests.

## Deviations from Plan

None - implementation stayed within the Relay-first Chat/Agent boundary.

## Issues Encountered

None.

## Next Phase Readiness

CHAT-06 is ready for API docs and release-contract reconciliation; frontend code can rely on the same Relay-first behavior for Chat and Agent flows.
