import { expect, test } from '@playwright/test';

import { registerAgentPlanningRoutes } from './fixtures/agentPlanning';

test.beforeEach(async ({ page }) => {
  await registerAgentPlanningRoutes(page);
});

test('agent planning browser journey covers tool approval plan-step execution and continue plan', async ({ page }) => {
  await page.goto('/agents');

  await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Agents' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('button', { name: 'Browser Planning Agent' })).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByText('Exercises planning-mode approval and plan-step controls through the browser router.')).toBeVisible();
  await expect(page.getByLabel('Run mode')).toHaveValue('planning');
  await expect(page.getByLabel('Run max iterations')).toHaveValue('8');
  await expect(page.getByLabel('Run token budget')).toHaveValue('30000');
  await expect(page.getByLabel('Require approval for web_search')).toBeChecked();

  await page.getByRole('button', { name: 'Load tool catalog' }).click();
  await expect(page.getByRole('button', { name: 'Tool web_search enabled' })).toBeVisible();

  await page.getByLabel('Run conversation ID').fill('conv_browser_agent');
  await page.getByLabel('Run goal').fill('Plan a browser route proof for Agent planning.');
  await page.getByRole('button', { name: 'Start run' }).click();

  await expect(page.getByRole('link', { name: 'Open run plan steps' })).toHaveAttribute(
    'href',
    '/agent-runs/run_browser_agent/plan-steps'
  );
  await page.getByRole('link', { name: 'Open run plan steps' }).click();

  await expect(page).toHaveURL(/\/agent-runs\/run_browser_agent\/plan-steps$/);
  await expect(page.getByRole('heading', { name: 'Agent Plan Steps' })).toBeVisible();
  await expect(page.getByText('Run run_browser_agent')).toBeVisible();
  await expect(page.getByText('Status: pending_approval')).toBeVisible();
  await expect(page.getByLabel('Agent run execution controls')).toContainText('Mode planning');
  await expect(page.getByLabel('Agent run execution controls')).toContainText('Iterations 2');
  await expect(page.getByLabel('Agent run execution controls')).toContainText('Tool calls 1');
  await expect(page.getByRole('heading', { name: 'Tool Approval Queue' })).toBeVisible();
  const searchToolRun = page.getByRole('article', { name: 'Tool run web_search' });
  await expect(searchToolRun).toContainText('custom');
  await expect(searchToolRun).toContainText('Server: custom-api-browser-search');
  await expect(searchToolRun).toContainText('Risk: medium');
  const scopeStep = page.getByRole('article', { name: 'Plan step Inspect browser route scope' });
  const patchStep = page.getByRole('article', { name: 'Plan step Patch browser route proof' });
  await expect(scopeStep).toContainText('Inspect browser route scope');
  await expect(scopeStep).toContainText('completed');
  await expect(patchStep).toContainText('Patch browser route proof');
  await expect(patchStep).toContainText('Patch the browser route proof after the release scope is known.');
  await expect(patchStep).toContainText('Depends on: 1');

  await page.getByLabel('Operator decision reason for web_search').fill('Browser route operator approval.');
  await page.getByRole('button', { name: 'Approve tool web_search' }).click();
  await expect(searchToolRun).toContainText('Search approved.');

  await page.getByRole('button', { name: 'Approve Patch browser route proof' }).click();
  await expect(page.getByRole('button', { name: 'Execute Patch browser route proof' })).toBeEnabled();
  await page.getByRole('button', { name: 'Execute Patch browser route proof' }).click();
  await expect(patchStep).toContainText('Browser route proof patched.');

  await page.getByRole('button', { name: 'Continue plan' }).click();
  await expect(page.getByText('Status: completed')).toBeVisible();
  await expect(page.getByLabel('Agent run execution controls')).toContainText('Iterations 5');
});

test('agent planning browser journey adjusts remaining plan with operator reason', async ({ page }) => {
  await page.goto('/agents');

  await page.getByLabel('Run conversation ID').fill('conv_browser_agent');
  await page.getByLabel('Run goal').fill('Adjust the remaining browser plan after scope changes.');
  await page.getByRole('button', { name: 'Start run' }).click();
  await page.getByRole('link', { name: 'Open run plan steps' }).click();

  await expect(page).toHaveURL(/\/agent-runs\/run_browser_agent\/plan-steps$/);
  await expect(page.getByText('Status: pending_approval')).toBeVisible();

  const completedStep = page.getByRole('article', { name: 'Plan step Inspect browser route scope' });
  const originalRemainingStep = page.getByRole('article', { name: 'Plan step Patch browser route proof' });
  await expect(completedStep).toContainText('completed');
  await expect(originalRemainingStep).toContainText('Depends on: 1');

  await page.getByLabel('Adjustment reason').fill('Browser scope changed after operator review.');
  await page.getByRole('button', { name: 'Adjust remaining plan' }).click();

  const adjustedStep = page.getByRole('article', { name: 'Plan step Run adjusted browser checks' });
  await expect(page.getByText('Status: pending_approval')).toBeVisible();
  await expect(completedStep).toContainText('Workspace Agent route scope inspected.');
  await expect(adjustedStep).toContainText('Run the adjusted browser checks after the route scope changed.');
  await expect(adjustedStep).toContainText('Depends on: 1');
  await expect(originalRemainingStep).not.toBeVisible();
  await expect(page.getByLabel('Adjustment reason')).toHaveValue('');

  await page.getByRole('button', { name: 'Continue plan' }).click();
  await expect(page.getByText('Status: completed')).toBeVisible();
  await expect(page.getByLabel('Agent run execution controls')).toContainText('Iterations 4');
  await expect(adjustedStep).toContainText('Adjusted browser checks completed.');
  await expect(page.getByLabel('Adjustment reason')).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Adjust remaining plan' })).toBeDisabled();
});

test('agent planning browser journey recovers from token budget stop', async ({ page }) => {
  await page.goto('/agent-runs/run_browser_agent_budget/plan-steps');

  await expect(page).toHaveURL(/\/agent-runs\/run_browser_agent_budget\/plan-steps$/);
  await expect(page.getByRole('heading', { name: 'Agent Plan Steps' })).toBeVisible();
  await expect(page.getByText('Run run_browser_agent_budget')).toBeVisible();
  await expect(page.getByText('Status: token_budget_exceeded')).toBeVisible();
  await expect(page.getByLabel('Agent run execution controls')).toContainText('Mode planning');
  await expect(page.getByLabel('Agent run execution controls')).toContainText('Iterations 2');
  await expect(page.getByLabel('Agent run execution controls')).toContainText('Tool calls 1');
  await expect(page.getByLabel('Agent run execution controls')).toContainText(
    'Stop reason token_budget_exceeded: used 32500 tokens exceeds budget 30000'
  );

  const retryStep = page.getByRole('article', { name: 'Plan step Retry after increased budget' });
  await expect(retryStep).toContainText('failed');
  await expect(retryStep).toContainText('Depends on: 1');
  await expect(page.getByLabel('Increased token budget')).toHaveValue('2500');

  await page.getByLabel('Increased token budget').fill('45000');
  await page.getByRole('button', { name: 'Continue with budget' }).click();

  await expect(page.getByText('Status: completed')).toBeVisible();
  await expect(page.getByLabel('Agent run execution controls')).toContainText('Iterations 3');
  await expect(page.getByLabel('Agent run execution controls')).not.toContainText('Stop reason');
  await expect(retryStep).toContainText('completed');
  await expect(retryStep).toContainText('Token-budget recovery completed in the browser.');
  await expect(page.getByRole('button', { name: 'Continue with budget' })).toHaveCount(0);
});
