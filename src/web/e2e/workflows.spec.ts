import { expect, test } from '@playwright/test';

import { registerWorkflowRoutes } from './fixtures/workflows';

test.beforeEach(async ({ page }) => {
  await registerWorkflowRoutes(page);
});

test('workflows mobile layout keeps landmarks and canvas scrolling contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/workflows');

  await expect(page.getByRole('heading', { name: 'Workflows' })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Workspace navigation' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Workflows' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('main')).toHaveCount(1);

  const documentFitsViewport = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1);
  expect(documentFitsViewport).toBe(true);

  const canvas = page.getByLabel('React Flow canvas for Release automation');
  await expect(canvas).toBeVisible();
  const canvasUsesContainedScroll = await canvas.evaluate(
    (element) => element.scrollWidth > element.clientWidth && element.clientWidth <= window.innerWidth
  );
  expect(canvasUsesContainedScroll).toBe(true);

  await expect(page.getByLabel('Node sequence for Release automation')).toContainText('manual-start');
  await expect(page.getByLabel('Signed webhook helper for Release automation')).toContainText('X-Oblivious-Signature');
});

test('workflows browser journey covers triggers execution webhook and debug evidence', async ({ page }) => {
  await page.goto('/workflows');

  await expect(page.getByRole('heading', { name: 'Workflows' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Workflows' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Release automation' })).toBeVisible();
  await expect(page.getByText('Status: published')).toBeVisible();
  await expect(page.getByLabel('React Flow canvas for Release automation')).toBeVisible();
  await expect(page.getByLabel('Node sequence for Release automation')).toContainText('manual-start');
  await expect(page.getByLabel('Signed webhook helper for Release automation')).toContainText(
    '/api/v1/workflows/webhooks/org_workflows/workflow_release'
  );

  await page.getByLabel('Conversation match ID').fill('conversation_release');
  await page.getByRole('button', { name: 'Check conversation matches' }).click();
  await expect(page.getByLabel('Conversation trigger match results')).toContainText('Release automation v3');
  await expect(page.getByLabel('Conversation trigger match results')).toContainText(
    'conversation-release | conversation_release'
  );

  await page.getByLabel('Semantic match message').fill('rollback the release incident now');
  await page.getByRole('button', { name: 'Check semantic matches' }).click();
  await expect(page.getByLabel('Semantic trigger match results')).toContainText('incident-response');
  await expect(page.getByLabel('Semantic trigger match results')).toContainText('score 0.94');

  await page.getByLabel('Run input JSON for Release automation').fill('{"release":"2026.06","severity":"sev1"}');
  await page.getByRole('button', { name: 'Run Release automation' }).click();
  await expect(page.getByText('Execution exec_release_run status: Succeeded.')).toBeVisible();

  await page
    .getByLabel('Webhook payload JSON for Release automation')
    .fill('{"event":"release.published","release":"2026.06"}');
  await page.getByRole('button', { name: 'Trigger webhook for Release automation' }).click();
  await expect(page.getByText('Execution exec_release_webhook status: Succeeded.')).toBeVisible();

  await page.getByRole('button', { name: 'Run scheduled task sched_release_daily now' }).click();
  await expect(page.getByText('Scheduled task run sched_release_run status: succeeded.')).toBeVisible();

  await page.getByRole('textbox', { name: 'Node ID', exact: true }).fill('classify');
  await page.getByRole('textbox', { name: 'Node input JSON', exact: true }).fill('{"severity":"sev1"}');
  await page.getByRole('button', { name: 'Test node' }).click();
  await expect(page.getByText('Node classify returned succeeded')).toBeVisible();

  await page.getByRole('button', { name: 'Load executions' }).click();
  await expect(page.getByText('exec_release_run', { exact: true })).toBeVisible();
  await expect(page.getByText('exec_release_webhook', { exact: true })).toBeVisible();

  await page.getByRole('button', { name: 'View details for exec_release_run' }).click();
  await expect(page.getByLabel('Execution debug details for exec_release_run')).toBeVisible();
  await expect(page.getByLabel('Execution debug details for exec_release_run')).toContainText(
    'manual-start -> classify -> notify'
  );
  await expect(page.getByLabel('Execution debug details for exec_release_run')).toContainText(
    'Release incident routed to operations'
  );
  await expect(page.getByLabel('Execution debug details for exec_release_run')).toContainText('Bottleneck: classify');
});

test('workflows browser journey covers version branch resource and paused-failure controls', async ({ page }) => {
  await page.goto('/workflows');

  await expect(page.getByRole('heading', { name: 'Workflows' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Release automation' })).toBeVisible();

  await page.getByRole('button', { name: 'Load versions for Release automation' }).click();
  const versionHistory = page.getByLabel('Version history Release automation');
  await expect(versionHistory).toContainText('Version 1');
  await expect(versionHistory.getByLabel('Workflow version 1 status')).toHaveText('draft');
  await expect(versionHistory).toContainText('manual-start, classify');
  await expect(versionHistory).toContainText('Version 3');

  await page.getByRole('button', { name: 'Rollback Release automation to version 1' }).click();
  await expect(page.getByText('Version: 4')).toBeVisible();
  await expect(page.getByText('Status: draft')).toBeVisible();

  await page.getByRole('button', { name: 'Create branch from Release automation version 2' }).click();
  await page.getByLabel('Branch name for Release automation version 2').fill('Release automation branch');
  await page.getByLabel('Branch description for Release automation version 2').fill('Experiment branch');
  await page.getByLabel('Experiment key for Release automation version 2').fill('release-routing-v2');
  await page.getByLabel('Traffic percent for Release automation version 2').fill('25');
  await page.getByRole('button', { name: 'Submit branch for Release automation version 2' }).click();

  await expect(page.getByRole('heading', { name: 'Release automation branch' })).toBeVisible();
  await expect(versionHistory.getByLabel('Workflow version 1 status')).toHaveText('draft');

  await page.getByRole('button', { name: 'Publish branch Release automation branch' }).click();
  await expect(versionHistory.getByLabel('Workflow version 1 status')).toHaveText('published');

  await page.getByRole('button', { name: 'Merge branch Release automation branch into Release automation' }).click();
  await expect(page.getByText('Version: 5')).toBeVisible();

  const releaseDebug = page.getByLabel('Debug Release automation', { exact: true });
  await releaseDebug.getByRole('button', { name: 'Load executions' }).click();
  const pausedStatus = page.getByLabel('Workflow execution exec_release_paused status');
  await expect(pausedStatus).toHaveText('Paused');
  await expect(page.getByLabel('Paused failure decisions for exec_release_paused')).toContainText(
    'Paused on failed node classify'
  );

  await page.getByLabel('Total tokens for exec_release_paused').fill('2048');
  await page.getByLabel('Node executions for exec_release_paused').fill('1001');
  await page.getByRole('button', { name: 'Check resources for exec_release_paused' }).click();
  await expect(pausedStatus).toHaveText('Paused');
  await expect(page.getByLabel('Resource limits for exec_release_paused')).toBeVisible();

  await page
    .getByLabel('Edited retry input for classify in exec_release_paused')
    .fill('{"severity":"sev1","retryReason":"model-recovered"}');
  await page.getByRole('button', { name: 'Retry classify with edited input for exec_release_paused' }).click();
  await expect(pausedStatus).toHaveText('Running');
  await expect(page.getByLabel('Debug and performance summary for exec_release_paused')).toContainText('notify');
});
