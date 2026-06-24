import { expect, test, type Page } from '@playwright/test';

import { registerAdminAPITokensRoutes } from './fixtures/adminApiTokens';

async function expectNoHorizontalOverflow(page: Page) {
  const overflowState = await page.evaluate(() => {
    const main = document.querySelector('main');
    const tableViewport = document.querySelector('[data-slot="table-container"]');

    return {
      documentFits: document.documentElement.scrollWidth <= window.innerWidth + 1,
      mainFits: main instanceof HTMLElement ? main.scrollWidth <= main.clientWidth + 1 : true,
      tableViewportFits:
        tableViewport instanceof HTMLElement
          ? tableViewport.getBoundingClientRect().width <= window.innerWidth + 1 &&
            window.getComputedStyle(tableViewport).overflowX === 'auto'
          : false,
    };
  });

  expect(overflowState).toEqual({
    documentFits: true,
    mainFits: true,
    tableViewportFits: true,
  });
}

test.beforeEach(async ({ page }) => {
  await registerAdminAPITokensRoutes(page);
});

test('admin API tokens filter relay keys and revoke a scoped token in the browser', async ({ page }) => {
  await page.goto('/admin/api-tokens');

  await expect(page.getByRole('link', { name: 'API Tokens' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'API Tokens' })).toBeVisible();
  await expect(page.getByText('1 relay keys matched')).toBeVisible();
  await expect(page.getByText('Initial admin key')).toBeVisible();
  await expect(page.getByText('obv_admin_initial')).toBeVisible();

  await page.getByLabel('Organization ID filter').fill('org_browser_api_tokens');
  await page.getByLabel('User ID filter').fill('user_browser_api_tokens');
  await page.getByLabel('Status filter').fill('active');
  await page.getByLabel('User group filter').fill('enterprise');
  await page.getByLabel('Search tokens').fill('browser admin');
  await page.getByLabel('Model filter').fill('gpt-4o');

  await expect(page.getByText('Browser admin key')).toBeVisible();
  await expect(page.getByText('browser-admin@example.com')).toBeVisible();
  await expect(page.getByText('enterprise')).toBeVisible();
  await expect(page.getByText('org_browser_api_tokens')).toBeVisible();
  await expect(page.getByText('gpt-4o, gpt-4.1-mini')).toBeVisible();
  await expect(page.getByText('$3.2500 / $25.5000')).toBeVisible();
  await expect(page.getByText('$0.1288')).toBeVisible();
  await expect(page.getByText('1,234')).toBeVisible();

  page.once('dialog', async (dialog) => {
    expect(dialog.message()).toContain('Browser admin key');
    await dialog.accept();
  });
  await page.getByRole('button', { name: 'Revoke Browser admin key' }).click();

  await expect(page.getByLabel('Revoked')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Revoke Browser admin key' })).toBeDisabled();
});

test('admin API tokens mobile layout keeps token evidence contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/admin/api-tokens');

  await expect(page.getByRole('link', { name: 'API Tokens' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'API Tokens' })).toBeVisible();
  await expect(page.getByRole('main')).toHaveCount(1);

  await page.getByLabel('Organization ID filter').fill('orgapitokensmobilewithoutbreaks20260624');
  await page.getByLabel('User ID filter').fill('userapitokensmobilewithoutbreaks20260624');
  await page.getByLabel('Status filter').fill('active');
  await page.getByLabel('User group filter').fill('enterpriseapitokensmobilewithoutbreaks20260624');
  await page.getByLabel('Search tokens').fill('mobileadmintokenwithoutbreaks20260624');
  await page.getByLabel('Model filter').fill('modelapitokensmobilewithoutbreaks20260624');

  await expect(page.getByText('mobileapitokennamewithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('obvadminmobileprefixwithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('mobileapitokensuserwithoutbreaks20260624@example.com')).toBeVisible();
  await expect(page.getByText('enterpriseapitokensmobilewithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('orgapitokensmobilewithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('modelapitokensmobilewithoutbreaks20260624, backupmodelapitokensmobilewithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('$9.7500 / $99.5000')).toBeVisible();
  await expect(page.getByText('$4.5678')).toBeVisible();
  await expect(page.getByText('9,876')).toBeVisible();

  await expectNoHorizontalOverflow(page);
});
