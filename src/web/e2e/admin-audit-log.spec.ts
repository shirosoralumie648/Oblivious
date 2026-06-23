import { expect, test } from '@playwright/test';

import { registerAdminAuditLogRoutes } from './fixtures/adminAuditLog';

test.beforeEach(async ({ page }) => {
  await registerAdminAuditLogRoutes(page);
});

test('admin audit log filters entries by organization ID in the browser', async ({ page }) => {
  await page.goto('/admin/audit-log');

  await expect(page.getByRole('heading', { name: 'Audit Log' })).toBeVisible();

  await page.getByLabel('Organization ID filter').fill('org_audit_browser');

  await expect(page.getByText('browser-audit@example.com')).toBeVisible();
  await expect(page.getByText('agent.approve')).toBeVisible();
  await expect(page.getByText('agent / agent_browser_audit')).toBeVisible();
});
