# Commercial Final Contract Freeze Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish one deterministic HTTP, gRPC, event, generated-client, and database contract system for all 17 services, then publish immutable F0 and F1 compatibility baselines.

**Architecture:** Split OpenAPI into service-owned sources with a generated bundle, consolidate Proto into one authority, introduce shared request/error/event semantics, and enforce database-per-service ownership with contract tests. Shared contract paths are single-writer lease domains; service contracts are frozen in dependency order and consumed through generated clients.

**Tech Stack:** OpenAPI 3, Python bundling and verification, Protobuf 25.1, `protoc-gen-go` 1.36.11, `protoc-gen-go-grpc` 1.6.2, Go contract tests, Kafka, PostgreSQL migrations, TypeScript generated API types, GitHub Actions.

---

## File Structure

**Create:**

```text
api/openapi/root.yaml
api/openapi/common/errors.yaml
api/openapi/common/headers.yaml
api/openapi/common/security.yaml
api/openapi/common/schemas.yaml
api/openapi/services/identity-access.yaml
api/openapi/services/api-gateway.yaml
api/openapi/services/event-contract-platform.yaml
api/openapi/services/platform-ops.yaml
api/openapi/services/observability-audit.yaml
api/openapi/services/relay.yaml
api/openapi/services/knowledge-rag.yaml
api/openapi/services/tool-mcp.yaml
api/openapi/services/sandbox.yaml
api/openapi/services/chat.yaml
api/openapi/services/agent.yaml
api/openapi/services/workflow.yaml
api/openapi/services/task-scheduler.yaml
api/openapi/services/channel.yaml
api/openapi/services/billing-payment.yaml
api/openapi/services/marketplace.yaml
api/openapi/services/admin-console.yaml
api/proto/oblivious/common/v1/request_context.proto
api/proto/oblivious/common/v1/error.proto
api/proto/oblivious/events/v1/envelope.proto
api/proto/oblivious/identityaccess/v1/service.proto
api/proto/oblivious/apigateway/v1/service.proto
api/proto/oblivious/eventcontractplatform/v1/service.proto
api/proto/oblivious/platformops/v1/service.proto
api/proto/oblivious/observabilityaudit/v1/service.proto
api/proto/oblivious/relay/v1/service.proto
api/proto/oblivious/knowledgerag/v1/service.proto
api/proto/oblivious/toolmcp/v1/service.proto
api/proto/oblivious/sandbox/v1/service.proto
api/proto/oblivious/chat/v1/service.proto
api/proto/oblivious/agent/v1/service.proto
api/proto/oblivious/workflow/v1/service.proto
api/proto/oblivious/taskscheduler/v1/service.proto
api/proto/oblivious/channel/v1/service.proto
api/proto/oblivious/billingpayment/v1/service.proto
api/proto/oblivious/marketplace/v1/service.proto
api/proto/oblivious/adminconsole/v1/service.proto
contracts/ownership.yaml
contracts/events/topic-catalog.yaml
contracts/leases/active/.gitkeep
contracts/baselines/f0/
contracts/baselines/f1/
scripts/contracts/bundle_openapi.py
scripts/contracts/verify_contract_lease.py
scripts/contracts/verify_event_contract.py
scripts/contracts/verify_database_ownership.py
scripts/tests/test_contract_lease.py
scripts/tests/test_openapi_bundle.py
scripts/tests/test_event_contract.py
scripts/tests/test_database_ownership.py
src/server/cmd/contractcheck/main.go
src/web/src/generated/openapi.ts
src/server/migrations/services/identity-access/
src/server/migrations/services/api-gateway/
src/server/migrations/services/event-contract-platform/
src/server/migrations/services/platform-ops/
src/server/migrations/services/observability-audit/
src/server/migrations/services/relay/
src/server/migrations/services/knowledge-rag/
src/server/migrations/services/tool-mcp/
src/server/migrations/services/sandbox/
src/server/migrations/services/chat/
src/server/migrations/services/agent/
src/server/migrations/services/workflow/
src/server/migrations/services/task-scheduler/
src/server/migrations/services/channel/
src/server/migrations/services/billing-payment/
src/server/migrations/services/marketplace/
src/server/migrations/services/admin-console/
```

**Modify:**

```text
docs/api/openapi.yaml
src/server/Makefile
package.json
src/web/package.json
src/server/pkg/event/producer.go
src/server/pkg/event/consumer.go
src/server/migrations/microservices/table-ownership.json
scripts/migrate-service-template.sh
scripts/verify-openapi-contract.py
scripts/check.sh
.github/workflows/ci.yml
```

**Remove only after consumer migration proves parity:** duplicate Proto sources under `src/server/api/proto/` and `src/server/pkg/relay/proto/`, old generated directories, and the old migration runner behavior.

## Task 1: Enforce Shared Contract Leases

**Files:**

- Create: `contracts/ownership.yaml`
- Create: `contracts/leases/active/.gitkeep`
- Create: `scripts/contracts/verify_contract_lease.py`
- Create: `scripts/tests/test_contract_lease.py`

- [ ] **Step 1: Write failing lease tests**

```python
import unittest

from scripts.contracts.verify_contract_lease import validate_lease


class ContractLeaseTests(unittest.TestCase):
    def test_rejects_expired_lease(self):
        errors = validate_lease({
            "scope": "proto",
            "holder": "agent-contract-1",
            "worktree": "../Oblivious-wt/f0-contracts",
            "baseCommit": "3531446",
            "paths": ["api/proto/oblivious/common/**"],
            "grantedBy": "@shirosoralumie648",
            "grantedAt": "2026-07-10T00:00:00Z",
            "expiresAt": "2026-07-10T04:00:00Z"
        }, now="2026-07-10T04:00:01Z")
        self.assertIn("lease is expired", errors)

    def test_rejects_overlapping_active_paths(self):
        self.assertEqual(
            ["active lease paths overlap"],
            validate_lease.overlap(
                ["contracts/**"], ["contracts/baselines/f0/**"]
            ),
        )
```

- [ ] **Step 2: Run tests and confirm failure**

Run: `python3 -m unittest scripts.tests.test_contract_lease -v`

Expected: import failure because the verifier does not exist.

- [ ] **Step 3: Implement exact lease validation**

Required fields:

```python
REQUIRED_FIELDS = {
    "scope", "issue", "holder", "worktree", "baseCommit", "paths",
    "affectedServices", "grantedBy", "grantedAt", "expiresAt",
}
MAX_LEASE_HOURS = 4
```

Reject missing fields, non-human `grantedBy`, duration over four hours, expired leases, path overlap, worktree mismatch, and base commit mismatch.

- [ ] **Step 4: Define shared ownership domains**

`contracts/ownership.yaml` must assign human owners to `external-api`, `proto`, `events`, `database`, `generated-clients`, `dependency-locks`, `deployment`, and `release-gates`.

- [ ] **Step 5: Verify and commit**

```bash
python3 -m unittest scripts.tests.test_contract_lease -v
python3 scripts/contracts/verify_contract_lease.py --repo-root .
git add contracts/ownership.yaml contracts/leases scripts/contracts \
  scripts/tests/test_contract_lease.py
git commit -m "chore(contracts): enforce ownership and write leases"
```

## Task 2: Split OpenAPI And Freeze Shared HTTP Semantics

**Files:**

- Create: `api/openapi/root.yaml`
- Create: `api/openapi/common/errors.yaml`
- Create: `api/openapi/common/headers.yaml`
- Create: `api/openapi/common/security.yaml`
- Create: `api/openapi/common/schemas.yaml`
- Create: the 17 exact `api/openapi/services/*.yaml` files listed in File Structure.
- Create: `scripts/contracts/bundle_openapi.py`
- Create: `scripts/tests/test_openapi_bundle.py`
- Modify: `docs/api/openapi.yaml`
- Modify: `scripts/verify-openapi-contract.py`

- [ ] **Step 1: Write failing bundle tests**

```python
def test_bundle_is_deterministic(tmp_path):
    first = bundle("api/openapi/root.yaml")
    second = bundle("api/openapi/root.yaml")
    assert first == second


def test_every_mutation_declares_idempotency(bundle_doc):
    for path, method, operation in mutations(bundle_doc):
        assert operation["x-oblivious-idempotency"] in {
            "required", "optional", "forbidden"
        }, f"{method.upper()} {path} missing idempotency contract"
```

- [ ] **Step 2: Run tests and confirm failure**

Run: `python3 -m unittest scripts.tests.test_openapi_bundle -v`

Expected: failure because the split source and bundler do not exist.

- [ ] **Step 3: Define the common error payload**

```yaml
ErrorPayload:
  type: object
  required: [code, message, retryable, requestId, traceId]
  properties:
    code: {type: string}
    message: {type: string}
    details: {type: object, additionalProperties: true}
    retryable: {type: boolean}
    requestId: {type: string}
    traceId: {type: string}
```

Relay `/v1/*` keeps OpenAI-compatible response bodies but uses the same error-code catalog and tracing headers.

- [ ] **Step 4: Define common headers**

```yaml
XRequestID:
  name: X-Request-ID
  in: header
  required: false
  schema: {type: string}
Traceparent:
  name: traceparent
  in: header
  required: false
  schema: {type: string}
IdempotencyKey:
  name: Idempotency-Key
  in: header
  required: true
  schema: {type: string, minLength: 16, maxLength: 255}
```

- [ ] **Step 5: Implement deterministic bundling**

The bundler resolves `$ref` values, sorts component and path keys, writes canonical YAML to `docs/api/openapi.yaml`, and supports `--check` without modifying files.

- [ ] **Step 6: Preserve current contract coverage**

Run:

```bash
python3 scripts/contracts/bundle_openapi.py
bash scripts/verify-openapi-contract.sh
python3 scripts/contracts/bundle_openapi.py --check
```

Expected: the generated bundle retains all current operations and adds shared tracing/idempotency semantics.

- [ ] **Step 7: Commit**

```bash
git add api/openapi docs/api/openapi.yaml scripts/contracts/bundle_openapi.py \
  scripts/tests/test_openapi_bundle.py scripts/verify-openapi-contract.py
git commit -m "contract(f0): freeze shared HTTP semantics"
```

## Task 3: Consolidate Proto, Request Context, Errors, And Event Envelope

**Files:**

- Create: `api/proto/oblivious/common/v1/request_context.proto`
- Create: `api/proto/oblivious/common/v1/error.proto`
- Create: `api/proto/oblivious/events/v1/envelope.proto`
- Create: the 17 exact service Proto files listed in File Structure.
- Modify: `src/server/Makefile`
- Create: `src/server/cmd/contractcheck/main.go`

- [ ] **Step 1: Define request context**

```proto
syntax = "proto3";
package oblivious.common.v1;

message RequestContext {
  string request_id = 1;
  string traceparent = 2;
  string tenant_id = 3;
  string principal_id = 4;
  string idempotency_key = 5;
  string region = 6;
  string deployment_id = 7;
}
```

- [ ] **Step 2: Define the event envelope**

```proto
syntax = "proto3";
package oblivious.events.v1;

import "google/protobuf/any.proto";
import "google/protobuf/timestamp.proto";

message EventEnvelope {
  string event_id = 1;
  string event_type = 2;
  uint32 event_version = 3;
  google.protobuf.Timestamp occurred_at = 4;
  string producer = 5;
  string tenant_id = 6;
  string aggregate_id = 7;
  string trace_id = 8;
  string correlation_id = 9;
  string causation_id = 10;
  string idempotency_key = 11;
  google.protobuf.Any payload = 12;
}
```

- [ ] **Step 3: Define gRPC error mapping**

Use `google.rpc.Status` with `ErrorInfo`, `BadRequest`, and `RetryInfo`. Add a Go test that maps each shared error code to the same HTTP status, gRPC code, retryable flag, and event failure category.

- [ ] **Step 4: Replace Proto generation with one command**

`make -C src/server proto-gen` must generate every service into these exact directories and produce one descriptor set for compatibility comparison:

```text
src/server/internal/grpc/identityaccessv1/
src/server/internal/grpc/apigatewayv1/
src/server/internal/grpc/eventcontractplatformv1/
src/server/internal/grpc/platformopsv1/
src/server/internal/grpc/observabilityauditv1/
src/server/internal/grpc/relayv1/
src/server/internal/grpc/knowledgeragv1/
src/server/internal/grpc/toolmcpv1/
src/server/internal/grpc/sandboxv1/
src/server/internal/grpc/chatv1/
src/server/internal/grpc/agentv1/
src/server/internal/grpc/workflowv1/
src/server/internal/grpc/taskschedulerv1/
src/server/internal/grpc/channelv1/
src/server/internal/grpc/billingpaymentv1/
src/server/internal/grpc/marketplacev1/
src/server/internal/grpc/adminconsolev1/
```

- [ ] **Step 5: Prove generated output is reproducible**

```bash
make -C src/server proto-gen
git diff --exit-code -- src/server/internal/grpc
(cd src/server && go run ./cmd/contractcheck proto \
  --source ../../api/proto \
  --descriptor ../../contracts/baselines/f0/proto.pb)
```

Expected: generation creates no unstaged difference on the second run; descriptor validation exits `0`.

- [ ] **Step 6: Commit**

```bash
git add api/proto/oblivious src/server/Makefile \
  src/server/internal/grpc src/server/cmd/contractcheck
git commit -m "contract(f0): freeze shared gRPC and event semantics"
```

## Task 4: Implement Reliable Event Delivery Contracts

**Files:**

- Create: `contracts/events/topic-catalog.yaml`
- Create: `scripts/contracts/verify_event_contract.py`
- Create: `scripts/tests/test_event_contract.py`
- Modify: `src/server/pkg/event/producer.go`
- Modify: `src/server/pkg/event/consumer.go`
- Create: service-local `outbox_events` and `inbox_receipts` migrations under each service migration directory.

- [ ] **Step 1: Write failing delivery tests**

Required cases:

```go
func TestConsumerDoesNotCommitWhenHandlerFails(t *testing.T) {}
func TestInboxExecutesDuplicateEventOnce(t *testing.T) {}
func TestOutboxRetriesAfterPublisherFailure(t *testing.T) {}
func TestDLQRedrivePreservesEventIdentity(t *testing.T) {}
func TestProducerReturnsMarshalError(t *testing.T) {}
```

- [ ] **Step 2: Run focused tests and confirm failure**

Run: `(cd src/server && go test ./pkg/event -count=1)`

Expected: failures expose automatic offset commit, ignored marshal error, and missing inbox/outbox behavior.

- [ ] **Step 3: Define the topic catalog schema**

Each topic contains `name`, `owner`, `partitionKey`, `payloadType`, `compatibility`, `retention`, `dlq`, and `dataClassification`.

- [ ] **Step 4: Implement explicit commit and idempotent inbox behavior**

Consumer flow:

```text
fetch message
validate envelope
begin local transaction
insert inbox receipt by event_id
execute handler
commit business data and inbox receipt
commit Kafka offset
```

- [ ] **Step 5: Verify catalog and event tests**

```bash
python3 -m unittest scripts.tests.test_event_contract -v
python3 scripts/contracts/verify_event_contract.py \
  --catalog contracts/events/topic-catalog.yaml \
  --proto-root api/proto
(cd src/server && go test ./pkg/event -count=1)
```

Expected: all commands exit `0`; duplicate processing and failed-handler commits are rejected.

- [ ] **Step 6: Commit**

```bash
git add contracts/events scripts/contracts/verify_event_contract.py \
  scripts/tests/test_event_contract.py src/server/pkg/event \
  src/server/migrations/services
git commit -m "feat(events): add outbox inbox and delivery contracts"
```

## Task 5: Freeze Database-Per-Service Ownership

**Files:**

- Modify: `src/server/migrations/microservices/table-ownership.json`
- Create: the 17 exact service migration directories listed in File Structure.
- Create: `scripts/contracts/verify_database_ownership.py`
- Create: `scripts/tests/test_database_ownership.py`
- Modify: `scripts/migrate-service-template.sh`

- [ ] **Step 1: Write failing ownership tests**

```python
from scripts.contracts.verify_database_ownership import validate_manifest


def test_rejects_duplicate_table_owner():
    errors = validate_manifest({
        "identity-access": {"database": "identity", "role": "identity_role", "tables": ["users"]},
        "admin-console": {"database": "admin", "role": "admin_role", "tables": ["users"]},
    })
    assert "table users has multiple owners" in errors


def test_rejects_cross_service_foreign_key():
    errors = validate_manifest({
        "chat": {
            "database": "chat",
            "role": "chat_role",
            "tables": ["messages"],
            "foreignKeys": [{"from": "messages.user_id", "to": "identity.users.id"}],
        }
    })
    assert "cross-service foreign key messages.user_id -> identity.users.id" in errors


def test_rejects_missing_migration_directory():
    errors = validate_manifest({
        "relay": {
            "database": "relay",
            "role": "relay_role",
            "tables": ["relay_requests"],
            "migrationDir": "src/server/migrations/services/relay",
        }
    }, existing_paths=set())
    assert "missing migration directory for relay" in errors


def test_rejects_legacy_service_alias():
    errors = validate_manifest({
        "gateway": {"database": "gateway", "role": "gateway_role", "tables": []}
    })
    assert "legacy service id gateway is not allowed" in errors


def test_rejects_shared_database_role():
    errors = validate_manifest({
        "relay": {"database": "relay", "role": "shared_role", "tables": []},
        "billing-payment": {"database": "billing", "role": "shared_role", "tables": []},
    })
    assert "database role shared_role is assigned to multiple services" in errors
```

- [ ] **Step 2: Define the 17-service manifest schema**

Each service entry contains:

```json
{
  "databaseEnv": "IDENTITY_ACCESS_DATABASE_URL",
  "roleEnv": "IDENTITY_ACCESS_DATABASE_ROLE",
  "migrationDir": "src/server/migrations/services/identity-access",
  "tables": ["users", "sessions"],
  "extensions": ["pgcrypto"],
  "ledgerTable": "schema_migrations"
}
```

The same shape applies to all approved service IDs. Service-specific values must match the architecture ownership plan.

- [ ] **Step 3: Move service migration inputs into owned directories**

Preserve migration order and checksums. Old `src/server/migrations/microservices/*.sql` files remain as migration inputs until replay parity passes; they are not the final runtime entrypoint.

- [ ] **Step 4: Replace the migration runner with manifest-driven execution**

The runner accepts `--service`, loads the manifest, refuses unknown/legacy IDs, connects using the service-specific database and role, validates ownership, applies migrations, and records file SHA-256 plus manifest SHA-256.

- [ ] **Step 5: Verify isolation**

```bash
python3 -m unittest scripts.tests.test_database_ownership -v
python3 scripts/contracts/verify_database_ownership.py \
  --manifest src/server/migrations/microservices/table-ownership.json \
  --migrations-root src/server/migrations/services
bash scripts/verify-migration-replay.sh
```

Expected: 17 unique database names and roles, no cross-service FK, no unowned table, and successful replay.

- [ ] **Step 6: Commit**

```bash
git add src/server/migrations scripts/contracts/verify_database_ownership.py \
  scripts/tests/test_database_ownership.py scripts/migrate-service-template.sh
git commit -m "contract(db): define seventeen service database ownership"
```

## Task 6: Generate Clients And Publish F0 Baselines

**Files:**

- Create: `src/web/src/generated/openapi.ts`
- Modify: `src/web/package.json`
- Modify: root `package.json`
- Create: `contracts/baselines/f0/openapi.yaml`
- Create: `contracts/baselines/f0/proto.pb`
- Create: `contracts/baselines/f0/topic-catalog.yaml`
- Create: `contracts/baselines/f0/database-ownership.json`
- Create: `contracts/baselines/f0/checksums.json`

- [ ] **Step 1: Add deterministic web generation**

Add script:

```json
{
  "scripts": {
    "generate:api": "openapi-typescript ../../docs/api/openapi.yaml -o src/generated/openapi.ts"
  }
}
```

- [ ] **Step 2: Run all generators twice**

```bash
python3 scripts/contracts/bundle_openapi.py
make -C src/server proto-gen
pnpm --dir src/web generate:api
git diff --exit-code -- docs/api/openapi.yaml src/server/internal/grpc \
  src/web/src/generated/openapi.ts
```

Expected: second generation produces no diff.

- [ ] **Step 3: Write canonical baseline hashes**

`checksums.json` records SHA-256 for the OpenAPI bundle, Proto descriptor, topic catalog, database ownership manifest, and generator versions.

- [ ] **Step 4: Run F0 compatibility gates**

```bash
bash scripts/verify-openapi-contract.sh
python3 scripts/contracts/verify_event_contract.py \
  --catalog contracts/events/topic-catalog.yaml --proto-root api/proto
python3 scripts/contracts/verify_database_ownership.py \
  --manifest src/server/migrations/microservices/table-ownership.json \
  --migrations-root src/server/migrations/services
```

Expected: all commands exit `0`.

- [ ] **Step 5: Commit**

```bash
git add src/web/src/generated src/web/package.json package.json \
  contracts/baselines/f0
git commit -m "contract(f0): publish shared contract baselines"
```

## Task 7: Freeze F1 Service Contracts In Dependency Order

**Files:** For each service, modify only its service OpenAPI, Proto, generated client directory, service migrations, provider tests, and consumer tests.

- [ ] **Step 1: Freeze Foundation contracts**

Order: `identity-access`, `api-gateway`, `event-contract-platform`, `observability-audit`, `platform-ops`.

- [ ] **Step 2: Freeze the Relay/Billing authority pair**

Relay defines authoritative `usage_id`, measured units, provider request ID, and pricing snapshot. Billing defines reservation, commit, release, refund, immutable ledger transaction, and reconciliation result. A joint consumer test must prove Billing never estimates tokens.

- [ ] **Step 3: Freeze AI Runtime contracts**

Order: `knowledge-rag`, `tool-mcp`, `sandbox`.

- [ ] **Step 4: Freeze Product contracts**

Order: `chat`, `agent`, `workflow`, `task-scheduler`, `channel`.

- [ ] **Step 5: Freeze Commerce contracts**

Order: `marketplace`, `admin-console`.

- [ ] **Step 6: Apply the same TDD loop to every service**

```text
write provider contract test
run and observe expected failure
define OpenAPI/Proto/Event/DB source
generate clients
write consumer contract test
implement adapter
run component test
run F0 compatibility checks
commit contract source and generated output atomically
```

- [ ] **Step 7: Publish F1 baseline**

Create `contracts/baselines/f1/` only after all 17 service contract checks, Gateway route checks, generated-client checks, and golden journey contract checks pass.

- [ ] **Step 8: Commit**

```bash
git add contracts/baselines/f1
git commit -m "contract(f1): publish release candidate contract baselines"
```

## Task 8: Wire Contract Gates Into CI

**Files:**

- Create: `.github/workflows/contract-gates.yml`
- Modify: `scripts/check.sh`

- [ ] **Step 1: Add the contract-gates workflow**

The workflow runs lease validation, OpenAPI bundle check, Proto generation diff, event catalog validation, database ownership validation, web generation diff, focused Go contract tests, and `bash scripts/check.sh docs`.

- [ ] **Step 2: Add local aggregate target**

`bash scripts/check.sh contracts` must run the same commands as CI and fail on generated-file drift.

- [ ] **Step 3: Run the complete gate**

```bash
bash scripts/check.sh contracts
bash scripts/check.sh docs
git diff --check
```

Expected: exit `0`; no generated-file drift and no ownership diagnostic.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/contract-gates.yml scripts/check.sh
git commit -m "ci: enforce contract compatibility gates"
```
