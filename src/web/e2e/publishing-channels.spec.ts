import { expect, test } from '@playwright/test';

import { registerPublishingChannelsRoutes } from './fixtures/publishingChannels';

test.beforeEach(async ({ page }) => {
  await registerPublishingChannelsRoutes(page);
});

test('publishing channels browser journey covers create edit fallback retry send and delete', async ({ page }) => {
  await page.goto('/publishing');

  await expect(page.getByRole('heading', { name: 'Publishing Channels' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Publishing' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('navigation', { name: 'Workspace navigation' })).toBeVisible();

  const channelList = page.getByLabel('Publishing channel list');
  await expect(channelList).toContainText('Ops Webhook');
  await expect(channelList).toContainText('Degraded');
  await expect(channelList).toContainText('Fallback Slack');
  await expect(page.getByLabel('Publishing channel message visibility')).toContainText('channel_message_failed_browser');

  await page.getByLabel('Channel name').fill(' Browser Webhook ');
  await page.getByLabel('Channel type').selectOption('webhook');
  await page.getByLabel('Endpoint URL').fill(' https://hooks.example/browser ');
  await page.getByLabel('Shared secret').fill(' browser-secret ');
  await page.getByRole('button', { name: 'Create channel' }).click();

  await expect(channelList).toContainText('Browser Webhook');
  await expect(channelList).toContainText('https://hooks.example/browser');
  await expect(page.getByLabel('Channel name')).toHaveValue('');

  const opsRow = channelList.locator('li').filter({ hasText: 'Ops Webhook' });
  await opsRow.getByRole('button', { name: 'Edit Ops Webhook' }).click();
  const editForm = page.getByRole('form', { name: 'Edit Ops Webhook' });
  await editForm.getByLabel('Channel name').fill(' Ops Webhook Edited ');
  await editForm.getByLabel('Channel status').selectOption('active');
  await editForm.getByLabel('Endpoint URL').fill(' https://hooks.example/ops-edited ');
  await editForm.getByRole('button', { name: 'Save channel' }).click();

  await expect(channelList).toContainText('Ops Webhook Edited');
  await expect(channelList).toContainText('https://hooks.example/ops-edited');
  await expect(channelList).toContainText('Channel updated.');

  const sendTest = page.getByRole('region', { name: 'Publishing channel send test' });
  await sendTest.getByRole('combobox', { name: 'Channel' }).selectOption('channel_ops_webhook');
  await expect(page.getByLabel('Publishing channel message visibility')).toContainText('channel_message_failed_browser');
  await sendTest.getByLabel('Conversation ID').fill(' conversation_browser_publishing ');
  await sendTest.getByLabel('Message text').fill(' Delivery recovered ');
  await sendTest.getByRole('button', { name: 'Send message' }).click();
  await expect(channelList).toContainText('Last send: recorded');

  const retryControls = page.getByRole('form', { name: 'Failed retry queue controls' });
  await retryControls.getByLabel('Fallback channel').selectOption('channel_fallback_slack');
  await retryControls.getByLabel('Retry limit').fill('5');
  await retryControls.getByRole('button', { name: 'Switch queue to fallback' }).click();
  await expect(page.getByText('Retry result: claimed 1, succeeded 1, failed 0, permanent failures 0')).toBeVisible();
  await expect(page.getByLabel('Publishing channel message visibility')).toContainText('channel_message_recovered_browser');

  const editedRow = channelList.locator('li').filter({ hasText: 'Ops Webhook Edited' });
  await editedRow.getByRole('button', { name: 'Delete Ops Webhook Edited' }).click();
  await expect(editedRow).toContainText('Disabled');
  await expect(editedRow).toContainText('Channel disabled.');
});
