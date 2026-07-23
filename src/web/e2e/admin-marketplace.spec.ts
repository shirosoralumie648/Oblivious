import { expect, test } from '@playwright/test';

import { registerAdminMarketplaceRoutes } from './fixtures/adminMarketplace';

test.beforeEach(async ({ page }) => {
  await registerAdminMarketplaceRoutes(page);
});

test('admin navigation exposes release management pages', async ({ page }) => {
  await page.goto('/admin');
  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
  await expect(page.getByText('API Calls (24h)')).toBeVisible();

  const adminPages = [
    { path: '/admin/channels', heading: 'Channels', content: 'OpenAI Primary' },
    { path: '/admin/routes', heading: 'Model Routes', content: 'gpt-4o-mini' },
    { path: '/admin/plans', heading: 'Plans', content: 'Team Release' },
    { path: '/admin/users', heading: 'Users', content: 'admin@example.com' },
    { path: '/admin/audit-log', heading: 'Audit Log', content: 'agent.approve' },
    { path: '/admin/reviews', heading: 'Review Queue', content: 'Release Helper' },
  ];

  for (const adminPage of adminPages) {
    await page.goto(adminPage.path);
    await expect(page.getByRole('heading', { name: adminPage.heading })).toBeVisible();
    await expect(page.getByText(adminPage.content).first()).toBeVisible();
  }
});

test('marketplace browse detail and install workflow works', async ({ page }) => {
  await page.goto('/marketplace');
  await expect(page.getByRole('heading', { name: 'Agent Marketplace' })).toBeVisible();
  await expect(page.getByText('Release Helper').first()).toBeVisible({ timeout: 15000 });

  await page.getByPlaceholder('Search agents...').fill('release');
  await expect(page.getByText('Release Helper').first()).toBeVisible({ timeout: 15000 });

  await page.goto('/marketplace/agents/agent_release_helper');
  await expect(page.getByRole('heading', { name: 'Release Helper' })).toBeVisible();
  await expect(page.getByText('Guides release owners').first()).toBeVisible();

  await page.getByRole('button', { name: 'Install Agent' }).click();
  await expect(page.getByText('Agent installed.')).toBeVisible();
});

test('marketplace publish and my agents workflow works', async ({ page }) => {
  await page.goto('/marketplace/publish');
  await expect(page.getByRole('heading', { name: 'Publish Agent' })).toBeVisible();

  await page.getByLabel('Name').fill('Release Notes Drafter');
  await page.getByLabel('Category').selectOption('cat_productivity');
  await page.getByLabel('Description').fill('Drafts release notes and validates candidate readiness for operators.');
  await page.getByLabel('Tags').fill('release, notes');
  await page.getByLabel('Tools').fill('{"tools":[{"name":"release_notes"}]}');
  await page.getByLabel('Example Conversations').fill('[{"userMessage":"Draft notes","assistantMessage":"Here is the release summary."}]');
  await page.getByLabel('System Prompt').fill('Help operators draft release notes.');
  await page.getByLabel('Version').fill('1.0.0');
  await page.getByRole('button', { name: 'Publish Agent' }).click();

  await expect(page.getByText('Agent submitted for review.')).toBeVisible();

  await page.goto('/marketplace/my-agents');
  await expect(page.getByRole('heading', { name: 'My Agents' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Published Agents', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Installed Agents', exact: true })).toBeVisible();
  await expect(page.getByText('Release Notes Drafter').first()).toBeVisible({ timeout: 15000 });
  await expect(page.getByText('Release Helper').first()).toBeVisible({ timeout: 15000 });
});
