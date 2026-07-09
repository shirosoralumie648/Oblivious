import { expect, test } from '@playwright/test';

import { registerCommercialJourneyRoutes } from './fixtures/commercialJourney';

test.beforeEach(async ({ page }) => {
  await registerCommercialJourneyRoutes(page);
});

test('commercial journey covers onboarding Chat Knowledge Agent Marketplace Admin and billing', async ({ page }) => {
  test.setTimeout(60_000);

  await page.goto('/onboarding');
  await expect(page.getByRole('heading', { name: 'Onboarding' })).toBeVisible();
  await page.getByRole('button', { name: 'Start with Chat' }).click();
  await page.getByRole('button', { name: 'Continue to workspace' }).click();

  await expect(page).toHaveURL(/\/chat$/);
  await expect(page.getByRole('heading', { name: 'Chat workspace' })).toBeVisible();
  await page.getByRole('button', { name: 'Create first conversation' }).click();
  await expect(page).toHaveURL(/\/chat\/conv_commercial$/);
  await expect(page.getByRole('heading', { name: 'Conversation transcript' })).toBeVisible();
  await page.getByLabel('Message draft').fill('Prove the commercial Relay journey.');
  await page.getByRole('button', { name: 'Send message' }).click();
  await expect(page.getByText('Relay settled this chat with quota, billing, and monitoring metadata attached.')).toBeVisible();
  await expect(page.getByText('Use knowledge base Commercial Runbook')).toBeVisible();

  await page.goto('/knowledge/kb_commercial');
  await expect(page.getByRole('heading', { name: 'Commercial Runbook' })).toBeVisible();
  await page.getByLabel('Retrieval query').fill('deployment rollback restore');
  await page.getByRole('button', { name: 'Search knowledge' }).click();
  await expect(page.getByRole('heading', { name: 'RAG citations' })).toBeVisible();
  await expect(page.getByText('Commercial deployment rollback restore runbook evidence with source citations.')).toBeVisible();
  await expect(page.getByText('Source: Commercial Deployment Runbook')).toBeVisible();
  await expect(page.getByText('embedding_rag')).toBeVisible();

  await page.goto('/solo?taskId=task_commercial_approval');
  await expect(page.getByRole('heading', { name: 'SOLO' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Commercial run readiness' })).toBeVisible();
  await expect(page.getByText('Authorization scope: workspace_tools')).toBeVisible();
  await expect(page.getByText('Budget consumed: 3 / 25')).toBeVisible();
  await expect(page.getByText('Approval boundary: SOLO will not continue until an authorized user approves this run.')).toBeVisible();
  await expect(page.getByText('Blocked tool boundary: http_request')).toBeVisible();
  await page.getByRole('button', { name: 'Approve plan' }).click();
  await expect(page.getByText('Commercial operator approved continuation.')).toBeVisible();

  await page.goto('/solo?taskId=task_commercial_failed');
  await expect(page.getByText('Retry recovery: failed runs can be restarted without losing the current task context.')).toBeVisible();
  await page.getByRole('button', { name: 'Retry run' }).click();
  await expect(page.getByText('Retry resumed with tenant and budget context.')).toBeVisible();

  await page.goto('/marketplace');
  await expect(page.getByRole('heading', { name: 'Agent Marketplace' })).toBeVisible();
  await expect(page.getByText('Commercial Operator').first()).toBeVisible();
  await page.getByPlaceholder('Search agents...').fill('commercial');
  await expect(page.getByText('Commercial Operator').first()).toBeVisible();

  await page.goto('/marketplace/agents/agent_commercial_operator');
  await expect(page.getByRole('heading', { name: 'Commercial Operator' })).toBeVisible();
  await expect(page.getByText('Paid installs create a checkout-backed marketplace order before workspace installation.')).toBeVisible();
  await expect(page.getByText('$50.00')).toBeVisible();
  await page.getByRole('button', { name: 'Install Agent' }).click();
  await expect(page.getByText('Agent installed.')).toBeVisible();

  await page.goto('/marketplace/publish');
  await expect(page.getByRole('heading', { name: 'Publish Agent' })).toBeVisible();
  await page.getByLabel('Name').fill('Commercial Audit Drafter');
  await page.getByLabel('Category').selectOption('cat_operations');
  await page.getByLabel('Description').fill('Drafts final commercial audit evidence for operators.');
  await page.getByLabel('Tags').fill('commercial, audit');
  await page.getByLabel('Tools').fill('{"tools":[{"name":"datetime","type":"builtin"}]}');
  await page.getByLabel('Example Conversations').fill('[{"userMessage":"Audit","assistantMessage":"Mapped to evidence."}]');
  await page.getByLabel('System Prompt').fill('Map commercial readiness to current evidence.');
  await page.getByLabel('Pricing').selectOption('one_time');
  await page.getByLabel('Price').fill('75');
  await page.getByLabel('Version').fill('1.0.0');
  await page.getByRole('button', { name: 'Publish Agent' }).click();
  await expect(page.getByText('Agent submitted for review. Paid installs remain checkout-backed until approval and settlement evidence exist.')).toBeVisible();

  await page.goto('/marketplace/my-agents');
  await expect(page.getByRole('heading', { name: 'My Agents' })).toBeVisible();
  await expect(page.getByText('Commercial Audit Drafter').first()).toBeVisible();
  await expect(page.getByText('Commercial Operator').first()).toBeVisible();

  await page.goto('/admin');
  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
  await expect(page.getByText('Commercial operations')).toBeVisible();
  await expect(page.getByText('API Calls (24h)')).toBeVisible();

  await page.goto('/admin/billing');
  await expect(page.getByRole('heading', { name: 'Billing' })).toBeVisible();
  await expect(page.getByText('Billing Sessions').first()).toBeVisible();
  await expect(page.getByText('bs_commercial_session')).toBeVisible();
  await expect(page.getByText('Marketplace Net')).toBeVisible();
  await page.getByRole('tab', { name: 'Settlements' }).click();
  await expect(page.getByText('settlement_commercial')).toBeVisible();
  await page.getByRole('tab', { name: 'Payouts' }).click();
  await expect(page.getByText('payout_commercial')).toBeVisible();

  await page.goto('/admin/reviews');
  await expect(page.getByRole('heading', { name: 'Review Queue' })).toBeVisible();
  await expect(page.getByText('Commercial Operator')).toBeVisible();
});
