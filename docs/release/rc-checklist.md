# RC Checklist

This checklist is the minimum release-candidate gate for the current Oblivious mainline.

## Automated Checks

- [ ] Frontend build passes
- [ ] Core route smoke tests pass
- [ ] Server contract tests pass
- [ ] Runtime configuration matches docs
- [ ] Web Vitest suite passes
- [ ] Server unit tests pass
- [ ] Server integration tests pass or are explicitly skipped with `TEST_DATABASE_URL` noted

## Manual Release Review

- [ ] No P0/P1 defects open
- [ ] Release notes summarize scope and known limitations
- [ ] Environment variables match [`config/.env.example`](/home/shirosora/code_storage/Oblivious/.worktrees/phase0-task1-contracts/config/.env.example)
- [ ] API contract changes are reflected in [`docs/architecture/current-system-contracts.md`](/home/shirosora/code_storage/Oblivious/.worktrees/phase0-task1-contracts/docs/architecture/current-system-contracts.md)
