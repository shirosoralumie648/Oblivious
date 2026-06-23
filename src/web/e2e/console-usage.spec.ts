import { expect, test } from '@playwright/test';

import { registerConsoleUsageRoutes } from './fixtures/consoleUsage';

test.beforeEach(async ({ page }) => {
  await registerConsoleUsageRoutes(page);
});

test('console usage fixture rejects unexpected usage query params', async ({ page }) => {
  await page.goto('/console/usage');

  const response = await page.evaluate(async () => {
    const result = await fetch('/api/v1/console/usage?period=30d');

    return {
      status: result.status,
      body: await result.json(),
    };
  });

  expect(response.status).toBe(422);
  expect(response.body).toMatchObject({
    ok: false,
    data: null,
    error: {
      code: 'fixture_contract_mismatch',
      message: 'console usage query params must be empty',
    },
  });
});

test('console usage renders current workspace usage in the built app', async ({ page }) => {
  await page.goto('/console/usage');

  await expect(page.getByRole('heading', { name: 'Console' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Usage' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Usage' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Current workspace scope' })).toBeVisible();
  await expect(page.getByText('Workspace: workspace_console_usage')).toBeVisible();
  await expect(page.getByText('usage-operator@example.com').first()).toBeVisible();

  await expect(page.getByText('Requests: 7')).toBeVisible();
  await expect(page.getByText('Period: 7d')).toBeVisible();

  const byModel = page.getByRole('region', { name: 'By model' });
  await expect(byModel.getByRole('heading', { name: 'By model' })).toBeVisible();
  await expect(byModel.getByText('gpt-4o')).toBeVisible();
  await expect(byModel.getByText('4 req')).toBeVisible();
  await expect(byModel.getByText('4,200 tokens')).toBeVisible();
  await expect(byModel.getByText('$0.8400')).toBeVisible();

  const byFeature = page.getByRole('region', { name: 'By feature' });
  await expect(byFeature.getByText('chat')).toBeVisible();
  await expect(byFeature.getByText('2,400 tokens')).toBeVisible();

  const topUsers = page.getByRole('region', { name: 'Top users' });
  await expect(topUsers.getByText('usage-operator@example.com')).toBeVisible();
  await expect(topUsers.getByText('$1.3200')).toBeVisible();

  const dailyTrend = page.getByRole('region', { name: 'Daily trend' });
  await expect(dailyTrend.getByText('2026-06-17')).toBeVisible();
  await expect(dailyTrend.getByText('6,600 tokens')).toBeVisible();

  await expect(page.getByRole('heading', { name: 'Recent relay requests' })).toBeVisible();
  await expect(page.getByText('req_console_usage')).toBeVisible();
  await expect(page.getByText('tok_console_usage')).toBeVisible();
  await expect(page.getByText('success')).toBeVisible();
  await expect(page.getByText('1500 tokens')).toBeVisible();
  await expect(page.getByText('$0.4200')).toBeVisible();
  await expect(page.getByText('95 ms')).toBeVisible();
  await expect(page.getByText('openai')).toHaveCount(0);
  await expect(page.getByText('channel_openai_primary')).toHaveCount(0);
});
