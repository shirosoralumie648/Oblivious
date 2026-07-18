---
status: resolved
trigger: Playwright commercial journey intermittently misses the settled chat response under the full 8-worker suite
created: 2026-07-18
updated: 2026-07-18
---

# Symptoms

- Expected: sending the commercial chat message renders the settled Relay response.
- Actual: the full suite intermittently times out at `commercial-journey.spec.ts:24`; the page remains on an empty conversation.
- Reproduction: `PLAYWRIGHT_BROWSERS_PATH="$PWD/.tmp/ms-playwright" pnpm --dir src/web test:e2e` produced `74 passed, 1 failed`; the isolated test and `--repeat-each=3` passed.

## Current Focus

- hypothesis: the test sends before the conversation workspace's initial parallel load has committed, so the draft state and send handler race under worker contention.
- test: inspect Playwright trace request/action ordering and rerun isolated/full suites.
- expecting: failed traces show the send action before initial GET completion and no `/messages/stream` request; a readiness-gated interaction should remove the race.
- next_action: verify the minimal condition-based readiness change and rerun the focused and full E2E suites.

## Evidence

- timestamp: 2026-07-18T12:30+08:00
  detail: failed trace shows `fill` at monotonic 71947.980 and `click` at 71961.791 while the six conversation bootstrap GETs began around 71919 and completed around 72262; no stream request was recorded.
- timestamp: 2026-07-18T12:30+08:00
  detail: single commercial test and `--repeat-each=3` passed; failure moves only under the full 8-worker run.

## Eliminated

- hypothesis: stable fixture route or missing response body
  reason: route defines both JSON and stream paths, and isolated runs receive the expected settled message.

## Resolution

- root_cause: Chat rendered the conversation controls before its initial parallel load completed. Under worker contention, the load effect reset the draft after the test filled it, so the send handler saw an empty draft and returned without opening the stream.
- fix: Chat now disables draft, attachment, and send controls during loading and guards the send handler; the commercial journey waits for the loading status to disappear before filling the draft.
- verification: RED regression test failed before the fix; focused behavior suite passed after the fix; `pnpm --dir src/web test` passed with 68 files and 641 tests; default Playwright suite passed with 75 tests and 8 workers; TypeScript and `git diff --check` passed.
- files_changed: `src/web/src/routes/workspace/ChatPage.tsx`, `src/web/src/routes/workspace/ChatPage.behavior.test.tsx`, `src/web/e2e/commercial-journey.spec.ts`
