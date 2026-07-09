# Marketplace Governance Review Policy Audit

Date: 2026-07-04

## Scope

Closed a repository-local Marketplace governance gap where runtime review actions and automated review policy evidence were not fully aligned with durable governance audit records.

## Changed Files

- `src/server/internal/marketplace/types.go`
- `src/server/internal/marketplace/review_scanner.go`
- `src/server/internal/marketplace/review_scanner_test.go`
- `src/server/internal/marketplace/governance.go`
- `src/server/internal/marketplace/governance_test.go`
- `src/server/migrations/0096_marketplace_governance_review_actions.sql`

## Runtime Fixes

- Added a forward migration that updates `marketplace_governance_events.action` to allow runtime review events:
  - `automated_review_pass`
  - `automated_review_reject`
  - `needs_changes`
- Added `policyVersion` and `policyChecksum` to `AutomatedReviewResult`.
- Static Marketplace review now stamps each result with a deterministic policy version and SHA-256 policy checksum.
- Governance event metadata now persists the scanner, decision, policy version, policy checksum, findings, and created timestamp for automated review events.

## Verification

- RED: `cd src/server && go test ./internal/marketplace -run TestGovernanceMigrationAllowsRuntimeReviewActions -count=1 -v` failed because `automated_review_pass` was missing from the existing migration constraint.
- RED: `cd src/server && go test ./internal/marketplace -run TestStaticReviewScannerAllowsCleanAgent -count=1 -v` failed because `AutomatedReviewResult` had no policy fingerprint fields.
- GREEN: `cd src/server && go test ./internal/marketplace -run 'TestGovernanceMigrationAllowsRuntimeReviewActions|TestStaticReviewScannerAllowsCleanAgent' -count=1 -v`

## Remaining Boundary

This makes repository-owned Marketplace automated review events auditable by policy version and checksum. It does not turn the static scanner into a full production policy engine; commercial release still needs target policy operations evidence, richer appeal/review history surfaces, and abuse/ranking controls beyond this local governance event closure.
