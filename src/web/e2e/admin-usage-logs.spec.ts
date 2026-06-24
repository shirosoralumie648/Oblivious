import { expect, test, type Page } from '@playwright/test';

import { registerAdminUsageLogsRoutes } from './fixtures/adminUsageLogs';

async function expectNoHorizontalOverflow(page: Page) {
  const overflowState = await page.evaluate(() => {
    const main = document.querySelector('main');
    const tableViewport = document.querySelector('[data-slot="table-container"]');
    const labelledSections = Array.from(document.querySelectorAll('section[aria-label]'));

    return {
      documentFits: document.documentElement.scrollWidth <= window.innerWidth + 1,
      mainFits: main instanceof HTMLElement ? main.scrollWidth <= main.clientWidth + 1 : true,
      tableViewportFits:
        tableViewport instanceof HTMLElement
          ? tableViewport.getBoundingClientRect().width <= window.innerWidth + 1 &&
            window.getComputedStyle(tableViewport).overflowX === 'auto'
          : false,
      labelledSectionsFit: labelledSections.every((section) => section.getBoundingClientRect().width <= window.innerWidth + 1),
    };
  });

  expect(overflowState).toEqual({
    documentFits: true,
    mainFits: true,
    tableViewportFits: true,
    labelledSectionsFit: true,
  });
}

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

test('admin usage logs mobile layout keeps analytics and relay evidence contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/admin/usage-logs');

  await expect(page.getByRole('link', { name: 'Usage Logs' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Usage Logs' })).toBeVisible();
  await expect(page.getByRole('main')).toHaveCount(1);

  await page.getByLabel('Organization ID filter').fill('orgusagelogsmobilewithoutbreaks20260624');
  await page.getByLabel('User ID filter').fill('userusagelogsmobilewithoutbreaks20260624');
  await page.getByLabel('Request ID filter').fill('requsagelogsmobilewithoutbreaks20260624');
  await page.getByLabel('API token ID filter').fill('tokusagelogsmobilewithoutbreaks20260624');
  await page.getByLabel('API type filter').fill('chat');
  await page.getByLabel('Feature type filter').fill('workspacechatmobilewithoutbreaks20260624');
  await page.getByLabel('Quota mode filter').fill('relaybillingmobilewithoutbreaks20260624');
  await page.getByLabel('Channel ID filter').fill('channelusagelogsmobilewithoutbreaks20260624');
  await page.getByLabel('Provider filter').fill('providerusagelogsmobilewithoutbreaks20260624');
  await page.getByLabel('Status filter').fill('success');
  await page.getByLabel('Model filter').fill('modelusagelogsmobilewithoutbreaks20260624');
  await page.getByLabel('Analytics granularity filter').selectOption('week');

  await expect(page.getByText('requsagelogsmobilewithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('tokusagelogsmobilewithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('modelusagelogsmobilewithoutbreaks20260624').first()).toBeVisible();
  await expect(page.getByText('providerusagelogsmobilewithoutbreaks20260624 / channelusagelogsmobilewithoutbreaks20260624')).toBeVisible();
  await expect(page.getByLabel('By model')).toContainText('modelusagelogsmobilewithoutbreaks20260624');
  await expect(page.getByLabel('By channel')).toContainText('channelusagelogsmobilewithoutbreaks20260624');
  await expect(page.getByLabel('By provider')).toContainText('providerusagelogsmobilewithoutbreaks20260624');
  await expect(page.getByLabel('Cross dimensions')).toContainText('modelusagelogsmobilewithoutbreaks20260624 / 2026-W25');
  await expect(page.getByLabel('Cross dimensions')).toContainText('userusagelogsmobilewithoutbreaks20260624 / workspacechatmobilewithoutbreaks20260624');

  await expectNoHorizontalOverflow(page);
});
