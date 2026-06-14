import { expect, test } from '@playwright/test';

import { registerAdminReviewsRoutes } from './fixtures/adminReviews';

test.beforeEach(async ({ page }) => {
  await registerAdminReviewsRoutes(page);
});

test('admin reviews requests publisher changes with SLA and governance context', async ({ page }) => {
  await page.goto('/admin/reviews');

  await expect(page.getByRole('heading', { name: 'Review Queue' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Review Queue' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByText('Browser Review Agent')).toBeVisible();
  await expect(page.getByText('Pricing: one_time $29.00')).toBeVisible();
  await expect(page.getByText('Governance status: pending_review')).toBeVisible();
  await expect(page.getByText('Manual SLA: Due soon by 2026-06-18 12:00 UTC')).toBeVisible();
  await expect(page.getByText('Automated SLA: Overdue')).toBeVisible();
  await expect(page.getByText('Publisher tier: vip')).toBeVisible();

  await page.getByRole('button', { name: 'Enforce SLA' }).click();
  await expect(page.getByText('Review SLA scan complete: 1 scanned, 1 alerted.')).toBeVisible();

  await page.getByRole('button', { name: 'Request changes for agent Browser Review Agent' }).click();
  await expect(page.getByRole('heading', { name: 'Request Changes: Browser Review Agent' })).toBeVisible();
  await page.getByRole('button', { name: 'Request Changes' }).click();
  await expect(page.getByText('A change request reason is required.')).toBeVisible();

  await page.getByLabel('Change Request Reason').fill('Add screenshots and clarify pricing.');
  await page.getByRole('button', { name: 'Request Changes' }).click();

  await expect(page.getByText('No agents waiting for review')).toBeVisible();
  await page.getByLabel('Review status filter').selectOption('all');
  await expect(page.getByText('Browser Review Agent')).toBeVisible();
  await expect(page.getByText('Governance status: needs_changes')).toBeVisible();
});

test('admin reviews approves and rejects publisher submissions with moderation evidence', async ({ page }) => {
  await page.goto('/admin/reviews');

  await page.getByRole('button', { name: 'Approve agent Browser Review Agent' }).click();
  await page.getByRole('button', { name: 'Approve Agent' }).click();

  await expect(page.getByText('No agents waiting for review')).toBeVisible();
  await page.getByLabel('Review status filter').selectOption('all');
  await expect(page.getByText('Governance status: approved')).toBeVisible();

  await page.getByRole('button', { name: 'Reject agent Browser Review Agent' }).click();
  await page.getByRole('button', { name: 'Reject Agent' }).click();
  await expect(page.getByText('A rejection reason is required.')).toBeVisible();

  await page.getByLabel('Rejection Reason').fill('Missing security evidence.');
  await page.getByRole('button', { name: 'Reject Agent' }).click();

  await expect(page.getByText('Governance status: rejected')).toBeVisible();
});

test('admin reviews resolves marketplace abuse reports with moderation evidence', async ({ page }) => {
  await page.goto('/admin/reviews');

  await expect(page.getByRole('heading', { name: 'Marketplace Abuse Reports' })).toBeVisible();
  await expect(page.getByText('The listing contains prompt-injection instructions.')).toBeVisible();

  await page.getByRole('button', { name: 'Resolve abuse report report_review_browser' }).click();
  await expect(page.getByRole('heading', { name: 'Resolve Abuse Report: report_review_browser' })).toBeVisible();
  await page.getByRole('button', { name: 'Resolve Report' }).click();
  await expect(page.getByText('Resolution is required.')).toBeVisible();

  await page.getByLabel('Resolution').fill('publisher fixed listing');
  await page.getByRole('button', { name: 'Resolve Report' }).click();

  await expect(page.getByText('Abuse report resolved.')).toBeVisible();
  await page.getByLabel('Abuse report status filter').selectOption('all');
  await expect(page.getByText('agent_review_browser')).toBeVisible();
  await expect(page.getByLabel('Resolved', { exact: true })).toBeVisible();
});

test('admin reviews dismisses marketplace abuse reports with moderation evidence', async ({ page }) => {
  await page.goto('/admin/reviews');

  await page.getByRole('button', { name: 'Dismiss abuse report report_review_browser' }).click();
  await page.getByRole('button', { name: 'Dismiss Report' }).click();
  await expect(page.getByText('Resolution is required.')).toBeVisible();

  await page.getByLabel('Resolution').fill('not reproducible after review');
  await page.getByRole('button', { name: 'Dismiss Report' }).click();

  await expect(page.getByText('Abuse report dismissed.')).toBeVisible();
  await page.getByLabel('Abuse report status filter').selectOption('all');
  await expect(page.getByLabel('Dismissed', { exact: true })).toBeVisible();
});

test('admin reviews applies takedown and reinstatement governance actions', async ({ page }) => {
  await page.goto('/admin/reviews');

  await page.getByRole('button', { name: 'Apply Governance' }).click();
  await expect(page.getByText('Agent ID is required.')).toBeVisible();

  await page.getByLabel('Agent ID').fill('agent_review_browser');
  await page.getByRole('button', { name: 'Apply Governance' }).click();
  await expect(page.getByText('Governance reason is required.')).toBeVisible();

  await page.getByLabel('Reason').fill('policy violation from abuse evidence');
  await page.getByRole('button', { name: 'Apply Governance' }).click();
  await expect(page.getByText('Marketplace agent taken down.')).toBeVisible();

  await page.getByLabel('Governance action').selectOption('reinstate');
  await page.getByLabel('Reason').fill('appeal accepted with remediation evidence');
  await page.getByRole('button', { name: 'Apply Governance' }).click();
  await expect(page.getByText('Marketplace agent reinstated.')).toBeVisible();
});
