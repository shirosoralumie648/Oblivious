import { expect, test } from '@playwright/test';

import { registerConsoleNotificationsRoutes } from './fixtures/consoleNotifications';

test.beforeEach(async ({ page }) => {
  await registerConsoleNotificationsRoutes(page);
});

test('console notifications manages unread count and notification lifecycle', async ({ page }) => {
  await page.goto('/console/notifications');

  await expect(page.getByRole('heading', { name: 'Console' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Notifications' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Notifications' })).toBeVisible();
  await expect(page.getByText('Review in-app alerts routed from workspace and system events.')).toBeVisible();

  await expect(page.getByText('3 total')).toBeVisible();
  await expect(page.getByText('2 unread')).toBeVisible();

  const databaseNotification = page.getByRole('listitem').filter({ hasText: 'Database down' });
  await expect(databaseNotification.getByRole('heading', { name: 'Database down' })).toBeVisible();
  await expect(databaseNotification.getByText('Primary database heartbeat failed')).toBeVisible();
  await expect(databaseNotification.getByText('critical', { exact: true })).toBeVisible();
  await expect(databaseNotification.getByText('system', { exact: true })).toBeVisible();
  await expect(databaseNotification.getByText('Unread', { exact: true })).toBeVisible();

  const quotaNotification = page.getByRole('listitem').filter({ hasText: 'Quota near limit' });
  await expect(quotaNotification.getByText('billing', { exact: true })).toBeVisible();
  await expect(quotaNotification.getByText('warning', { exact: true })).toBeVisible();
  await expect(quotaNotification.getByText('Unread', { exact: true })).toBeVisible();

  const reportNotification = page.getByRole('listitem').filter({ hasText: 'Usage report ready' });
  await expect(reportNotification.getByText('reports', { exact: true })).toBeVisible();
  await expect(reportNotification.getByText('info', { exact: true })).toBeVisible();
  await expect(reportNotification.getByText('Unread', { exact: true })).toHaveCount(0);

  await databaseNotification.getByRole('button', { name: 'Mark Database down as read' }).click();
  await expect(page.getByText('1 unread')).toBeVisible();
  await expect(databaseNotification.getByRole('button', { name: 'Mark Database down as read' })).toHaveCount(0);
  await expect(databaseNotification.getByText('Unread', { exact: true })).toHaveCount(0);

  await page.getByRole('button', { name: 'Mark all read' }).click();
  await expect(page.getByText('0 unread')).toBeVisible();
  await expect(page.getByText('Unread', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Mark all read' })).toBeDisabled();

  await quotaNotification.getByRole('button', { name: 'Delete Quota near limit' }).click();
  await expect(page.getByText('2 total')).toBeVisible();
  await expect(page.getByText('Quota near limit')).toHaveCount(0);
  await expect(page.getByText('Unable to delete notification.')).toHaveCount(0);
});
