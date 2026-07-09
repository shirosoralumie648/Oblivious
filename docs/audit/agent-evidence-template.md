# Agent Evidence: <feature>

Date:

Agent:

Commit:

## Runtime Claim

State the exact behavior that is now implemented in production runtime code.

Do not count docs, mocks, fakes, local-only fallbacks, fixtures, generated OpenAPI, or tests as runtime evidence.

## Reference Inputs

List reference projects, files, and line anchors inspected before implementation.

```text
reference/<project>/<path>:<line> - pattern used
reference/<project>/<path>:<line> - pattern rejected, with reason
```

## Oblivious Files Changed

List only files changed by this agent.

```text
src/server/...
src/web/...
docs/...
scripts/...
```

## Contract Changes

List API, database, configuration, environment variable, billing, request-log, or tenant/RBAC contract changes.

If none, write `None`.

## Verification Commands

Record commands actually run and their result.

```text
command:
result:
```

If a command could not run, record the exact blocker and the environment prerequisite.

## Runtime Evidence IDs

Record IDs that join the runtime claim across systems.

```text
request_id:
trace_id:
organization_id:
workspace_id:
user_id:
usage_id:
billing_session_id:
request_log_id:
agent_run_id:
workflow_execution_id:
knowledge_document_id:
```

Remove fields that do not apply to the feature.

## Failure Evidence

Describe at least one negative path that was verified, such as provider failure, quota failure, client abort, retry exhaustion, tenant denial, unsupported route, or missing production sink.

## Unsupported / Deferred Surfaces

List adjacent surfaces that remain unsupported or intentionally fail closed.

## Known Residual Risk

List remaining risks that must not be represented as commercially complete.
