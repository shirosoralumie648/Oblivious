---
phase: 31
slug: release-contract-current-baseline
status: verified
threats_open: 0
asvs_level: 1
block_on: high
created: 2026-07-17
---

# Phase 31 - Security

## Trust Boundaries

| Boundary | Description | Data Crossing |
|---|---|---|
| Authored JSON to typed authority | Repository text becomes capability and profile policy. | Public release commitments and operation references |
| Operation metadata to OS process | A committed profile operation becomes an executable invocation. | Repo-relative executable path and literal argv |
| Git checkout to build identity | Mutable source and Git objects become a release tuple. | Commit, tree, contract digest, dirty state |
| Linker and Docker transport to runtime | Build arguments become binary and OCI metadata. | Repository-local public identity fields |
| Producer observations to report | Artifact observations enter the shared report envelope. | Component paths, digests, match results, residual risks |
| Report to filesystem | Validated evidence replaces a prior durable report. | Repository-local typed JSON evidence |

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Evidence | Status |
|---|---|---|---|---|---|---|
| T-31-01-01 | Tampering | contract/schema | high | mitigate | Duplicate-key scan, closed schema, strict typed decode, semantic closure, mutation tests | closed |
| T-31-01-02 | Elevation of Privilege | profile commitment/default | high | mitigate | Exactly one committed/default monolith; candidates remain excluded | closed |
| T-31-01-03 | Tampering | operation path/argv | high | mitigate | Realpath under `scripts/`, executable check, fixed argv, symlink/traversal tests | closed |
| T-31-01-04 | Information Disclosure | validation errors | medium | mitigate | Stable codes and sanitized field/value output; no raw file bodies | closed |
| T-31-01-05 | Tampering | JSON Schema dependency | medium | mitigate | v6.0.2 pinned in `go.mod`/`go.sum`; local behavior tests and module verification | closed |
| T-31-02-01 | Tampering | canonical digest | high | mitigate | Schema/model/authored timing convergence plus passing canonical golden/equivalent/semantic tests and Stage A | closed |
| T-31-02-02 | Elevation of Privilege | operation dispatcher | high | mitigate | Commitment-first resolution, literal argv, minimal PATH-only environment, zero-call tests | closed |
| T-31-02-03 | Spoofing | targeted test evidence | high | mitigate | Concrete test listing precedes execution; zero/invalid/failing selectors fail | closed |
| T-31-02-04 | Repudiation | operation failures | medium | mitigate | Stable required/unknown/excluded/mismatch/path/runner codes | closed |
| T-31-02-05 | Denial of Service | bounded inputs | low | accept | Contract/schema byte limits and developer-controlled test selector | closed |
| T-31-03-01 | Spoofing | Git/build identity | high | mitigate | Clean porcelain check, explicit-root commit/tree, canonical digest, shape validation | closed |
| T-31-03-02 | Tampering | embedded linker tuple | high | mitigate | `TestEmbeddedProviderRecomputesContractDigest` mutates authored max age and passes from committed source | closed |
| T-31-03-03 | Elevation of Privilege | CLI operation profile | high | mitigate | Explicit profile required; excluded/unknown inputs stop before runner | closed |
| T-31-03-04 | Repudiation | identity failures | medium | mitigate | Structured missing/mismatch/dirty/digest codes | closed |
| T-31-03-05 | Information Disclosure | Git/CLI errors | medium | mitigate | Git stderr is not emitted through public errors; CLI serializes stable code/field only | closed |
| T-31-04-01 | Spoofing | report release identity | high | mitigate | Identity/profile are resolver-owned and component identities are cross-checked | closed |
| T-31-04-02 | Tampering | report envelope/details | high | mitigate | Closed structs, strict decode, typed registry, duplicate/unregistered rejection | closed |
| T-31-04-03 | Tampering | atomic writer | high | mitigate | Same-directory staging, sync, rollback snapshot, directory sync, byte verification | closed |
| T-31-04-04 | Repudiation | producer/write failures | medium | mitigate | Primary producer error is preserved with a typed secondary writer code | closed |
| T-31-04-05 | Information Disclosure | report details | medium | mitigate | Narrow typed repository-local observations; Stage B supplies no credentials or target artifacts | closed |
| T-31-05-01 | Spoofing | Docker build identity | high | mitigate | Host clean-Git derivation plus post-build binary/label/package comparison | closed |
| T-31-05-02 | Tampering | binary/OCI/package tuple | high | mitigate | Shared linker tuple, OCI labels, packaged digest recomputation, dynamic mismatch fixture | closed |
| T-31-05-03 | Elevation of Privilege | inspection startup path | high | mitigate | Inspection handler runs before config, DB, flags, listener, dial, or RPC effects | closed |
| T-31-05-04 | Information Disclosure | inspection output | medium | mitigate | Output contains only public repository-local identity fields | closed |
| T-31-05-05 | Denial of Service | Docker availability | low | accept | Stage B fails closed; Stage A retains fast repository-local feedback | closed |
| T-31-06-01 | Spoofing | build report identity/profile | high | mitigate | Trusted resolvers enrich observations and validator rejects identity splicing | closed |
| T-31-06-02 | Tampering | Stage B source/artifacts | high | mitigate | Clean HEAD, exact Git tuple, real Docker/package inspection, no identity overrides | closed |
| T-31-06-03 | Repudiation | skipped/failed checks | high | mitigate | Empty committed skip invariant and nonzero structured failures | closed |
| T-31-06-04 | Tampering | report output | high | mitigate | Atomic writer plus strict typed read-back before pass | closed |
| T-31-06-05 | Information Disclosure | fixture/report logs | medium | mitigate | Temporary/ignored outputs and stable errors; no secret or target-evidence inputs | closed |
| T-31-07-01 | Tampering | profile timing authority | high | mitigate | Required schema fields, typed semantic bounds, checked-in 30/120/30 values, negative tests | closed |
| T-31-07-02 | Spoofing | structural verification | medium | mitigate | Canonical artifact/key-link checks pass 42/42 without waiver | closed |
| T-31-07-03 | Tampering | final release tuple | high | mitigate | Clean porcelain, exact Git/tree/digest derivation, real Stage B, mandatory rerun after tracking commit | closed |
| T-31-07-04 | Repudiation | skipped checks | high | mitigate | Stage B report has `skippedChecks: []`; failures preserve nonzero status | closed |
| T-31-07-05 | Information Disclosure | evidence output | medium | mitigate | Ignored repository-local reports contain no credentials or target artifact bodies | closed |

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---|---|---|---|---|
| AR-31-01 | T-31-02-05 | Inputs are bounded and the remaining selector cost is developer-controlled. | Phase 31 plan | 2026-07-17 |
| AR-31-02 | T-31-05-05 | Docker unavailability must block Stage B rather than create a fallback release claim. | Phase 31 plan | 2026-07-17 |

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|---|---:|---:|---:|---|
| 2026-07-17 | 30 | 28 | 2 | Codex inline audit |
| 2026-07-17 | 35 | 35 | 0 | Codex inline gap-closure audit |

## Blocking Decision

No blocking threat remains. The previously open canonical-digest and embedded-linker threats are closed by committed timing convergence, typed semantic bounds, Stage A, full regression, and clean-HEAD Stage B evidence. Target/live and supply-chain risks remain explicitly deferred to later phases rather than accepted as Phase 31 proof.

## Sign-Off

- [x] All threats have a disposition.
- [x] Accepted risks are documented.
- [x] `threats_open: 0` confirmed.
- [x] `status: verified` set in frontmatter.

**Approval:** verified 2026-07-17 for repository-local Phase 31 scope.
