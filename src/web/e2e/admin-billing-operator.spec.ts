import { expect, type Page, test } from '@playwright/test';

import { registerAdminBillingOperatorRoutes } from './fixtures/adminBillingOperator';

async function expectAdminBillingLayoutContained(page: Page) {
  const overflowState = await page.evaluate(() => {
    const main = document.querySelector('main');
    const table = document.querySelector('table');
    const tableViewport = table?.parentElement;

    return {
      documentFits: document.documentElement.scrollWidth <= window.innerWidth + 1,
      mainFits: main instanceof HTMLElement ? main.scrollWidth <= main.clientWidth + 1 : true,
      tableViewportFits:
        tableViewport instanceof HTMLElement ? tableViewport.getBoundingClientRect().width <= window.innerWidth + 1 : false,
    };
  });

  expect(overflowState).toEqual({
    documentFits: true,
    mainFits: true,
    tableViewportFits: true,
  });
}

test.beforeEach(async ({ page }) => {
  await registerAdminBillingOperatorRoutes(page);
});

test('admin billing mobile layout keeps operator tables and forms contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/admin/billing');

  await expect(page.getByRole('heading', { name: 'Billing' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Billing' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('main')).toHaveCount(1);
  await expect(page.getByText('session_admin_billing_initial')).toBeVisible();
  await expectAdminBillingLayoutContained(page);

  await page.getByLabel('Organization ID filter').fill('org_billing_operator');
  await page.getByLabel('Status filter').fill('payout_pending');
  await page.getByLabel('Kind filter').fill('marketplace_install');
  await page.getByLabel('Provider filter').fill('stripe_connect');
  await page.getByRole('tab', { name: 'Payouts' }).click();

  const failedPayoutRow = page.getByRole('row').filter({ hasText: 'payout_browser_failed' });
  await expect(failedPayoutRow).toBeVisible();
  await expect(failedPayoutRow.getByLabel('Payout Pending', { exact: true })).toBeVisible();
  await expectAdminBillingLayoutContained(page);

  await failedPayoutRow.getByRole('button', { name: 'Mark payout payout_browser_failed failed' }).click();
  await expect(page.getByRole('heading', { name: 'Payout failure' })).toBeVisible();
  await expect(page.getByLabel('Provider payout ID')).toBeVisible();
  await expect(page.getByLabel('Failure reason')).toBeVisible();
  await expectAdminBillingLayoutContained(page);

  await page.getByLabel('Provider payout ID').fill('po_browser_failed_confirmed');
  await page.getByLabel('Failure reason').fill('bank account closed by publisher');
  await page.getByRole('button', { name: 'Confirm failed payout' }).click();
  await expect(failedPayoutRow.getByLabel('Failed', { exact: true })).toBeVisible();
  await expectAdminBillingLayoutContained(page);
});

test('admin billing marks marketplace payouts paid and failed with provider evidence', async ({ page }) => {
  await page.goto('/admin/billing');

  await expect(page.getByRole('heading', { name: 'Billing' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Billing' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByText('session_admin_billing_initial')).toBeVisible();

  await page.getByLabel('Organization ID filter').fill('org_billing_operator');
  await page.getByLabel('Status filter').fill('payout_pending');
  await page.getByLabel('Kind filter').fill('marketplace_install');
  await page.getByLabel('Provider filter').fill('stripe_connect');
  await page.getByRole('tab', { name: 'Payouts' }).click();

  const paidPayoutRow = page.getByRole('row').filter({ hasText: 'payout_browser_paid' });
  const failedPayoutRow = page.getByRole('row').filter({ hasText: 'payout_browser_failed' });
  await expect(paidPayoutRow).toBeVisible();
  await expect(failedPayoutRow).toBeVisible();
  await expect(paidPayoutRow.getByLabel('Payout Pending', { exact: true })).toBeVisible();
  await expect(failedPayoutRow.getByLabel('Payout Pending', { exact: true })).toBeVisible();

  await paidPayoutRow.getByRole('button', { name: 'Mark payout payout_browser_paid paid' }).click();
  await expect(page.getByRole('heading', { name: 'Payout confirmation' })).toBeVisible();
  await page.getByLabel('Provider payout ID').fill('po_browser_paid_confirmed');
  await page.getByRole('button', { name: 'Confirm paid payout' }).click();

  await expect(paidPayoutRow.getByLabel('Paid Out', { exact: true })).toBeVisible();
  await expect(paidPayoutRow.getByText('po_browser_paid_confirmed')).toBeVisible();

  await failedPayoutRow.getByRole('button', { name: 'Mark payout payout_browser_failed failed' }).click();
  await expect(page.getByRole('heading', { name: 'Payout failure' })).toBeVisible();
  await page.getByLabel('Provider payout ID').fill('po_browser_failed_confirmed');
  await page.getByLabel('Failure reason').fill('bank account closed by publisher');
  await page.getByRole('button', { name: 'Confirm failed payout' }).click();

  await expect(failedPayoutRow.getByLabel('Failed', { exact: true })).toBeVisible();
  await expect(failedPayoutRow.getByText('po_browser_failed_confirmed')).toBeVisible();
  await expect(page.getByText('Unable to mark payout failed.')).toHaveCount(0);
});

test('admin billing records a provider-confirmed top-up refund in the browser', async ({ page }) => {
  await page.goto('/admin/billing');

  await expect(page.getByRole('heading', { name: 'Billing' })).toBeVisible();
  await page.getByLabel('Organization ID filter').fill('org_billing_operator');
  await page.getByLabel('User ID filter').fill('user_billing_operator');
  await page.getByLabel('Status filter').fill('paid');
  await page.getByLabel('Kind filter').fill('topup');
  await page.getByLabel('Provider filter').fill('stripe');
  await page.getByRole('tab', { name: 'Top-ups' }).click();

  const topupRow = page.getByRole('row').filter({ hasText: 'topup_browser_refund' });
  await expect(topupRow).toBeVisible();
  await expect(topupRow.getByText('$10.00')).toBeVisible();
  await expect(topupRow.getByLabel('Paid', { exact: true })).toBeVisible();

  await topupRow.getByRole('button', { name: 'Record refund for top-up topup_browser_refund' }).click();
  await expect(page.getByRole('heading', { name: 'Top-up refund' })).toBeVisible();
  await expect(page.getByLabel('Provider', { exact: true })).toHaveValue('stripe');
  await expect(page.getByLabel('Provider charge ID', { exact: true })).toHaveValue('ch_browser_refund_1');
  await expect(page.getByLabel('Provider payment intent ID', { exact: true })).toHaveValue('pi_browser_refund_1');
  await page.getByLabel('Provider refund ID', { exact: true }).fill('re_browser_refund_1');
  await page.getByLabel('Refund amount', { exact: true }).fill('12.5');
  await page.getByLabel('Reason', { exact: true }).fill('duplicate provider capture');
  await page.getByRole('button', { name: 'Confirm top-up refund' }).click();

  await expect(topupRow.getByText('$22.50')).toBeVisible();
  await expect(page.getByText('Unable to record top-up refund.')).toHaveCount(0);
});

test('admin billing records a domestic top-up refund without stripe charge evidence in the browser', async ({ page }) => {
  await page.goto('/admin/billing');

  await expect(page.getByRole('heading', { name: 'Billing' })).toBeVisible();
  await page.getByLabel('Organization ID filter').fill('org_billing_operator');
  await page.getByLabel('User ID filter').fill('user_billing_operator');
  await page.getByLabel('Status filter').fill('paid');
  await page.getByLabel('Kind filter').fill('topup');
  await page.getByLabel('Provider filter').fill('alipay');
  await page.getByRole('tab', { name: 'Top-ups' }).click();

  const topupRow = page.getByRole('row').filter({ hasText: 'topup_browser_domestic_refund' });
  await expect(topupRow).toBeVisible();
  await expect(topupRow.getByText('$5.00')).toBeVisible();
  await expect(topupRow.getByLabel('Paid', { exact: true })).toBeVisible();

  await topupRow.getByRole('button', { name: 'Record refund for top-up topup_browser_domestic_refund' }).click();
  await expect(page.getByRole('heading', { name: 'Top-up refund' })).toBeVisible();
  await expect(page.getByLabel('Provider', { exact: true })).toHaveValue('alipay');
  await expect(page.getByLabel('Provider charge ID', { exact: true })).toHaveValue('');
  await expect(page.getByLabel('Provider payment intent ID', { exact: true })).toHaveValue('alipay_pi_browser_refund_1');
  await expect(page.getByLabel('Currency', { exact: true })).toHaveValue('cny');

  await page.getByLabel('Provider refund ID', { exact: true }).fill('alipay_re_browser_refund_1');
  await page.getByLabel('Refund amount', { exact: true }).fill('7.5');
  await page.getByLabel('Reason', { exact: true }).fill('domestic refund confirmed by Alipay');
  await page.getByRole('button', { name: 'Confirm top-up refund' }).click();

  await expect(topupRow.getByText('$12.50')).toBeVisible();
  await expect(page.getByText('Unable to record top-up refund.')).toHaveCount(0);
});
