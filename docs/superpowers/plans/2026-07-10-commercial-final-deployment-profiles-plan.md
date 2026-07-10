# Commercial Final Deployment Profiles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship global SaaS, China SaaS, self-hosted Kubernetes, and air-gapped deployments from one application source, Helm interface, image digest lock, contract baseline, and migration set.

**Architecture:** Replace hand-maintained production Kubernetes manifests with one Helm chart and four capability profiles. SaaS profiles bind managed state services; self-hosted and air-gapped profiles bind certified operators or customer services. Every profile uses per-service identity, secrets, network policy, telemetry, backup/restore, and evidence tied to one RC manifest.

**Tech Stack:** Helm 3, Kubernetes, CloudNativePG, Strimzi Kafka, Valkey/Redis HA, Altinity ClickHouse Operator, Qdrant, S3-compatible object storage, External Secrets, Vault/cloud KMS, Cilium-compatible NetworkPolicy, OpenTelemetry, Prometheus, Grafana, Cosign, Syft/CycloneDX, GitHub Actions.

---

## File Structure

**Create:**

```text
deploy/helm/oblivious/Chart.yaml
deploy/helm/oblivious/values.yaml
deploy/helm/oblivious/values.schema.json
deploy/helm/oblivious/templates/_helpers.tpl
deploy/helm/oblivious/templates/services/identity-access.yaml
deploy/helm/oblivious/templates/services/api-gateway.yaml
deploy/helm/oblivious/templates/services/event-contract-platform.yaml
deploy/helm/oblivious/templates/services/platform-ops.yaml
deploy/helm/oblivious/templates/services/observability-audit.yaml
deploy/helm/oblivious/templates/services/relay.yaml
deploy/helm/oblivious/templates/services/knowledge-rag.yaml
deploy/helm/oblivious/templates/services/tool-mcp.yaml
deploy/helm/oblivious/templates/services/sandbox.yaml
deploy/helm/oblivious/templates/services/chat.yaml
deploy/helm/oblivious/templates/services/agent.yaml
deploy/helm/oblivious/templates/services/workflow.yaml
deploy/helm/oblivious/templates/services/task-scheduler.yaml
deploy/helm/oblivious/templates/services/channel.yaml
deploy/helm/oblivious/templates/services/billing-payment.yaml
deploy/helm/oblivious/templates/services/marketplace.yaml
deploy/helm/oblivious/templates/services/admin-console.yaml
deploy/helm/oblivious/templates/platform/
deploy/helm/oblivious/templates/security/
deploy/helm/oblivious/templates/observability/
deploy/helm/oblivious/templates/recovery/
deploy/profiles/saas-global.yaml
deploy/profiles/saas-cn.yaml
deploy/profiles/self-hosted.yaml
deploy/profiles/air-gapped.yaml
config/capabilities/base.yaml
config/capabilities/saas-global.yaml
config/capabilities/saas-cn.yaml
config/capabilities/self-hosted.yaml
config/capabilities/air-gapped.yaml
release/manifests/schema.json
scripts/verify-profile-conformance.py
scripts/verify-profile-conformance-fixtures.sh
scripts/verify-image-digest-lock.py
scripts/verify-adapter-certifications.py
scripts/recovery/backup-all.sh
scripts/recovery/restore-all.sh
scripts/recovery/verify-restored-system.sh
scripts/release/build-images.sh
scripts/release/build-airgap-bundle.sh
scripts/release/verify-airgap-bundle.sh
test/install/j10_airgap_install.sh
test/recovery/j09_upgrade_restore.sh
```

**Modify:**

```text
.github/workflows/ci.yml
.github/workflows/release-evidence.yml
docker-compose.yml
deploy/observability/grafana-dashboard.json
deploy/observability/prometheus-alerts.yaml
scripts/verify-k8s-recovery-policy.sh
scripts/verify-deployment-operations-contract.sh
scripts/verify-target-release-evidence.py
scripts/verify-commercial-completion.sh
scripts/check.sh
docs/release/release-rollback-runbook.md
```

**Retire after parity:** move `deploy/kubernetes/*.yaml` to `deploy/legacy/kubernetes/` after Helm-rendered parity and target-cluster smoke pass. The aggregate `app/server` deployment does not enter the final chart.

## Task 1: Define Profile And Digest Schemas Test-First

**Files:**

- Create: `deploy/helm/oblivious/values.schema.json`
- Create: `release/manifests/schema.json`
- Create: `scripts/verify-profile-conformance.py`
- Create: `scripts/verify-profile-conformance-fixtures.sh`

- [ ] **Step 1: Create invalid profile fixtures**

Fixtures must cover a profile overriding image digest, a missing service binding, duplicate database credentials, air-gap external egress, China profile using international payment rails, and self-hosted requiring a SaaS callback.

- [ ] **Step 2: Run fixtures and confirm failure**

Run: `bash scripts/verify-profile-conformance-fixtures.sh`

Expected: failure because the verifier does not exist.

- [ ] **Step 3: Define the values schema**

Core shape:

```json
{
  "type": "object",
  "required": ["profile", "release", "services", "bindings"],
  "properties": {
    "profile": {
      "enum": ["saas-global", "saas-cn", "self-hosted", "air-gapped"]
    },
    "release": {
      "type": "object",
      "required": ["commit", "imagesLockDigest", "contractsDigest", "migrationsDigest"]
    },
    "services": {"type": "object", "minProperties": 17},
    "bindings": {"type": "object"}
  }
}
```

Profiles may choose adapters and state-service bindings but may not override application digest, API/Proto/event version, or migration set.

- [ ] **Step 4: Define release manifest schema**

One release candidate contains `commit`, `chartDigest`, `imageDigests`, `contractsDigest`, `migrationsDigest`, `sbomDigests`, `provenanceDigests`, and four `profiles` evidence entries.

- [ ] **Step 5: Implement verifier and rerun fixtures**

```bash
bash scripts/verify-profile-conformance-fixtures.sh
python3 scripts/verify-profile-conformance.py --all
```

Expected: valid fixtures pass; each invalid fixture fails with a profile-specific diagnostic.

- [ ] **Step 6: Commit**

```bash
git add deploy/helm/oblivious/values.schema.json release/manifests/schema.json \
  scripts/verify-profile-conformance.py scripts/verify-profile-conformance-fixtures.sh
git commit -m "test(deploy): define profile conformance schema"
```

## Task 2: Build The Helm Chart Foundation

**Files:**

- Create: `deploy/helm/oblivious/Chart.yaml`
- Create: `deploy/helm/oblivious/values.yaml`
- Create: `deploy/helm/oblivious/templates/_helpers.tpl`

- [ ] **Step 1: Define chart metadata**

```yaml
apiVersion: v2
name: oblivious
description: Oblivious commercial final 17-service platform
type: application
version: 1.0.0
appVersion: "1.0.0"
```

- [ ] **Step 2: Define common service values**

Each service requires `enabled`, `image.repository`, `image.digest`, `replicas`, `resources`, `serviceAccount`, `databaseBinding`, `telemetry`, `networkPolicy`, `podDisruptionBudget`, and probe configuration.

- [ ] **Step 3: Define common labels and annotations**

Every workload includes `app.kubernetes.io/name`, `app.kubernetes.io/component`, `app.kubernetes.io/version`, `oblivious.io/profile`, `oblivious.io/region`, and `oblivious.io/release-digest`.

- [ ] **Step 4: Lint the foundation**

Run: `helm lint deploy/helm/oblivious`

Expected: chart metadata and values schema pass; service templates are added in the next task.

- [ ] **Step 5: Commit**

```bash
git add deploy/helm/oblivious/Chart.yaml deploy/helm/oblivious/values.yaml \
  deploy/helm/oblivious/templates/_helpers.tpl
git commit -m "feat(deploy): add commercial Helm chart foundation"
```

## Task 3: Add All Seventeen Service Templates

**Files:**

- Create: `deploy/helm/oblivious/templates/services/*.yaml`

- [ ] **Step 1: Create templates for the five Foundation services**

`identity-access`, `api-gateway`, `event-contract-platform`, `platform-ops`, and `observability-audit` each render Deployment, Service, ServiceAccount, PDB, NetworkPolicy, HPA, and ServiceMonitor resources.

- [ ] **Step 2: Create templates for the four AI Runtime services**

`relay`, `knowledge-rag`, `tool-mcp`, and `sandbox` use separate identities, database bindings, credentials, object prefixes, and egress policy.

- [ ] **Step 3: Create templates for the five Product services**

`chat`, `agent`, `workflow`, `task-scheduler`, and `channel` consume generated service endpoints and do not point at the aggregate server.

- [ ] **Step 4: Create templates for the three Commerce services**

`billing-payment`, `marketplace`, and `admin-console` use separate credentials and restrictive network policy; Admin has no direct database access to other services.

- [ ] **Step 5: Render each profile**

```bash
for profile in saas-global saas-cn self-hosted air-gapped; do
  helm template oblivious deploy/helm/oblivious \
    -f "deploy/profiles/${profile}.yaml" \
    > ".tmp/${profile}.yaml"
done
```

Expected: each render includes 17 Deployments and no aggregate `app/server` Deployment.

- [ ] **Step 6: Commit**

```bash
git add deploy/helm/oblivious/templates/services
git commit -m "feat(deploy): template seventeen independent services"
```

## Task 4: Define Capability Profiles And Adapter Certification

**Files:**

- Create: `deploy/profiles/*.yaml`
- Create: `config/capabilities/*.yaml`
- Create: `scripts/verify-adapter-certifications.py`

- [ ] **Step 1: Define the shared capability base**

The base lists product surfaces, provider capabilities, payment capabilities, channel capabilities, telemetry, recovery, and external egress. Unsupported capabilities are absent or explicitly disabled.

- [ ] **Step 2: Define `saas-global`**

Enable international managed state services, Stripe-class payment, international providers, regional telemetry, and certified global channels.

- [ ] **Step 3: Define `saas-cn`**

Enable China-region managed services, Alipay/WeChat Pay, domestic providers, domestic alert channels, and China residency controls. Disable international payment endpoints unless legally approved and certified.

- [ ] **Step 4: Define `self-hosted`**

Default to customer-owned providers, customer KMS/Vault, optional internal channels, disabled platform marketplace payout, and no required SaaS callback.

- [ ] **Step 5: Define `air-gapped`**

Require zero external DNS/HTTP access, local OpenAI-compatible endpoints only, offline licensing, local registry, local telemetry, and disabled checkout/KYC/payout.

- [ ] **Step 6: Verify adapters**

Run: `python3 scripts/verify-adapter-certifications.py --all-profiles`

Expected: every enabled adapter has region, endpoint policy, secret keys, webhook validation, failure mode, rate limit, evidence ID, and expiry date.

- [ ] **Step 7: Commit**

```bash
git add deploy/profiles config/capabilities scripts/verify-adapter-certifications.py
git commit -m "feat(deploy): define sovereign capability profiles"
```

## Task 5: Implement Per-Service Secrets, Identity, And Egress

**Files:**

- Create: `deploy/helm/oblivious/templates/security/*.yaml`
- Modify: service templates to remove shared `oblivious-secrets` usage.

- [ ] **Step 1: Create one ServiceAccount per service**

Disable automount where Kubernetes API access is unnecessary. Bind workload identity to cloud IAM or Vault roles per profile.

- [ ] **Step 2: Create one ExternalSecret contract per service**

Each workload references only its own database credential, provider credential references, TLS CA, and adapter secrets. Platform Ops can manage references but cannot read business secret values.

- [ ] **Step 3: Define default-deny ingress and egress**

Allow DNS, owned state services, named service dependencies, OTLP collector, and certified adapters only. Air-gap permits no external CIDR or DNS destination.

- [ ] **Step 4: Add security contexts**

All containers use non-root UID, read-only root filesystem, dropped Linux capabilities, seccomp RuntimeDefault, explicit writable volumes, and resource limits.

- [ ] **Step 5: Verify rendered security policy**

```bash
helm template oblivious deploy/helm/oblivious \
  -f deploy/profiles/air-gapped.yaml \
  | python3 scripts/verify-profile-conformance.py --stdin --profile air-gapped
```

Expected: no shared secret, privileged container, missing ServiceAccount, or external egress.

- [ ] **Step 6: Commit**

```bash
git add deploy/helm/oblivious/templates/security \
  deploy/helm/oblivious/templates/services
git commit -m "feat(security): isolate workload identity secrets and egress"
```

## Task 6: Add State-Service Bindings And Certified Bundles

**Files:**

- Create: `deploy/helm/oblivious/templates/platform/*.yaml`
- Modify: `docker-compose.yml` for developer parity only.

- [ ] **Step 1: Define the service binding object**

Each binding contains `endpoint`, `tlsCARef`, `credentialRef`, `region`, `durabilityClass`, `rpo`, `rto`, and `healthRef`.

- [ ] **Step 2: Support `external` mode**

SaaS profiles require external bindings for PostgreSQL, Kafka, ClickHouse, Redis/Valkey, Qdrant, and object storage.

- [ ] **Step 3: Support `bundled-certified` mode**

Self-hosted and air-gapped profiles may install certified CloudNativePG, Strimzi, Valkey HA, Altinity ClickHouse, Qdrant, and S3-compatible components.

- [ ] **Step 4: Prohibit cache/durable-queue ambiguity**

Valkey cache and durable message transport use separate bindings and policies; Kafka remains the durable event authority.

- [ ] **Step 5: Render and validate**

```bash
helm template oblivious deploy/helm/oblivious \
  -f deploy/profiles/self-hosted.yaml \
  | kubeconform -strict -summary
```

Expected: all resources validate; each service references only owned bindings.

- [ ] **Step 6: Commit**

```bash
git add deploy/helm/oblivious/templates/platform docker-compose.yml
git commit -m "feat(deploy): add managed and certified state bindings"
```

## Task 7: Add Observability And Audit Profile Parity

**Files:**

- Create: `deploy/helm/oblivious/templates/observability/*.yaml`
- Modify: `deploy/observability/grafana-dashboard.json`
- Modify: `deploy/observability/prometheus-alerts.yaml`

- [ ] **Step 1: Deploy OpenTelemetry Collector and ServiceMonitors**

Required resource attributes: `service.name`, `deployment.profile`, `region`, `tenant.id`, `request.id`, `trace.id`, and `release.digest`.

- [ ] **Step 2: Provision dashboards**

Dashboards cover Gateway, Relay, Billing settlement, RAG ingestion, Workflow backlog, Task misfire, Marketplace payout, database health, Kafka lag, backup age, and SLO burn rate.

- [ ] **Step 3: Define alerts and profile routes**

Global routes support PagerDuty/Slack/SMTP-class adapters; China routes support Feishu/DingTalk/WeCom/SMS-class adapters; self-hosted and air-gapped route to customer SIEM/SMTP/webhook.

- [ ] **Step 4: Verify dashboard and alerts**

```bash
node scripts/verify-observability-dashboard.mjs
bash scripts/check.sh docs
```

Expected: required panels, SLOs, owners, severities, and delivery routes exist.

- [ ] **Step 5: Commit**

```bash
git add deploy/helm/oblivious/templates/observability deploy/observability
git commit -m "feat(observability): add profile-neutral telemetry stack"
```

## Task 8: Implement Full-State Backup, Restore, And Upgrade Gates

**Files:**

- Create: `deploy/helm/oblivious/templates/recovery/*.yaml`
- Create: `scripts/recovery/*.sh`
- Create: `test/recovery/j09_upgrade_restore.sh`
- Modify: `scripts/verify-k8s-recovery-policy.sh`
- Modify: `scripts/verify-deployment-operations-contract.sh`

- [ ] **Step 1: Back up every state authority**

Cover all 17 PostgreSQL databases, ClickHouse, Kafka metadata and retained events, Qdrant, object storage, Redis/Valkey configuration, capability profiles, and secret metadata without exporting secret plaintext.

- [ ] **Step 2: Restore into an isolated namespace or cluster**

Restore must verify migration head, file checksums, record counts, tenant isolation, topic offsets, Qdrant collection counts, object checksums, and service readiness.

- [ ] **Step 3: Verify RPO/RTO**

Core PostgreSQL and Relay: RPO 5 minutes; Relay RTO 30 minutes; core PostgreSQL RTO 60 minutes; Kafka RTO 2 hours; ClickHouse and object storage RTO 4 hours.

- [ ] **Step 4: Run upgrade and restore journey**

```bash
bash test/recovery/j09_upgrade_restore.sh
```

Expected: N-1 to N migration, restore, rollback/forward-fix, then J02, J04, and J08 pass.

- [ ] **Step 5: Commit**

```bash
git add deploy/helm/oblivious/templates/recovery scripts/recovery \
  test/recovery/j09_upgrade_restore.sh scripts/verify-k8s-recovery-policy.sh \
  scripts/verify-deployment-operations-contract.sh
git commit -m "feat(recovery): verify all-state upgrade and restore"
```

## Task 9: Build Signed Images, Digest Locks, And Air-Gap Bundle

**Files:**

- Create: `scripts/release/*.sh`
- Create: `release/manifests/rc/`
- Create: `test/install/j10_airgap_install.sh`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Build every image once**

Build 17 service images plus web and approved infrastructure helper images. Record immutable digests in `images.lock.yaml`.

- [ ] **Step 2: Generate supply-chain artifacts**

Produce SBOM, provenance, vulnerability report, license report, chart package, chart digest, and contracts lock; sign all release artifacts with Cosign.

- [ ] **Step 3: Build the offline bundle**

Bundle OCI image archives, Helm package, signatures, certificate chain, SBOMs, provenance, migration set, capability profiles, offline verifier, and installation runbook.

- [ ] **Step 4: Install with network denied**

```bash
bash test/install/j10_airgap_install.sh
```

Expected: fresh install, provider call to a local endpoint, backup/restore, upgrade, and uninstall complete without public DNS or HTTP access.

- [ ] **Step 5: Pin CI actions and verify digests**

All GitHub Actions references use commit SHAs. CI fails when an image tag lacks a digest, a signature is missing, or generated SBOM/provenance hashes differ from the manifest.

- [ ] **Step 6: Commit**

```bash
git add scripts/release release/manifests test/install/j10_airgap_install.sh \
  .github/workflows/ci.yml
git commit -m "ci(release): build signed digest-locked release bundles"
```

## Task 10: Extend Target Evidence To Four Profiles

**Files:**

- Modify: `scripts/verify-target-release-evidence.py`
- Modify: `.github/workflows/release-evidence.yml`
- Modify: `scripts/verify-commercial-completion.sh`
- Modify: `scripts/check.sh`

- [ ] **Step 1: Require one release candidate and four profile entries**

Each profile evidence records chart digest, image digest map, capability digest, migration digest, adapter certification IDs, install/upgrade/restore run IDs, cluster identity, region, and golden journey evidence references.

- [ ] **Step 2: Reject cross-profile drift**

The verifier rejects different application digests, contract hashes, migration sets, chart interfaces, or release commit across profiles.

- [ ] **Step 3: Add profile conformance to docs and release gates**

```bash
python3 scripts/verify-profile-conformance.py --all
bash scripts/check.sh docs
bash scripts/verify-target-release-evidence-fixtures.sh
```

Expected: all fixtures pass, including failures for profile drift and missing air-gap proof.

- [ ] **Step 4: Run the final profile-aware verifier**

```bash
COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
COMMERCIAL_COMPLETION_RUN_K8S=true \
COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=false \
bash scripts/verify-commercial-completion.sh
```

Expected: exit `0` only when all four profiles are proven against the same RC digest set.

- [ ] **Step 5: Commit**

```bash
git add scripts/verify-target-release-evidence.py \
  .github/workflows/release-evidence.yml \
  scripts/verify-commercial-completion.sh scripts/check.sh
git commit -m "feat(release): require four-profile target evidence"
```
