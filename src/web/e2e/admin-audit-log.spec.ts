import { expect, test, type Page } from '@playwright/test';

import { registerAdminAuditLogRoutes } from './fixtures/adminAuditLog';

test.beforeEach(async ({ page }) => {
  await registerAdminAuditLogRoutes(page);
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

test('admin audit log filters entries by organization ID in the browser', async ({ page }) => {
  await page.goto('/admin/audit-log');

  await expect(page.getByRole('heading', { name: 'Audit Log' })).toBeVisible();

  await page.getByLabel('Organization ID filter').fill('org_audit_browser');

  await expect(page.getByText('browser-audit@example.com')).toBeVisible();
  await expect(page.getByText('agent.approve')).toBeVisible();
  await expect(page.getByText('agent / agent_browser_audit')).toBeVisible();
});

test('admin audit log mobile layout keeps long audit evidence contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/admin/audit-log');

  await expect(page.getByRole('link', { name: 'Audit Log' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Audit Log' })).toBeVisible();
  await expect(page.getByRole('main')).toHaveCount(1);

  await page.getByLabel('Organization ID filter').fill('org_audit_mobile_without_breaks_20260624_primary');

  await expect(page.getByText('auditlogmobileoperatorwithoutbreaks20260624@example.com')).toBeVisible();
  await expect(page.getByText('billing.refund.providerreconciliationmobilewithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('billingproviderrailwithoutbreaks / refundtopupmobileevidencewithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('changeevidencemobilewithoutbreaks20260624')).toBeVisible();

  await expectNoHorizontalOverflow(page);
});
