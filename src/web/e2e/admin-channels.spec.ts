import { expect, test } from '@playwright/test';

import { registerAdminChannelsRoutes } from './fixtures/adminChannels';

test.beforeEach(async ({ page }) => {
  await registerAdminChannelsRoutes(page);
});

test('admin channels create edit diagnose and batch in the built app', async ({ page }) => {
  await page.goto('/admin/channels');

  await expect(page.getByRole('heading', { name: 'Channels' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Channels' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('cell', { name: 'OpenAI Browser Primary', exact: true })).toBeVisible();
  await expect(page.getByRole('cell', { name: '17 RPM 2,048 TPM' })).toBeVisible();
  await expect(page.getByRole('cell', { name: '112ms avg' })).toBeVisible();
  await expect(page.getByLabel('Runtime diagnostics').getByText('5 active')).toBeVisible();

  await page.getByRole('button', { name: 'Add Channel' }).click();
  const addDrawer = page.getByRole('dialog', { name: 'Add Channel' });
  await expect(addDrawer).toBeVisible();
  await addDrawer.getByLabel('Name', { exact: true }).fill('Browser OpenRouter');
  await addDrawer.getByLabel('Provider').selectOption('openrouter');
  await addDrawer.getByLabel('API Key').fill('sk-browser-openrouter');
  await addDrawer.getByLabel('Base URL').fill('https://openrouter.browser.test/api/v1');
  await addDrawer.getByLabel('Models').fill('gpt-4o, gpt-4.1-mini');
  await addDrawer.getByLabel('Groups').fill('default, enterprise');
  await addDrawer.getByLabel('RPM Limit').fill('240');
  await addDrawer.getByLabel('TPM Limit').fill('240000');
  await addDrawer.getByLabel('Estimated Cost per 1K').fill('0.0042');
  await addDrawer.getByLabel('Cost Multiplier').fill('1.15');
  await addDrawer.getByLabel('Priority').fill('2');
  await addDrawer.getByLabel('Weight').fill('80');
  await addDrawer.getByRole('button', { name: 'Create Channel' }).click();

  await expect(page.getByRole('cell', { name: 'Browser OpenRouter', exact: true })).toBeVisible();
  await expect(page.getByText('0.0042')).toBeVisible();

  await page.getByRole('button', { name: 'Edit channel Browser OpenRouter' }).click();
  const editDrawer = page.getByRole('dialog', { name: 'Edit Channel' });
  await expect(editDrawer).toBeVisible();
  await editDrawer.getByLabel('Name', { exact: true }).fill('Browser OpenRouter Updated');
  await editDrawer.getByLabel('Base URL').fill('https://openrouter.browser.test/api/v2');
  await editDrawer.getByLabel('Models').fill('gpt-4o, gpt-4.1-mini, gpt-4.1');
  await editDrawer.getByLabel('Groups').fill('default, enterprise, qa');
  await editDrawer.getByLabel('RPM Limit').fill('360');
  await editDrawer.getByLabel('TPM Limit').fill('360000');
  await editDrawer.getByLabel('Estimated Cost per 1K').fill('0.0037');
  await editDrawer.getByLabel('Cost Multiplier').fill('1.05');
  await editDrawer.getByLabel('Priority').fill('1');
  await editDrawer.getByLabel('Weight').fill('90');
  await editDrawer.getByRole('button', { name: 'Save Changes' }).click();

  await expect(page.getByRole('cell', { name: 'Browser OpenRouter Updated', exact: true })).toBeVisible();
  await expect(page.getByText('0.0037')).toBeVisible();

  await page.getByRole('button', { name: 'Test connection for Browser OpenRouter Updated' }).click();
  const diagnostics = page.getByRole('region', { name: 'Browser OpenRouter Updated diagnostics' });
  await expect(diagnostics).toBeVisible();
  await expect(diagnostics.getByText('USD 48.75')).toBeVisible();
  await expect(diagnostics.getByText('Health online')).toBeVisible();

  await page.getByRole('button', { name: 'Detect model updates for Browser OpenRouter Updated' }).click();
  const updates = page.getByRole('region', { name: 'Model updates for Browser OpenRouter Updated' });
  await expect(updates).toBeVisible();
  await expect(updates.getByText('Added 1')).toBeVisible();
  await expect(updates.getByText('gpt-4.1-nano')).toBeVisible();

  const applyResponse = page.waitForResponse((response) =>
    response.url().includes('/api/v1/admin/channels/channel_browser_openrouter/model-updates/apply') &&
    response.status() === 200
  );
  await page.getByRole('button', { name: 'Apply model updates for Browser OpenRouter Updated' }).click();
  await applyResponse;
  await expect(updates.getByText('gpt-4.1-nano')).toBeVisible();

  await page.getByRole('checkbox', { name: 'Select row OpenAI Browser Primary' }).check();
  await expect(page.getByText('1 selected')).toBeVisible();
  await page.getByRole('button', { name: 'Batch Disable' }).click();
  await expect(page.getByLabel('Offline')).toBeVisible();
});
