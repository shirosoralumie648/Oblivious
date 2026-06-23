import { expect, type Page, test } from '@playwright/test';

import { registerConsoleAccessRoutes } from './fixtures/consoleAccess';

async function expectConsoleAccessLayoutContained(page: Page) {
  const overflowState = await page.evaluate(() => {
    const main = document.querySelector('main');
    const consoleCanvas = document.querySelector('.console-canvas');

    return {
      documentFits: document.documentElement.scrollWidth <= window.innerWidth + 1,
      mainFits: main instanceof HTMLElement ? main.scrollWidth <= main.clientWidth + 1 : true,
      consoleCanvasFits: consoleCanvas instanceof HTMLElement ? consoleCanvas.scrollWidth <= consoleCanvas.clientWidth + 1 : true,
    };
  });

  expect(overflowState).toEqual({
    documentFits: true,
    mainFits: true,
    consoleCanvasFits: true,
  });
}

test.beforeEach(async ({ page }) => {
  await registerConsoleAccessRoutes(page);
});

test('console access creates API token views sanitized usage and revokes token', async ({ page }) => {
  await page.goto('/console/access');

  await expect(page.getByRole('heading', { name: 'Console' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Access' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Access' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Current workspace scope' })).toBeVisible();
  await expect(page.getByText('User: access-operator@example.com')).toBeVisible();

  await expect(page.getByRole('heading', { name: 'API tokens' })).toBeVisible();
  await expect(page.getByText('CI gateway key')).toBeVisible();
  await expect(page.getByText('obv_ci_123')).toBeVisible();
  await expect(page.getByText('2.5 / 10 quota')).toBeVisible();

  await page.getByLabel('Token name').fill('Browser key');
  await page.getByLabel('Allowed models').fill('gpt-4o-mini, gpt-4.1-mini');
  await page.getByLabel('Routing group').fill('vip');
  await page.getByLabel('Quota limit').fill('25.5');
  await page.getByLabel('Expires at').fill('2026-06-30T00:00:00Z');
  await page.getByRole('button', { name: 'Create API token' }).click();

  await expect(page.getByText('obv_browser_raw_secret')).toBeVisible();
  await expect(page.getByText('Browser key')).toBeVisible();
  await expect(page.getByText('obv_browser', { exact: true })).toBeVisible();
  await expect(page.getByText('vip')).toBeVisible();
  await expect(page.getByText('gpt-4o-mini, gpt-4.1-mini')).toBeVisible();
  await expect(page.getByText('0 / 25.5 quota')).toBeVisible();

  await page.getByRole('button', { name: 'View usage for CI gateway key' }).click();
  await expect(page.getByText('req_console_ci')).toBeVisible();
  await expect(page.getByText('chat', { exact: true })).toBeVisible();
  await expect(page.getByText('1100 tokens')).toBeVisible();
  await expect(page.getByText('$0.004')).toBeVisible();
  await expect(page.getByText('42 ms')).toBeVisible();
  await expect(page.getByText('openai')).toHaveCount(0);
  await expect(page.getByText('channel_openai_primary')).toHaveCount(0);

  const ciGatewayToken = page.getByRole('listitem').filter({ hasText: 'CI gateway key' });
  await ciGatewayToken.getByRole('button', { name: 'Revoke' }).click();
  await expect(ciGatewayToken.getByText('revoked', { exact: true })).toBeVisible();
  await expect(page.getByText('Unable to revoke API token.')).toHaveCount(0);
});

test('console access keeps mobile token controls and model limits contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/console/access');

  const main = page.getByRole('main');
  await expect(page.getByRole('heading', { name: 'Access' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Access' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('main')).toHaveCount(1);
  await expect(main.getByText('Workspace: workspace_console_access')).toHaveCount(2);
  await expect(main.getByText('CI gateway key')).toBeVisible();
  await expect(main.getByText('providerresearchclusterultralongcontextmodel20260624previewwithunbrokenidentifier')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Create API token' })).toBeDisabled();
  await expectConsoleAccessLayoutContained(page);
});
