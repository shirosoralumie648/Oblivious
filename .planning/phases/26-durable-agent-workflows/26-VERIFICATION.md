# Phase 26 Verification — Durable Agent Workflows

## Scope

Phase 26 closes only `PROD-02`: Agent workflows are durable and observable with persisted run/tool-run state, approval points, memory evidence, failure state, retry state, status APIs, and tenant-scoped access.

Phase 27 Knowledge Product Promise Alignment, Phase 28 commercial UX hardening, Phase 29 public docs/onboarding/pricing/operator guides, Phase 30 end-to-end commercial journey, and the final commercial audit remain required before final commercial readiness.

## Evidence

### Focused Agent Store And Runner Tests

Command:

```bash
cd src/server && TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable' OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/agent ./internal/http -run 'AgentRun|ToolRun|Approval|Retry|MemoryEvidence|Tenant|Durable' -count=1
```

Result: PASS.

Evidence covered:
- `agent_runs` and `agent_tool_runs` schema/store lifecycle.
- Cross-tenant run/tool-run access rejection.
- Successful tool loop durable run/tool-run completion.
- Failed tool execution durable failed state and error text.
- Approval-required tool pause before executor call.
- Memory search evidence on Agent runs.
- Tenant-scoped HTTP list/detail/approve/reject/retry endpoints.
- Retry attempt evidence for failed tool runs and rejection of retry for non-failed tool runs.

### Docs And Gate Checks

Command:

```bash
bash scripts/check.sh docs
```

Result: PASS.

Command:

```bash
git diff --check
```

Result: PASS.

## Boundary

`PROD-02` is complete. The Product Completeness Gate and final commercial readiness remain open until Phase 27 through Phase 30 and `AUDIT-01` are complete.
