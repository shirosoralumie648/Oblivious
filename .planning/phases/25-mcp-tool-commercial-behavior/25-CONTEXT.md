# Phase 25 Context: MCP Tool Commercial Behavior

## Milestone

v08 Product Completeness.

## Why This Phase Exists

v08 exists to remove MVP and placeholder behavior from the customer-facing commercial platform. `PROD-01` is the first product-completeness requirement because built-in MCP tools are currently available to Agent execution without a commercial behavior policy.

The current code exposes four built-ins from `src/server/internal/mcp/builtin.go`: `web_search`, `calculator`, `datetime`, and `http_request`. Two of them are explicit placeholders:

- `web_search` returns `Search results for: ... (placeholder - integrate with search API)`.
- `calculator` returns `Result of ... = (placeholder - implement expression parser)`.

`datetime` returns a real timestamp. `http_request` makes a real outbound HTTP request, but it is default-available as a builtin and lacks tenant-safe outbound policy, allowlisting, response-size limits, audit semantics, or product copy explaining the risk boundary.

The Agent executor uses `mcp.BuiltinTools` directly through `src/server/internal/agent/executor.go`. If an Agent has an enabled builtin tool, `GetToolDefinitions` surfaces it to the model and `Execute` runs it. That means placeholder or unsafe default built-ins can become customer-facing behavior.

## Requirement

- **PROD-01:** Built-in MCP tools such as `web_search`, `calculator`, `datetime`, and `http_request` either use real providers/parsers with tenant-safe configuration and tests, or are disabled from default commercial use with product copy that reflects the disabled state.

## Current Evidence And Gaps

Current evidence:

- `src/server/internal/mcp/builtin.go` has a single global `BuiltinTools` map and no commercial availability metadata.
- `src/server/internal/agent/executor.go` reads that map directly and does not distinguish enabled, configured, disabled, provider-backed, or tenant-safe built-ins.
- `src/server/internal/agent/service_test.go` already proves enabled builtin tools can be executed through the Agent tool loop.
- `docs/release/v07-operations-evidence.md` explicitly leaves real or disabled built-in MCP tools for v08.

Current gaps:

- No tests fail on customer-facing placeholder output from built-ins.
- No registry policy prevents placeholder tools from being surfaced in commercial Agent tool definitions.
- `calculator` has no parser.
- `web_search` has no provider-backed implementation and no disabled-by-default policy.
- `http_request` can perform outbound network I/O without a commercial safety policy.
- Product docs do not yet describe which built-ins are available by default and which are disabled pending provider or outbound-policy configuration.
- Quality gates do not yet guard against enabled default builtin placeholder output returning to customers.

## Design Direction

Phase 25 should use the conservative commercial default:

- Make `calculator` real for a deliberately small arithmetic grammar: decimal numbers, parentheses, unary signs, `+`, `-`, `*`, `/`, exponentiation only if implemented safely, whitespace, and deterministic error messages. Reject unsupported tokens and division by zero.
- Keep `datetime` enabled and real. Add tests around non-empty RFC3339 output and optional timezone handling only if the implementation adds it.
- Disable `web_search` by default unless a real provider integration is configured. Because no search provider contract exists in the current repo, Phase 25 should not fake search results.
- Disable `http_request` by default for commercial Agent built-ins unless an explicit tenant-safe outbound policy exists. Because SSRF/egress policy is bigger than `PROD-01`, Phase 25 should not expose raw HTTP by default.
- Add availability metadata so Agent tool definitions and execution share the same decision, instead of relying on comments or docs.
- Add docs and quality-gate evidence that default-enabled built-ins cannot return placeholder strings.

## Expected Code Areas

- `src/server/internal/mcp/builtin.go`: builtin registry, availability metadata, calculator parser, disabled tool result/error semantics.
- `src/server/internal/mcp/builtin_test.go`: focused unit tests for registry defaults, calculator, datetime, web search disabled default, HTTP request disabled default, and placeholder-output guard.
- `src/server/internal/agent/executor.go`: filter disabled commercial built-ins from tool definitions and block execution even if stale Agent config still says enabled.
- `src/server/internal/agent/service_test.go` or a new executor-focused test: proves disabled builtin tools are not surfaced/executable through Agent behavior.
- `docs/release/commercial-gates.md` and `docs/API.md`: document built-in MCP commercial behavior.
- `scripts/verify-quality-gates.sh`: add a docs gate or source gate proving placeholder MCP built-ins are not enabled by default.

## Verification Design

Phase 25 must prove behavior, not just edit copy:

- Unit tests must fail before implementation when enabled placeholder built-ins are visible or executable.
- Calculator tests must assert exact results for representative expressions and exact errors for malformed expressions.
- Disabled `web_search` and `http_request` tests must assert no network/provider call is attempted by default.
- Agent executor tests must prove disabled built-ins are filtered from OpenAI tool definitions and rejected during execution.
- Docs checks must prove customer-facing docs match default behavior.
- `git diff --check` must pass.

## Closeout Boundary

Phase 25 may close only `PROD-01`.

It must not claim:

- Durable Agent workflow state, approval points, retry state, or execution observability (`PROD-02`).
- Knowledge RAG/source citation or product-copy alignment (`PROD-03`).
- Full Chat/Admin/Marketplace UX hardening (`PROD-04`).
- Public onboarding, pricing, and operator guide completion (`PROD-05`).
- End-to-end commercial journeys or final commercial completion audit (`PROD-06`, `AUDIT-01`).

---

*Phase: 25-mcp-tool-commercial-behavior*
*Context gathered: 2026-05-28 from current repository evidence*
