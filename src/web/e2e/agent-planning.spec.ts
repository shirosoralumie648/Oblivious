import { expect, type Page, test } from '@playwright/test';

import { registerAgentPlanningRoutes } from './fixtures/agentPlanning';

async function expectAgentLayoutContained(page: Page) {
  const overflowState = await page.evaluate(() => {
    const main = document.querySelector('main');

    return {
      documentFits: document.documentElement.scrollWidth <= window.innerWidth + 1,
      mainFits: main instanceof HTMLElement ? main.scrollWidth <= main.clientWidth + 1 : true,
    };
  });

  expect(overflowState).toEqual({
    documentFits: true,
    mainFits: true,
  });
}

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

test('agent browser journey creates and updates advanced runtime config', async ({ page }) => {
  test.setTimeout(60_000);

  await page.goto('/agents');

  await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Browser Planning Agent' })).toBeVisible();
  await page.getByRole('button', { name: 'Create agent' }).click();

  const createForm = page.getByRole('region', { name: 'Create agent form' });
  await expect(createForm).toBeVisible();
  await createForm.getByLabel('Agent name').fill('Browser Config Agent');
  await expect(createForm.getByLabel('Agent name')).toHaveValue('Browser Config Agent');
  await createForm.getByLabel('Model', { exact: true }).fill('gpt-4o-mini');
  await createForm.getByLabel('Description').fill('Exercises the browser create and update agent config flow.');
  await createForm.getByLabel('System prompt').fill('Prefer explicit browser evidence.');
  await createForm.getByLabel('Approval mode').selectOption('all');
  await createForm.getByLabel('Default execution mode').selectOption('planning');
  await createForm.getByLabel('Long-term memory writes').selectOption('explicit_only');
  await createForm.getByLabel('Long-term memory extraction').selectOption('llm_assisted');
  await createForm.getByLabel('Long-term memory update').selectOption('memory_key_consolidate');
  await createForm.getByLabel('Max iterations').fill('30');
  await createForm.getByLabel('Token budget').fill('60000');
  await createForm.getByLabel('Max skills').fill('1');
  await createForm
    .getByLabel('Model routing rules JSON')
    .fill(JSON.stringify([{ minIteration: 2, requiresToolResult: true, targetModel: 'gpt-4o' }], null, 2));
  await createForm
    .getByLabel('Skills JSON')
    .fill(JSON.stringify([{ instructions: 'Check weather sources.', name: 'Weather', toolNames: ['web_search'], triggers: ['weather'] }], null, 2));
  await expect(createForm.getByRole('button', { name: 'Save agent' })).toBeEnabled();
  await createForm.getByRole('button', { name: 'Save agent' }).click();

  await expect(page.getByRole('region', { name: 'Create agent form' })).toHaveCount(0);
  await expect(page.getByText('Agent created.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Browser Config Agent' })).toHaveAttribute('aria-pressed', 'true');

  const policySection = page.getByRole('region', { name: 'Agent policy Browser Config Agent' });
  await expect(policySection).toBeVisible();
  await expect(policySection.getByLabel('Approval mode')).toHaveValue('all');
  await expect(policySection.getByLabel('Default execution mode')).toHaveValue('planning');
  await expect(policySection.getByLabel('Long-term memory writes')).toHaveValue('explicit_only');
  await expect(policySection.getByLabel('Long-term memory extraction')).toHaveValue('llm_assisted');
  await expect(policySection.getByLabel('Long-term memory update')).toHaveValue('memory_key_consolidate');
  await expect(policySection.getByLabel('Max iterations', { exact: true })).toHaveValue('30');
  await expect(policySection.getByLabel('Token budget', { exact: true })).toHaveValue('60000');
  await expect(policySection.getByLabel('Max skills')).toHaveValue('1');
  await expect(policySection.getByLabel('Model routing rules JSON')).toHaveValue(
    JSON.stringify([{ minIteration: 2, requiresToolResult: true, targetModel: 'gpt-4o' }], null, 2)
  );
  await expect(policySection.getByLabel('Skills JSON')).toHaveValue(
    JSON.stringify([{ instructions: 'Check weather sources.', name: 'Weather', toolNames: ['web_search'], triggers: ['weather'] }], null, 2)
  );

  await policySection.getByLabel('Approval mode').selectOption('custom');
  await policySection.getByLabel('Default execution mode').selectOption('react');
  await policySection.getByLabel('Long-term memory writes').selectOption('manual_only');
  await policySection.getByLabel('Long-term memory extraction').selectOption('deterministic');
  await policySection.getByLabel('Long-term memory update').selectOption('exact_refresh');
  await policySection.getByLabel('Max iterations', { exact: true }).fill('40');
  await policySection.getByLabel('Token budget', { exact: true }).fill('75000');
  await policySection.getByLabel('Max skills').fill('3');
  await policySection.getByLabel('Model routing rules JSON').fill(JSON.stringify([{ minInputChars: 2000, targetModel: 'gpt-4.1' }], null, 2));
  await policySection.getByLabel('Skills JSON').fill(
    JSON.stringify([{ instructions: 'Summarize with citations.', name: 'Summarizer', toolNames: ['web_search'] }], null, 2)
  );
  await policySection.getByRole('button', { name: 'Save agent policy' }).click();

  await expect(page.getByText('Agent policy saved.')).toBeVisible();
  await expect(policySection.getByLabel('Approval mode')).toHaveValue('custom');
  await expect(policySection.getByLabel('Default execution mode')).toHaveValue('react');
  await expect(policySection.getByLabel('Long-term memory writes')).toHaveValue('manual_only');
  await expect(policySection.getByLabel('Long-term memory extraction')).toHaveValue('deterministic');
  await expect(policySection.getByLabel('Long-term memory update')).toHaveValue('exact_refresh');
  await expect(policySection.getByLabel('Max iterations', { exact: true })).toHaveValue('40');
  await expect(policySection.getByLabel('Token budget', { exact: true })).toHaveValue('75000');
  await expect(policySection.getByLabel('Max skills')).toHaveValue('3');
  await expect(policySection.getByLabel('Model routing rules JSON')).toHaveValue(
    JSON.stringify([{ minInputChars: 2000, targetModel: 'gpt-4.1' }], null, 2)
  );
  await expect(policySection.getByLabel('Skills JSON')).toHaveValue(
    JSON.stringify([{ instructions: 'Summarize with citations.', name: 'Summarizer', toolNames: ['web_search'] }], null, 2)
  );
});

test('agent planning keeps mobile policy and run evidence contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/agents');

  const longAgentName = 'AgentRuntimePolicyIncidentResponderWithExtremelyLongUnbrokenIdentifier20260624';
  const longToolName = 'provider_research_cluster_browser_policy_tool_with_unbroken_runtime_identifier_20260624';

  await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Agents' })).toHaveAttribute('aria-current', 'page');
  await page.getByRole('button', { name: longAgentName }).click();

  const policySection = page.getByRole('region', { name: `Agent policy ${longAgentName}` });
  await expect(policySection).toBeVisible();
  await expect(policySection.getByRole('heading', { name: longAgentName })).toBeVisible();
  await expect(policySection.getByText('providerresearchclusterultralongagentmodelidentifier20260624preview')).toBeVisible();
  const toolPolicy = policySection.getByLabel(`Tool policy ${longToolName}`);
  await expect(toolPolicy).toContainText(longToolName);
  await expect(toolPolicy).toContainText('Server: providerresearchclustertoolserverwithoutbreaks20260624');
  await expect(toolPolicy.getByLabel(`Require approval for ${longToolName}`)).toBeChecked();
  await expectAgentLayoutContained(page);

  await page.goto('/agent-runs/run_browser_agent_mobile_long/plan-steps');

  await expect(page.getByRole('heading', { name: 'Agent Plan Steps' })).toBeVisible();
  await expect(page.getByText('Run run_browser_agent_mobile_long')).toBeVisible();
  await expect(page.getByLabel('Agent run execution controls')).toContainText(
    'token_budget_exceeded_providerresearchclustermobileruntimewithoutbreaks20260624'
  );
  const longPlanStep = page.getByRole('article', {
    name: 'Plan step Review provider research cluster browser policy containment',
  });
  await expect(longPlanStep).toContainText('providerresearchclusterbrowserpolicycontainmentwithoutbreaks20260624');
  await expect(longPlanStep).toContainText('provider_research_cluster_mobile_policy_tool_without_breaks_20260624');
  await expectAgentLayoutContained(page);
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
