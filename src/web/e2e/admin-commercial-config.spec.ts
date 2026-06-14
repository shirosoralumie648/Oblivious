import { expect, test } from '@playwright/test';

import { registerAdminCommercialConfigRoutes } from './fixtures/adminCommercialConfig';

test.beforeEach(async ({ page }) => {
  await registerAdminCommercialConfigRoutes(page);
});

test('admin plans preserve request-token caps in the browser', async ({ page }) => {
  await page.goto('/admin/plans');

  await expect(page.getByRole('heading', { name: 'Plans' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Plans' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByText('Starter Browser')).toBeVisible();

  await page.getByPlaceholder('Search plans...').fill('browser plan');
  await page.getByLabel('Plan status filter').selectOption('active');
  await page.getByLabel('Plan visibility filter').selectOption('public');
  await expect(page.getByText('Browser Growth')).toBeVisible();
  await expect(page.getByText('32,000 tokens')).toBeVisible();

  await page.getByRole('button', { name: 'Add Plan' }).click();
  await page.getByLabel('Name').fill('Browser Enterprise');
  await page.getByLabel('Description').fill('Browser proof plan with per-request token cap');
  await page.getByLabel('Price').fill('199');
  await page.getByLabel('Quota Amount').fill('120000');
  await page.getByLabel('Token Quota').fill('2000000');
  await page.getByLabel('Agent Limit').fill('25');
  await page.getByLabel('Request Token Cap').fill('64000');
  await page.getByLabel('Model Access').fill('gpt-4o, claude-3-5-sonnet');
  await page.getByLabel('Duration Days').fill('30');
  await page.getByLabel('Sort Order').fill('7');
  await page.getByRole('button', { name: 'Create Plan' }).click();

  await expect(page.getByText('Browser Enterprise')).toBeVisible();
  await expect(page.getByText('64,000 tokens')).toBeVisible();
  await expect(page.getByText('$199.00')).toBeVisible();
});

test('admin users update commercial entitlements and account status in the browser', async ({ page }) => {
  await page.goto('/admin/users');

  await expect(page.getByRole('heading', { name: 'Users' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Users' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByText('buyer-browser@example.com')).toBeVisible();

  await page.getByPlaceholder('Search users...').fill('buyer_browser');
  await page.getByLabel('Role filter').selectOption('user');
  await page.getByLabel('User status filter').selectOption('active');
  await expect(page.getByText('Browser Growth')).toBeVisible();
  await expect(page.getByText('18,000 tokens / 240 calls / $12.50')).toBeVisible();

  await page.getByRole('button', { name: 'Edit user buyer-browser@example.com' }).click();
  await page.getByLabel('Role', { exact: true }).selectOption('admin');
  await page.getByLabel('Plan ID').fill('plan_browser_enterprise');
  await page.getByLabel('Status', { exact: true }).selectOption('disabled');
  await page.getByRole('button', { name: 'Save User' }).click();

  await expect(page.getByText('Browser Enterprise')).toBeVisible();
  await expect(page.getByLabel('Disabled', { exact: true })).toBeVisible();

  await page.getByRole('button', { name: 'Enable user buyer-browser@example.com' }).click();
  await expect(page.getByLabel('Active', { exact: true })).toBeVisible();
});

test('admin settings save relay pricing and usage-limit runtime controls in the browser', async ({ page }) => {
  await page.goto('/admin/settings');

  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Settings' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('button', { name: 'Edit organization org_browser_settings request_tokens minute' })).toBeVisible();
  await expect(page.getByText('Enforcing')).toBeVisible();
  await expect(page.getByText('1 recent hit - relay_rate_limited')).toBeVisible();
  await expect(page.getByText('Recovery: req_browser_recovered')).toBeVisible();

  await page.getByLabel('Model multipliers JSON').fill(JSON.stringify({ 'gpt-4o': 1.35, 'claude-3-5-sonnet': 1.6 }, null, 2));
  await page.getByLabel('Group multipliers JSON').fill(JSON.stringify({ enterprise: 0.85, vip: 0.75 }, null, 2));
  await page.getByRole('button', { name: 'Save Settings' }).click();
  await expect(page.getByText('Settings saved.')).toBeVisible();

  await page.getByRole('button', { name: 'Edit organization org_browser_settings request_tokens minute' }).click();
  await page.getByLabel('Limit value').fill('8192');
  await page.getByRole('button', { name: 'Save Usage Limit' }).click();
  await expect(page.getByText('Usage limit saved.')).toBeVisible();
  await expect(page.getByText('8192')).toBeVisible();
});
