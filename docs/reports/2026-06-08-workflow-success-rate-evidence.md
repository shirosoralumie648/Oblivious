# Workflow Success Rate Evidence - 2026-06-08

This report captures the deterministic workflow execution evidence that the
release-gates `verify-workflow-success-rate-evidence.sh` gate relies on, and
records the boundary between the local proof and the environment-specific
external production proof.

## Local Deterministic Workflow Load Evidence

- Verifier script: `scripts/verify-workflow-success-rate-evidence.sh`
- Gate test: `TestServiceWorkflowSuccessRateEvidenceGate` in
  `src/server/internal/workflow/runtime_execution_test.go`
- Required deterministic line emitted by the test:
  `workflow_success_rate_evidence executions=100 succeeded=100 failed=0 success_rate=1.0000 threshold=0.9900`
- Result captured locally: 100/100 successful executions with
  `success_rate=1.0000` against the in-tree `threshold=0.9900`.

This local deterministic workflow load evidence is the only signal the
release-gates job will block on. It is reproducible from a clean checkout by
running `bash scripts/check.sh docs`, which transitively invokes the verifier
script above.

## External Production Proof Boundary

external production proof remains environment-specific and is not part of
this repository's CI contract.

- Operators must capture the equivalent metric in their production telemetry
  pipeline before promoting a release; the 100/100 gate above is a necessary
  precondition, not a substitute.
- Any divergence between local and external success rates must be triaged via
  the `blocker-escalation` governance process.
