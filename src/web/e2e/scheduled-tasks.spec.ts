import { expect, test } from '@playwright/test';

import { registerScheduledTasksRoutes } from './fixtures/scheduledTasks';

async function expectNoHorizontalOverflow(page: import('@playwright/test').Page) {
  const overflowState = await page.evaluate(() => {
    const main = document.querySelector('main');

    return {
      documentFits: document.documentElement.scrollWidth <= window.innerWidth + 1,
      mainFits: main instanceof HTMLElement ? main.scrollWidth <= main.clientWidth + 1 : true,
    };
  });

  expect(overflowState).toEqual({
    documentFits: true,
    mainFits: true,
  });
}

test.beforeEach(async ({ page }) => {
  await registerScheduledTasksRoutes(page);
});

test('scheduled tasks browser journey covers create enable run-now and recent runs', async ({ page }) => {
  await page.goto('/scheduled-tasks');

  await expect(page.getByRole('heading', { name: 'Scheduled Tasks' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Scheduled Tasks' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('navigation', { name: 'Workspace navigation' })).toBeVisible();

  const taskList = page.getByLabel('Scheduled task list');
  await expect(taskList).toContainText('Release readiness digest');
  await expect(taskList).toContainText('workflow_release_digest');
  await expect(taskList).toContainText('0 9 * * 1-5');
  await expect(taskList).toContainText('Next: 2026-06-15 09:00');
  await expect(taskList).toContainText('Agent summary pulse');
  await expect(taskList).toContainText('Disabled');

  await page.getByLabel('Schedule name').fill(' Agent browser pulse ');
  await page.getByLabel('Target type').selectOption('agent');
  await page.getByLabel('Target ID').fill(' agent_browser_operator ');
  await page.getByLabel('Cron expression').fill(' */20 * * * * ');
  await page.getByLabel('Enabled').uncheck();
  await page.getByRole('button', { name: 'Create schedule' }).click();

  const createdTask = page.locator('li').filter({ hasText: 'Agent browser pulse' });
  await expect(createdTask).toBeVisible();
  await expect(createdTask).toContainText('agent_browser_operator');
  await expect(createdTask).toContainText('*/20 * * * *');
  await expect(createdTask).toContainText('Disabled');
  await expect(page.getByLabel('Schedule name')).toHaveValue('');
  await expect(page.getByLabel('Target ID')).toHaveValue('');
  await expect(page.getByText('3 scheduled tasks')).toBeVisible();

  await createdTask.getByRole('button', { name: 'Enable agent_browser_operator schedule' }).click();
  await expect(createdTask).toContainText('Enabled');
  await expect(createdTask).toContainText('Next: 2026-06-15 09:20');

  await createdTask.getByRole('button', { name: 'Run agent_browser_operator schedule now' }).click();
  await expect(createdTask.locator('section[aria-label="Recent runs for agent_browser_operator"]')).toBeVisible();
  await expect(createdTask).toContainText('scheduled_run_browser_now');
  await expect(createdTask).toContainText('running');
  await expect(createdTask).toContainText('Started: 2026-06-15 09:07');
  await expect(createdTask).toContainText('Error: None');

  const workflowTask = page.locator('li').filter({ hasText: 'Release readiness digest' });
  await workflowTask.getByRole('button', { name: 'Show recent runs for workflow_release_digest' }).click();
  await expect(workflowTask.locator('section[aria-label="Recent runs for workflow_release_digest"]')).toBeVisible();
  await expect(workflowTask).toContainText('scheduled_run_digest_success');
  await expect(workflowTask).toContainText('completed');
  await expect(workflowTask).toContainText('scheduled_run_digest_failed');
  await expect(workflowTask).toContainText('workflow guard rejected stale deployment input');
});

test('scheduled tasks mobile layout keeps shell navigation and task controls within viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/scheduled-tasks');

  await expect(page.getByRole('heading', { name: 'Scheduled Tasks' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Scheduled Tasks' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('main')).toHaveCount(1);
  await expect(page.getByRole('button', { name: 'Run workflow_release_digest schedule now' })).toBeVisible();

  await expectNoHorizontalOverflow(page);

  await page.getByRole('button', { name: 'Show recent runs for workflow_release_digest' }).click();
  await expect(page.locator('section[aria-label="Recent runs for workflow_release_digest"]')).toContainText(
    'scheduled_run_digest_failed'
  );
  await expect(page.locator('section[aria-label="Recent runs for workflow_release_digest"]')).toContainText(
    'scheduled_run_provider_research_cluster_mobile_without_breaks_20260624'
  );
  await expectNoHorizontalOverflow(page);
});
