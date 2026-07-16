#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
matched="$repo_root/scripts/run-go-tests-matched.sh"
fixture_root=$(mktemp -d)
status_before=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)

cleanup() {
  rm -rf "$fixture_root"
}
trap cleanup EXIT

export TMPDIR="$fixture_root"

bash "$repo_root/scripts/run-go-tests-matched-fixtures.sh"

(
  cd "$repo_root/src/server"

  bash "$matched" ./internal/releasecontract \
    '^(TestContractSchemaRejectsUnknownAndAuthoredIdentityFields|TestAuthoredContractV1ModelsRequiredSections|TestCheckedInContractProfilePolicyAndReferenceClosure|TestLoadBytesRejectsNegativeFamilies|TestLoadBytesUsesExplicitRepoRootAndIgnoresCWD|TestLoadBytesRejectsInvalidRepoRoots|TestFileProfileResolverRequiresCommittedExplicitProfile|TestLoadRejectsContractAndSchemaOutsideExplicitRoot)$'

  bash "$matched" ./internal/releasecontract \
    '^(TestCanonicalBytesGolden|TestDigestEquivalentFormatting|TestDigestSemanticMutation|TestDispatchCommittedOperationPassesLiteralArgv|TestDispatchRejectsBeforeRunner|TestResolveOperationPathRejectsEscapes|TestOperationRunnerReceivesMinimalEnvironment)$'

  bash "$matched" ./internal/buildinfo \
    '^(TestBuildIdentityV1Validation|TestParseLinkedIdentityRejectsInvalidValues|TestHandleInspectionReturnsTrustedIdentityWithoutSideEffects|TestHandleInspectionRejectsMismatch|TestHandleInspectionUsesExplicitRepoRoot|TestGitProviderDerivesCleanIdentity|TestGitProviderRejectsDirtyAndSubstitution|TestEmbeddedProviderRecomputesContractDigest|TestIdentityProvidersUseExplicitRepoRoot)$'

  bash "$matched" ./internal/surfacereport \
    '^(TestDetailsRegistryRejectsDuplicateUnknownAndWrongType|TestBuildIdentityDetailsRegistration|TestSurfaceReportV1NestedValidation|TestSurfaceReportV1RejectsFlatAndMisplacedFields|TestNewBuildIdentityReportUsesTrustedIdentityAndCommittedProfile|TestNewBuildIdentityReportRejectsUnknownExcludedAndConditionalProfile|TestBuildIdentityReportRejectsCallerIdentityAndMismatch|TestAtomicWriterCreatesParentAndReplaces|TestAtomicWriterPreservesDestinationOnInjectedFailures|TestAtomicWriterRollsBackPostRenameDirectorySyncFailure|TestPreserveProducerErrorKeepsPrimaryStatus)$'

  bash "$matched" ./cmd/release-contract \
    '^(TestRunReportBuildIdentityUsesTrustedResolversAndAtomicWriter|TestRunReportBuildIdentityRejectsUntrustedProfile|TestRunVerifyReportRejectsSchemaAndIdentitySplice|TestRunReportPreservesProducerFailure|TestRunValidateDigestAndIdentity|TestRunOperationRequiresExplicitProfile|TestRunRejectsIdentityOverrideFlags|TestRunInspectHasNoExternalSideEffects)$'
)

bash "$repo_root/scripts/verify-release-build-fixtures.sh"

status_after=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)
if [[ "$status_after" != "$status_before" ]]; then
  echo "[fixture] Stage A changed repository status" >&2
  diff -u <(printf '%s\n' "$status_before") <(printf '%s\n' "$status_after") >&2 || true
  exit 1
fi

echo '[fixture] release contract/report Stage A fixtures passed (repository-local; no target evidence)'
