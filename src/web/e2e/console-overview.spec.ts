import { expect, test } from '@playwright/test';

import { registerConsoleOverviewRoutes } from './fixtures/consoleOverview';

test.beforeEach(async ({ page }) => {
  await registerConsoleOverviewRoutes(page);
});

test('console overview renders drill-down summaries and models in the built app', async ({ page }) => {
  await page.goto('/console');

  await expect(page.getByRole('heading', { name: 'Console' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Console Home' })).toBeVisible();
  await expect(page.getByText('Current workspace scope: workspace_console_overview')).toBeVisible();

  await expect(page.getByRole('link', { name: 'Estimated cost' })).toHaveAttribute('href', '/console/billing');
  await expect(page.getByRole('link', { name: 'Estimated cost' }).getByText('$12.3456')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Requests' })).toHaveAttribute('href', '/console/usage');
  await expect(page.getByRole('link', { name: 'Requests' }).getByText('9')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Top model' })).toHaveAttribute('href', '/console/models');
  await expect(page.getByRole('link', { name: 'Top model' }).getByText('balanced-chat')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Access posture' })).toHaveAttribute('href', '/console/access');
  await expect(page.getByRole('link', { name: 'Access posture' }).getByText('Session session_console_overview')).toBeVisible();
  await expect(page.getByText('Active user: overview-operator@example.com')).toBeVisible();

  await page.getByRole('link', { name: 'Top model' }).click();

  await expect(page).toHaveURL(/\/console\/models$/);
  await expect(page.getByRole('link', { name: 'Models' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Models' })).toBeVisible();
  await expect(page.getByText('Workspace: workspace_console_overview')).toBeVisible();
  await expect(page.getByText('balanced-chat')).toBeVisible();
  await expect(page.getByText('Requests: 6')).toBeVisible();
  await expect(page.getByText('quality-chat')).toBeVisible();
  await expect(page.getByText('Requests: 3')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Open access' })).toHaveAttribute('href', '/console/access');
});

test('console models fixture rejects unexpected model query params', async ({ page }) => {
  await page.goto('/console/models');

  const response = await page.evaluate(async () => {
    const result = await fetch('/api/v1/console/models?period=7d');

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
      message: 'console models query params must be empty',
    },
  });
});
