import { expect, test } from '@playwright/test';

import { registerSettingsRoutes } from './fixtures/settings';

test.beforeEach(async ({ page }) => {
  await registerSettingsRoutes(page);
});

test('settings saves workspace preferences without dropping extended preference fields', async ({ page }) => {
  await page.goto('/settings');

  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Settings' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByLabel('Default mode')).toHaveValue('chat');
  await expect(page.getByLabel('Model strategy')).toHaveValue('quality');
  await expect(page.getByLabel('Enable web suggestions')).toBeChecked();
  await expect(page.getByText('Oblivious Safe Builtins')).toBeVisible();

  await page.getByLabel('Default mode').selectOption('solo');
  await page.getByLabel('Model strategy').selectOption('cost');
  await page.getByLabel('Enable web suggestions').uncheck();
  await page.getByRole('button', { name: 'Save preferences' }).click();

  await expect(page.getByText('Preferences saved.')).toBeVisible();
});
