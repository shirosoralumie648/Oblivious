import { expect, test } from '@playwright/test';

import { registerAdminUsersRoutes } from './fixtures/adminUsers';

test.beforeEach(async ({ page }) => {
  await registerAdminUsersRoutes(page);
});

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
