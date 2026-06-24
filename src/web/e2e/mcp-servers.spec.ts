import { expect, type Page, test } from '@playwright/test';

import { registerMcpServersRoutes } from './fixtures/mcpServers';

async function expectMcpServersLayoutContained(page: Page) {
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
  await registerMcpServersRoutes(page);
});

test('MCP servers browser journey covers catalog lifecycle diagnostics tools and execution', async ({ page }) => {
  await page.goto('/mcp-servers');

  await expect(page.getByRole('heading', { name: 'MCP Servers & Tools' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'MCP Servers' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('navigation', { name: 'Workspace navigation' })).toBeVisible();

  await expect(page.getByLabel('Local MCP servers')).toContainText('Local Search Tools');
  await expect(page.getByLabel('Local MCP servers')).toContainText('2 tools');
  await expect(page.getByLabel('Local MCP servers')).toContainText('Local Release Diagnostics');

  const researchCard = page.locator('article').filter({ hasText: 'Research tools' });
  await expect(researchCard).toBeVisible();
  await expect(researchCard).toContainText('https://mcp.example/sse');
  await expect(researchCard).toContainText('disconnected');

  await researchCard.getByRole('button', { name: 'Connect', exact: true }).click();
  await expect(researchCard).toContainText('connected');
  await expect(researchCard).toContainText('Last connected: 2026-06-15T14:05:00Z');

  await researchCard.getByRole('button', { name: 'Disconnect', exact: true }).click();
  await expect(researchCard).toContainText('disconnected');

  await researchCard.getByRole('button', { name: 'Diagnose' }).click();
  await expect(researchCard).toContainText('Diagnostic: connected');

  await researchCard.getByRole('button', { name: 'List tools' }).click();
  await expect(researchCard).toContainText('search_docs');
  await expect(researchCard).toContainText('Search indexed workspace documents.');
  await expect(researchCard.getByLabel('Tool name')).toHaveValue('search_docs');

  await researchCard.getByLabel('Tool arguments JSON').fill('{');
  await researchCard.getByRole('button', { name: 'Execute test call' }).click();
  await expect(page.getByRole('alert')).toHaveText('Tool arguments JSON is invalid.');

  await researchCard.getByLabel('Tool arguments JSON').fill('{"query":"fusion"}');
  await researchCard.getByRole('button', { name: 'Execute test call' }).click();
  await expect(researchCard.locator('output')).toContainText('Found fusion design details.');

  await page.getByLabel('Server name').fill('Internal MCP');
  await page.getByLabel('Endpoint URL').fill('https://mcp.internal/sse');
  await page.getByLabel('Auth token').fill('secret-token');
  await page.getByRole('button', { name: 'Add MCP server' }).click();

  const createdCard = page.locator('article').filter({ hasText: 'Internal MCP' });
  await expect(createdCard).toBeVisible();
  await expect(createdCard).toContainText('https://mcp.internal/sse');
  await expect(createdCard).toContainText('Auth token configured');
  await expect(page.getByLabel('Server name')).toHaveValue('');
  await expect(page.getByLabel('Endpoint URL')).toHaveValue('');
  await expect(page.getByLabel('Auth token')).toHaveValue('');
  await expect(page.getByText('secret-token')).toHaveCount(0);

  await createdCard.getByRole('button', { name: 'Delete' }).click();
  await expect(createdCard).toHaveCount(0);
  await expect(researchCard).toBeVisible();
});

test('mcp servers keeps mobile server tools and long evidence contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/mcp-servers');

  const longServerName = 'ProviderResearchClusterMobileServerWithoutBreaks20260624';
  const longServerCard = page.locator('article').filter({ hasText: longServerName });

  await expect(page.getByRole('heading', { name: 'MCP Servers & Tools' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'MCP Servers' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('main')).toHaveCount(1);
  await expect(longServerCard.getByRole('heading', { name: longServerName })).toBeVisible();
  await expect(longServerCard.getByText('https://mcp.example/providerresearchclustermobileserverwithoutbreaks20260624/sse')).toBeVisible();
  await expect(longServerCard.getByRole('button', { name: 'Connect', exact: true })).toBeVisible();
  await expect(longServerCard.getByRole('button', { name: 'Disconnect', exact: true })).toBeVisible();
  await expect(longServerCard.getByRole('button', { name: 'Diagnose' })).toBeVisible();
  await expect(longServerCard.getByRole('button', { name: 'List tools' })).toBeVisible();
  await expect(longServerCard.getByRole('button', { name: `Delete ${longServerName}` })).toBeVisible();

  await longServerCard.getByRole('button', { name: 'List tools' }).click();
  await expect(longServerCard.getByText('provider_research_cluster_mobile_policy_tool_without_breaks_20260624')).toBeVisible();
  await expect(
    longServerCard.getByText(
      'Validates mobile containment for provider research cluster policy evidence without spaces.'
    )
  ).toBeVisible();

  await longServerCard.getByLabel('Tool arguments JSON').fill('{"query":"mobile containment evidence"}');
  await longServerCard.getByRole('button', { name: 'Execute test call' }).click();
  await expect(
    longServerCard.getByText(
      'provider_research_cluster_mobile_policy_tool_without_breaks_20260624_mobile_evidence_providerresearchclustermobilecontainmentwithoutbreaks20260624'
    )
  ).toBeVisible();

  await expectMcpServersLayoutContained(page);
});
