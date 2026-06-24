import { expect, test, type Page } from '@playwright/test';

import { registerAdminUsersRoutes } from './fixtures/adminUsers';

test.beforeEach(async ({ page }) => {
  await registerAdminUsersRoutes(page);
});

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

test('admin users filters by plan ID in the browser', async ({ page }) => {
  await page.goto('/admin/users');

  await expect(page.getByRole('link', { name: 'Users' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Users' })).toBeVisible();
  await expect(page.getByText('initial-user@example.com')).toBeVisible();

  await page.getByLabel('Plan ID filter').fill('plan_enterprise_browser');

  await expect(page.getByText('plan-filtered@example.com')).toBeVisible();
  await expect(page.getByText('Enterprise Browser')).toBeVisible();
  await expect(page.getByText('9,500 quota')).toBeVisible();
});

test('admin users mobile layout keeps long tenant and entitlement evidence contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/admin/users');

  await expect(page.getByRole('link', { name: 'Users' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Users' })).toBeVisible();
  await expect(page.getByRole('main')).toHaveCount(1);

  await page.getByLabel('Plan ID filter').fill('plan_browser_users_mobile_without_breaks_20260624_primary');

  const mobileEmail = 'browserusersmobilewithoutbreaks20260624primary@example.com';
  await expect(page.getByText(mobileEmail)).toBeVisible();
  await expect(page.getByText('browserusersmobilewithoutbreaks20260624primaryplan')).toBeVisible();
  await expect(page.getByText('987,654.32 quota')).toBeVisible();
  await expect(page.getByText('1,234,567 tokens / 98,765 calls / $4321.09')).toBeVisible();

  await page.getByRole('button', { name: `Edit user ${mobileEmail}` }).click();
  await expect(page.getByRole('heading', { name: `Edit User: ${mobileEmail}` })).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'Plan ID' })).toHaveValue('plan_browser_users_mobile_without_breaks_20260624_primary');

  await expectNoHorizontalOverflow(page);
});
