import { expect, test, type Page } from '@playwright/test';

import { registerAdminMarketplaceRoutes } from './fixtures/adminMarketplace';

test.beforeEach(async ({ page }) => {
  await registerAdminMarketplaceRoutes(page);
});

async function expectNoHorizontalOverflow(page: Page) {
  const overflowState = await page.evaluate(() => {
    const main = document.querySelector('main');
    const tableViewports = Array.from(document.querySelectorAll('[data-slot="table-container"]')).map((viewport) => {
      if (!(viewport instanceof HTMLElement)) {
        return false;
      }
      const style = window.getComputedStyle(viewport);
      return viewport.getBoundingClientRect().width <= window.innerWidth + 1 && style.overflowX === 'auto';
    });

    return {
      documentFits: document.documentElement.scrollWidth <= window.innerWidth + 1,
      mainFits: main instanceof HTMLElement ? main.scrollWidth <= main.clientWidth + 1 : true,
      tableViewportCount: tableViewports.length,
      tableViewportsFit: tableViewports.every(Boolean),
    };
  });

  expect(overflowState).toEqual({
    documentFits: true,
    mainFits: true,
    tableViewportCount: 3,
    tableViewportsFit: true,
  });
}

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
  await expect(page.getByText('Release Helper').first()).toBeVisible();
  const releaseRecommendation = page.getByRole('note', { name: 'Recommendation for Release Helper' }).first();
  await expect(releaseRecommendation.getByText('Recommended')).toBeVisible();
  await expect(releaseRecommendation.getByText('92% match')).toBeVisible();
  await expect(releaseRecommendation.getByText('Matches "release"; Productivity category; release and ops tags; 4.8 rating')).toBeVisible();

  await page.getByPlaceholder('Search agents...').fill('release');
  await expect(page.getByText('Release Helper').first()).toBeVisible();
  await expect(releaseRecommendation.getByText('92% match')).toBeVisible();
  await expect(releaseRecommendation.getByText('Matches "release"; Productivity category; release and ops tags; 4.8 rating')).toBeVisible();

  await page.goto('/marketplace/agents/agent_release_helper');
  await expect(page.getByRole('heading', { name: 'Release Helper' })).toBeVisible();
  await expect(page.getByText('Guides release owners').first()).toBeVisible();

  await page.getByRole('button', { name: 'Install Agent' }).click();
  await expect(page.getByText('Agent installed.')).toBeVisible();
});

test('marketplace installs templates from the canonical browse route', async ({ page }) => {
  await page.goto('/marketplace');

  await expect(page.getByRole('heading', { name: 'Agent Marketplace' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Templates' })).toBeVisible();
  const templateInstallButton = page.getByRole('button', { name: 'Use Launch Browser Template' });
  await expect(templateInstallButton).toBeVisible();

  await templateInstallButton.click();

  await expect(page.getByText('Template ready to use.')).toBeVisible();
});

test('marketplace paid install sends selected provider and exposes checkout continuation', async ({ page }) => {
  await page.goto('/marketplace/agents/agent_paid_release_helper');

  await expect(page.getByRole('link', { name: 'Marketplace' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Paid Release Operator' })).toBeVisible();
  await expect(page.getByText('Paid installs create a checkout-backed marketplace order before workspace installation.')).toBeVisible();
  await expect(page.getByText('$75.00')).toBeVisible();
  await expect(page.getByLabel('Agent version')).toHaveValue('version_paid_release_1');
  await expect(page.getByLabel('Payment provider')).toHaveValue('stripe');
  await expect(page.getByLabel('Payment provider')).toContainText('Alipay');
  await expect(page.getByLabel('Payment provider')).toContainText('WeChat Pay');

  await page.getByLabel('Payment provider').selectOption('wechatpay');
  await page.getByRole('button', { name: 'Install Agent' }).click();

  await expect(page.getByText('Checkout session ready.')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Continue WeChat Pay checkout' })).toHaveAttribute(
    'href',
    'https://checkout.wechatpay.test/session/cs_paid_release_wechatpay_browser'
  );
  await expect(page.getByText('Agent installed.')).toHaveCount(0);
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
  await expect(page.getByRole('heading', { name: 'Installed Agents' })).toBeVisible();
  await expect(page.getByText('Release Notes Drafter').first()).toBeVisible();
  await expect(page.getByText('Release Helper').first()).toBeVisible();
});

test('marketplace my agents preserves publisher mutation contracts in the browser', async ({ page }) => {
  await page.goto('/marketplace/my-agents');

  await expect(page.getByRole('heading', { name: 'My Agents' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Marketplace' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Template Factory' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Settlement Cycle' })).toBeVisible();
  await expect(page.getByText('Current cycle: Monthly')).toBeVisible();
  await expect(page.getByText('Release Notes Drafter').first()).toBeVisible();
  await expect(page.getByText('Release Helper').first()).toBeVisible();

  await page.getByLabel('Template name').fill('Launch Browser Template');
  await page.getByLabel('Template type').selectOption('workflow');
  await page.getByLabel('Template category').fill('Operations');
  await page.getByLabel('Template tags').fill('launch, ops');
  await page.getByLabel('Template description').fill('Reusable browser launch workflow.');
  await page.getByLabel('Template JSON').fill(JSON.stringify({ nodes: [{ id: 'start', type: 'trigger' }], edges: [] }, null, 2));
  await page.getByRole('button', { name: 'Create Template' }).click();
  await expect(page.getByText('Template created: Launch Browser Template.')).toBeVisible();

  await page.getByLabel('Settlement cycle').selectOption('weekly');
  await page.getByRole('button', { name: 'Save Settlement Cycle' }).click();
  await expect(page.getByText('Current cycle: Weekly')).toBeVisible();
  await expect(page.getByText('2% processing fee')).toBeVisible();
  await expect(page.getByText('$50 minimum payout')).toBeVisible();

  await page.getByRole('button', { name: 'Delete agent Release Notes Drafter' }).click();
  const deleteDialog = page.getByRole('dialog', { name: 'Delete Agent' });
  await expect(deleteDialog).toBeVisible();
  await deleteDialog.getByRole('button', { name: 'Delete Agent' }).click();
  await expect(page.getByText('No published agents -- Publish your first agent to start the review process.')).toBeVisible();

  await page.getByRole('button', { name: 'Uninstall Release Helper' }).click();
  await expect(page.getByText('No installed agents -- Install agents from the marketplace to use them in your workspace.')).toBeVisible();
});

test('marketplace my agents mobile layout keeps publisher and settlement evidence contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/marketplace/my-agents');

  await expect(page.getByRole('link', { name: 'Marketplace' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'My Agents' })).toBeVisible();
  await expect(page.getByRole('main')).toHaveCount(1);
  await expect(page.getByRole('heading', { name: 'Template Factory' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Settlement Cycle' })).toBeVisible();

  await expect(page.getByText('publishermobileagentwithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('revenue-tier-mobile-without-breaks-20260624')).toBeVisible();
  await expect(page.getByText('installmobileagentwithoutbreaks20260624')).toBeVisible();
  await expect(page.getByText('publisherstatsmobileagentwithoutbreaks20260624')).toBeVisible();

  await expectNoHorizontalOverflow(page);
});
