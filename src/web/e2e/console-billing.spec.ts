import { expect, type Page, test } from '@playwright/test';

import { registerConsoleBillingRoutes } from './fixtures/consoleBilling';

async function expectConsoleBillingLayoutContained(page: Page) {
  const overflowState = await page.evaluate(() => {
    const main = document.querySelector('main');
    const table = document.querySelector('table');
    const tableViewport = table?.parentElement;

    return {
      documentFits: document.documentElement.scrollWidth <= window.innerWidth + 1,
      mainFits: main instanceof HTMLElement ? main.scrollWidth <= main.clientWidth + 1 : true,
      tableViewportFits:
        tableViewport instanceof HTMLElement ? tableViewport.getBoundingClientRect().width <= window.innerWidth + 1 : true,
    };
  });

  expect(overflowState).toEqual({
    documentFits: true,
    mainFits: true,
    tableViewportFits: true,
  });
}

test.beforeEach(async ({ page }) => {
  await registerConsoleBillingRoutes(page);
});

test('console billing starts subscription checkout with selected package and provider', async ({ page }) => {
  await page.goto('/console/billing');

  await expect(page.getByRole('heading', { name: 'Console', exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Billing' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Billing' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Current workspace scope' })).toBeVisible();
  await expect(page.getByText('Workspace: workspace_console_billing')).toBeVisible();

  await expect(page.getByText('Balance: $42.50')).toBeVisible();
  const subscriptionRegion = page.getByRole('region', { name: 'Subscription packages' });
  await expect(subscriptionRegion.getByRole('heading', { name: 'Subscription packages' })).toBeVisible();
  const packageSelect = subscriptionRegion.getByLabel('Package', { exact: true });
  await expect(packageSelect).toHaveValue('pkg_starter');
  await expect(packageSelect.locator('option')).toHaveText([
    'Starter - $12.00 - 30 days',
    'Pro - $29.00 - 30 days'
  ]);

  await packageSelect.selectOption('pkg_pro');
  await expect(page.getByText('Quota credit: $150.00')).toBeVisible();
  await expect(page.getByText('Token quota: 1,500,000')).toBeVisible();
  await expect(page.getByText('Agent limit: 25')).toBeVisible();
  await expect(page.getByText('Max tokens per request: 32,000')).toBeVisible();

  await expect(subscriptionRegion.getByLabel('Subscription payment provider')).toHaveValue('stripe');
  await expect(subscriptionRegion.getByLabel('Subscription payment provider')).toContainText('WeChat Pay');
  await subscriptionRegion.getByLabel('Subscription payment provider').selectOption('wechatpay');
  await subscriptionRegion.getByRole('button', { name: 'Start subscription checkout' }).click();

  await expect(page.getByRole('link', { name: 'Continue WeChat Pay checkout' })).toHaveAttribute(
    'href',
    'https://checkout.wechatpay.test/session/cs_subscription_browser'
  );
  await expect(page.getByText('Unable to start subscription checkout.')).toHaveCount(0);
});

test('console billing starts top-up checkout with selected amount and provider', async ({ page }) => {
  await page.goto('/console/billing');

  const topUpRegion = page.getByRole('region', { name: 'Quota top-up checkout' });
  await expect(topUpRegion.getByRole('heading', { name: 'Add balance' })).toBeVisible();
  await expect(topUpRegion.getByLabel('Top-up amount USD')).toHaveValue('25');
  await topUpRegion.getByLabel('Top-up amount USD').fill('37.50');
  await expect(topUpRegion.getByLabel('Payment provider')).toHaveValue('stripe');
  await expect(topUpRegion.getByLabel('Payment provider')).toContainText('WeChat Pay');
  await topUpRegion.getByLabel('Payment provider').selectOption('wechatpay');
  await topUpRegion.getByRole('button', { name: 'Start top-up checkout' }).click();

  await expect(page.getByRole('link', { name: 'Continue WeChat Pay checkout' })).toHaveAttribute(
    'href',
    'https://checkout.wechatpay.test/session/cs_topup_browser'
  );
  await expect(page.getByText('Unable to start top-up checkout.')).toHaveCount(0);
});

test('console billing keeps mobile layout and billing controls contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/console/billing');

  await expect(page.getByRole('heading', { name: 'Billing' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Billing' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('main')).toHaveCount(1);
  await expect(page.getByText('Workspace: workspace_console_billing')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Subscription packages' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Invoice history' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Start subscription checkout' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Start top-up checkout' })).toBeVisible();
  await expectConsoleBillingLayoutContained(page);
});
