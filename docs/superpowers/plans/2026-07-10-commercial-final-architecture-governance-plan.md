# Commercial Final Architecture Governance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Freeze the 17-service architecture, ownership, migration authority, CODEOWNERS, contract leases, and RFC process before implementation work begins.

**Architecture:** Replace the accepted 12-service ADR with an explicit superseding ADR while preserving historical context. Store machine-readable service and contract ownership, validate it against migration ownership and CODEOWNERS, and make the existing docs gate reject drift.

**Tech Stack:** Markdown ADRs, JSON governance manifests, GitHub CODEOWNERS and issue templates, Python 3 verifier, Bash fixtures, existing `scripts/check.sh docs` CI path.

---

## File Structure

**Create:**

- `docs/architecture/ADR-013-commercial-final-service-boundaries.md`
- `docs/governance/service-ownership.json`
- `docs/governance/contract-domains.json`
- `docs/governance/contract-change-policy.md`
- `docs/governance/rfcs/README.md`
- `docs/governance/rfcs/RFC-0000-template.md`
- `.github/CODEOWNERS`
- `.github/PULL_REQUEST_TEMPLATE.md`
- `.github/ISSUE_TEMPLATE/contract-rfc.yml`
- `.github/ISSUE_TEMPLATE/contract-lease.yml`
- `scripts/verify-architecture-governance.py`
- `scripts/verify-architecture-governance-fixtures.sh`
- `scripts/tests/test_architecture_governance.py`

**Modify:**

- `docs/architecture/ADR-012-microservices-boundaries.md`
- `docs/governance/owner-matrix.md`
- `src/server/migrations/microservices/table-ownership.json`
- `scripts/check.sh`

**Do not modify:** API/Proto/event sources, SQL migration files, deployment files, dependency locks, release verifier behavior, existing untracked evidence scripts, or `src/server/internal/outboundpolicy/`.

## Task 1: Build The Governance Verifier Test-First

**Files:**

- Create: `scripts/tests/test_architecture_governance.py`
- Create: `scripts/verify-architecture-governance.py`
- Create: `scripts/verify-architecture-governance-fixtures.sh`

- [ ] **Step 1: Write failing unit tests**

```python
import unittest

from scripts.verify_architecture_governance import validate_repository


class GovernanceVerifierTests(unittest.TestCase):
    def test_rejects_missing_service(self):
        errors = validate_repository("tests/fixtures/governance/missing-service")
        self.assertIn("service catalog must contain exactly 17 approved services", errors)

    def test_rejects_duplicate_table_owner(self):
        errors = validate_repository("tests/fixtures/governance/duplicate-table-owner")
        self.assertIn("table organizations has multiple owners", errors)

    def test_rejects_unsuperseded_adr(self):
        errors = validate_repository("tests/fixtures/governance/unsuperseded-adr")
        self.assertIn("ADR-012 must be superseded by ADR-013", errors)

    def test_rejects_missing_codeowner(self):
        errors = validate_repository("tests/fixtures/governance/missing-codeowner")
        self.assertIn("missing CODEOWNERS coverage", "\n".join(errors))

    def test_rejects_contract_domain_without_lease(self):
        errors = validate_repository("tests/fixtures/governance/missing-lease")
        self.assertIn("contract domain must require an exclusive lease", errors)
```

- [ ] **Step 2: Run tests and confirm failure**

Run: `python3 -m unittest scripts.tests.test_architecture_governance -v`

Expected: import failure because `scripts/verify_architecture_governance.py` does not exist.

- [ ] **Step 3: Implement the verifier contract**

```python
APPROVED_SERVICES = {
    "identity-access", "api-gateway", "event-contract-platform",
    "platform-ops", "observability-audit", "relay", "knowledge-rag",
    "tool-mcp", "sandbox", "chat", "agent", "workflow",
    "task-scheduler", "channel", "billing-payment", "marketplace",
    "admin-console",
}

APPROVED_DOMAINS = {"Foundation", "AI Runtime", "Product", "Commerce"}
LEASE_DOMAINS = {
    "external-api", "proto", "events", "database",
    "generated-clients", "dependency-locks", "deployment", "release-gates",
}
```

The verifier must load the two governance JSON files, migration ownership, ADR status, and CODEOWNERS; collect all errors; print one diagnostic per line; and exit non-zero when errors exist.

- [ ] **Step 4: Run unit and fixture tests**

```bash
python3 -m unittest scripts.tests.test_architecture_governance -v
bash scripts/verify-architecture-governance-fixtures.sh
```

Expected: valid fixture passes; five invalid fixtures fail with the exact diagnostics above.

- [ ] **Step 5: Commit**

```bash
git add scripts/tests/test_architecture_governance.py \
  scripts/verify-architecture-governance.py \
  scripts/verify-architecture-governance-fixtures.sh
git commit -m "test: add architecture governance verifier"
```

## Task 2: Supersede ADR-012 With ADR-013

**Files:**

- Modify: `docs/architecture/ADR-012-microservices-boundaries.md`
- Create: `docs/architecture/ADR-013-commercial-final-service-boundaries.md`

- [ ] **Step 1: Add the supersession marker to ADR-012**

The header must contain:

```markdown
状态: SUPERSEDED
替代文档: ADR-013 Commercial Final Service Boundaries
```

Retain the historical 12-service decision below the marker.

- [ ] **Step 2: Create ADR-013 with binding decisions**

ADR-013 must define all 17 services, four domain units, database-per-service, Relay usage authority, Billing financial authority, transactional outbox/idempotent inbox, one big-bang production cutover, and four deployment profiles from one artifact set.

- [ ] **Step 3: Run the verifier**

Run: `python3 scripts/verify-architecture-governance.py --repo-root .`

Expected: the ADR check passes; remaining diagnostics refer only to ownership files not created yet.

- [ ] **Step 4: Commit**

```bash
git add docs/architecture/ADR-012-microservices-boundaries.md \
  docs/architecture/ADR-013-commercial-final-service-boundaries.md
git commit -m "docs: replace legacy microservice boundary ADR"
```

## Task 3: Define Machine-Readable Service Ownership

**Files:**

- Create: `docs/governance/service-ownership.json`
- Modify: `docs/governance/owner-matrix.md`

- [ ] **Step 1: Write the 17-service catalog**

Use this exact ID and domain mapping:

```json
{
  "domainUnits": {
    "Foundation": ["identity-access", "api-gateway", "event-contract-platform", "platform-ops", "observability-audit"],
    "AI Runtime": ["relay", "knowledge-rag", "tool-mcp", "sandbox"],
    "Product": ["chat", "agent", "workflow", "task-scheduler", "channel"],
    "Commerce": ["billing-payment", "marketplace", "admin-console"]
  }
}
```

Each service object must also contain `primaryRole`, `secondaryRole`, `codePaths`, `contractDomains`, `databaseName`, `migrationOwner`, and `legacyAliases`.

- [ ] **Step 2: Add role definitions to the owner matrix**

Define `ARC`, `EDGE`, `DATA`, `COM`, and `PLAT` with one primary and one secondary human per service. Until a second GitHub collaborator exists, record the missing human assignment as a release blocker in prose; do not fabricate an account.

- [ ] **Step 3: Verify**

Run: `python3 scripts/verify-architecture-governance.py --repo-root .`

Expected: service/domain counts pass; migration and CODEOWNERS checks may remain open.

- [ ] **Step 4: Commit**

```bash
git add docs/governance/service-ownership.json docs/governance/owner-matrix.md
git commit -m "docs: define seventeen-service ownership"
```

## Task 4: Freeze Migration Ownership

**Files:**

- Modify: `src/server/migrations/microservices/table-ownership.json`

- [ ] **Step 1: Replace old service keys with the approved 17 IDs**

Rules:

```text
mcp_servers -> tool-mcp
users, sessions, memberships, invitations -> identity-access
audit_logs and alert state -> observability-audit
admin policies and approvals -> admin-console
provider/model/usage authority -> relay
quota/payment/ledger authority -> billing-payment
```

Services with no current table use an empty array. A table may appear under exactly one owner.

- [ ] **Step 2: Run ownership and migration tests**

```bash
python3 scripts/verify-architecture-governance.py --repo-root .
bash scripts/verify-migration-contract.sh
(cd src/server && go test ./internal/knowledge \
  -run TestKnowledgeIngestionJobsMigrationDeclaresDurableWorkerFields -count=1)
```

Expected: no duplicate owner, migration naming passes, focused Go test returns `ok`.

- [ ] **Step 3: Commit**

```bash
git add src/server/migrations/microservices/table-ownership.json
git commit -m "docs: freeze target migration ownership"
```

## Task 5: Enforce CODEOWNERS

**Files:**

- Create: `.github/CODEOWNERS`

- [ ] **Step 1: Add broad ownership rules**

```text
/docs/architecture/ @shirosoralumie648
/docs/governance/ @shirosoralumie648
/api/ @shirosoralumie648
/src/server/migrations/ @shirosoralumie648
/deploy/ @shirosoralumie648
/scripts/verify-commercial-completion.sh @shirosoralumie648
/scripts/check.sh @shirosoralumie648
```

- [ ] **Step 2: Add narrow service rules after broad rules**

Include each service's `cmd`, `internal`, Proto/OpenAPI, migration, image, and deployment paths. Put `src/server/internal/workflow/sandbox/` after `src/server/internal/workflow/` so Sandbox ownership wins.

- [ ] **Step 3: Verify coverage**

Run: `python3 scripts/verify-architecture-governance.py --repo-root .`

Expected: all 17 services and all eight shared contract domains have CODEOWNERS coverage.

- [ ] **Step 4: Commit**

```bash
git add .github/CODEOWNERS
git commit -m "chore: enforce service code ownership"
```

## Task 6: Define Contract Lease And RFC Governance

**Files:**

- Create: `docs/governance/contract-domains.json`
- Create: `docs/governance/contract-change-policy.md`
- Create: `docs/governance/rfcs/README.md`
- Create: `docs/governance/rfcs/RFC-0000-template.md`
- Create: `.github/PULL_REQUEST_TEMPLATE.md`
- Create: `.github/ISSUE_TEMPLATE/contract-rfc.yml`
- Create: `.github/ISSUE_TEMPLATE/contract-lease.yml`

- [ ] **Step 1: Define eight exclusive lease domains**

```json
{
  "leaseDurationHours": 4,
  "domains": [
    "external-api", "proto", "events", "database",
    "generated-clients", "dependency-locks", "deployment", "release-gates"
  ]
}
```

- [ ] **Step 2: Define required lease fields**

The lease issue form requires `scope`, `holder`, `worktree`, `baseCommit`, `paths`, `affectedServices`, `grantedBy`, `grantedAt`, and `expiresAt`. Only a human owner can grant or renew a lease.

- [ ] **Step 3: Define breaking-change RFC fields**

The RFC template requires current behavior, proposed behavior, consumers, compatibility strategy, migration order, rollback/forward-fix, security impact, data impact, and approvals from all affected owners.

- [ ] **Step 4: Extend the PR template**

Required checkboxes: contract impact, migration impact, lease ID, RFC ID, exact verification commands, recovery method, generated-file status, and target-evidence impact.

- [ ] **Step 5: Verify and commit**

```bash
python3 scripts/verify-architecture-governance.py --repo-root .
git add docs/governance .github/PULL_REQUEST_TEMPLATE.md .github/ISSUE_TEMPLATE
git commit -m "docs: define contract lease and RFC governance"
```

Expected: governance verifier reports 17 services, four domain units, and eight lease domains.

## Task 7: Wire Governance Into The Existing Docs Gate

**Files:**

- Modify: `scripts/check.sh`

- [ ] **Step 1: Add fixture and repository checks to `run_docs_checks`**

```bash
echo "[check] Verifying architecture governance fixtures."
bash "$repo_root/scripts/verify-architecture-governance-fixtures.sh"

echo "[check] Verifying architecture governance."
python3 "$repo_root/scripts/verify-architecture-governance.py" --repo-root "$repo_root"
```

- [ ] **Step 2: Run the complete docs gate**

Run: `bash scripts/check.sh docs`

Expected: exit `0`, including the new fixture and repository checks.

- [ ] **Step 3: Run final hygiene checks**

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only planned governance files are changed. Existing unrelated untracked paths remain unmodified.

- [ ] **Step 4: Commit**

```bash
git add scripts/check.sh
git commit -m "ci: enforce architecture governance gate"
```
