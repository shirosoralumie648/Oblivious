import { expect, test } from '@playwright/test';

import { registerAdminModelsRoutes } from './fixtures/adminModels';

test.beforeEach(async ({ page }) => {
  await registerAdminModelsRoutes(page);
});

test('admin models renders relay inventory and preserves filters in the built app', async ({ page }) => {
  await page.goto('/admin/models');

  await expect(page.getByRole('heading', { name: 'Models' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Models' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByText('2 relay models matched')).toBeVisible();
  await expect(page.getByText('gpt-4o')).toBeVisible();
  await expect(page.getByText('claude-3-5-sonnet')).toBeVisible();
  await expect(page.getByText('openai').first()).toBeVisible();
  await expect(page.getByText('azure', { exact: true })).toBeVisible();
  await expect(page.getByText('enterprise').first()).toBeVisible();
  await expect(page.getByText('OpenAI Browser Primary')).toBeVisible();
  await expect(page.getByText('Azure Browser Fallback')).toBeVisible();
  await expect(page.getByText('2 / 3 enabled')).toBeVisible();
  await expect(page.getByText('$0.0020 - $0.0060')).toBeVisible();
  await expect(page.getByText('1.18x')).toBeVisible();
  await expect(page.getByText('1,280')).toBeVisible();
  await expect(page.getByText('$12.4800')).toBeVisible();
  await expect(page.getByText('$7.1000')).toBeVisible();
  await expect(page.getByText('$5.3800')).toBeVisible();

  await page.getByLabel('Provider filter').fill('openai');
  await page.getByLabel('Group filter').fill('enterprise');
  await page.getByLabel('Status filter').fill('enabled');
  await page.getByLabel('Search models').fill('gpt-4');

  await expect(page.getByText('1 relay models matched')).toBeVisible();
  await expect(page.getByText('OpenAI Enterprise Browser')).toBeVisible();
  await expect(page.getByText('Claude Browser Primary')).toHaveCount(0);

  const sortResponse = page.waitForResponse((response) => response.url().includes('/api/v1/admin/models') && response.status() === 200);
  await page.getByRole('button', { name: 'Sort by Requests' }).click();
  await sortResponse;

  await expect(page.getByText('2,560')).toBeVisible();
  await expect(page.getByText('$18.2000')).toBeVisible();
  await expect(page.getByText('$10.5000')).toBeVisible();
  await expect(page.getByText('$7.7000')).toBeVisible();
});
