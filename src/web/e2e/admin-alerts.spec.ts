import { expect, test } from '@playwright/test';

import { registerAdminAlertsRoutes } from './fixtures/adminAlerts';

test.beforeEach(async ({ page }) => {
  await registerAdminAlertsRoutes(page);
});

test('admin alerts filters operational alerts and inspects delivery and recovery evidence', async ({ page }) => {
  await page.goto('/admin/alerts');

  await expect(page.getByRole('heading', { name: 'Alerts' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Alerts' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByText('Workflow failure rate')).toBeVisible();
  await expect(page.getByText('DAG success rate dropped below 99 percent.')).toBeVisible();

  await expect(page.getByLabel('Recovery actions')).toContainText('restart-relay');
  await expect(page.getByLabel('Recovery actions')).toContainText('Relay backlog recovery policy');

  await page.getByLabel('Severity').selectOption('critical');
  await page.getByLabel('Status').selectOption('acknowledged');
  await page.getByLabel('Component').fill('relay');
  await page.getByRole('button', { name: 'Apply alert filters' }).click();

  const relayRow = page.locator('article').filter({ hasText: 'Relay backlog' });
  await expect(relayRow.getByRole('heading', { name: 'Relay backlog' })).toBeVisible();
  await expect(relayRow).toContainText('Queue depth exceeded the recovery policy threshold.');
  await expect(relayRow).toContainText('acknowledged');

  await page.getByRole('button', { name: 'View deliveries Relay backlog' }).click();
  const history = page.getByLabel('Notification delivery history');
  await expect(history).toBeVisible();
  await expect(history).toContainText('alert_provider_smtp');
  await expect(history).toContainText('delivered');
  await expect(history).toContainText('alert_provider_slack');
  await expect(history).toContainText('im webhook failed');
});

test('admin alerts acknowledges and resolves alert state from the built app', async ({ page }) => {
  await page.goto('/admin/alerts');

  const workflowRow = page.locator('article').filter({ hasText: 'Workflow failure rate' });
  await expect(workflowRow).toContainText('Workflow failure rate');
  await expect(workflowRow).toContainText('open');

  await page.getByRole('button', { name: 'Acknowledge Workflow failure rate' }).click();
  await expect(workflowRow).toContainText('acknowledged');

  await page.getByRole('button', { name: 'Resolve Workflow failure rate' }).click();
  await expect(workflowRow).toContainText('resolved');
});

test('admin alerts saves routing and tests notification providers', async ({ page }) => {
  await page.goto('/admin/alerts');

  await expect(page.getByLabel('Notification routing')).toContainText('email + im');
  await page.getByLabel('Route warning alerts to sms').check();
  await page.getByRole('button', { name: 'Save notification routing' }).click();
  await expect(page.getByLabel('Notification routing')).toContainText('email + im + sms');

  const providers = page.getByLabel('Alert notification providers');
  await expect(providers).toContainText('Primary SMTP');
  await expect(providers).toContainText('smtp.example.com');
  await expect(providers).toContainText('********');

  await page.getByLabel('Provider name').fill('Slack Ops');
  await page.getByLabel('Provider kind').selectOption('slack_webhook');
  await page.getByLabel('Provider webhook URL').fill('https://hooks.slack.example/browser');
  await page.getByRole('button', { name: 'Create alert provider' }).click();

  await expect(providers).toContainText('Slack Ops');
  await expect(providers).toContainText('im');

  await page.getByRole('button', { name: 'Test provider Primary SMTP' }).click();
  await expect(page.getByText('Primary SMTP: provider configuration validated')).toBeVisible();
});
