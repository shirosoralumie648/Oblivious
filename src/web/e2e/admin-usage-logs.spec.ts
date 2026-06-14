import { expect, test } from '@playwright/test';

import { registerAdminUsageLogsRoutes } from './fixtures/adminUsageLogs';

test.beforeEach(async ({ page }) => {
  await registerAdminUsageLogsRoutes(page);
});

test('admin usage logs filters relay cost analytics in the browser', async ({ page }) => {
  await page.goto('/admin/usage-logs');

  await expect(page.getByRole('link', { name: 'Usage Logs' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Usage Logs' })).toBeVisible();
  await expect(page.getByText('1 relay requests matched')).toBeVisible();
  await expect(page.getByText('req_initial')).toBeVisible();
  await expect(page.getByText('openai / channel_initial')).toBeVisible();

  await page.getByLabel('Organization ID filter').fill('org_browser');
  await page.getByLabel('User ID filter').fill('user_browser');
  await page.getByLabel('Request ID filter').fill('req_browser_filtered');
  await page.getByLabel('API token ID filter').fill('tok_browser');
  await page.getByLabel('API type filter').fill('chat');
  await page.getByLabel('Feature type filter').fill('workspace_chat');
  await page.getByLabel('Quota mode filter').fill('relay_billing');
  await page.getByLabel('Channel ID filter').fill('channel_openai_primary');
  await page.getByLabel('Provider filter').fill('openai');
  await page.getByLabel('Status filter').fill('success');
  await page.getByLabel('Model filter').fill('gpt-4o');
  await page.getByLabel('Analytics granularity filter').selectOption('week');

  await expect(page.getByText('req_browser_filtered')).toBeVisible();
  await expect(page.getByText('user_browser').first()).toBeVisible();
  await expect(page.getByText('tok_browser')).toBeVisible();
  await expect(page.getByText('workspace_chat').first()).toBeVisible();
  await expect(page.getByText('relay_billing')).toBeVisible();
  await expect(page.getByText('gpt-4o').first()).toBeVisible();
  await expect(page.getByText('openai / channel_openai_primary')).toBeVisible();
  await expect(page.getByLabel('Success')).toBeVisible();
  await expect(page.getByText('$0.1234')).toBeVisible();
  await expect(page.getByText('$0.0456')).toBeVisible();
  await expect(page.getByText('1,240')).toBeVisible();
  await expect(page.getByText('88 ms')).toBeVisible();

  await expect(page.getByRole('heading', { name: 'Usage Analytics' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'By model' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'By channel' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'By provider' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Cross dimensions' })).toBeVisible();
  await expect(page.getByLabel('By time').getByText('2026-W25', { exact: true })).toBeVisible();
  await expect(page.getByText('gpt-4o / 2026-W25')).toBeVisible();
  await expect(page.getByText('user_browser / workspace_chat')).toBeVisible();
  await expect(page.getByText('$1.1106').first()).toBeVisible();
});
