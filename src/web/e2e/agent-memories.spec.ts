import { expect, test, type Page } from '@playwright/test';

import { registerAgentMemoriesRoutes } from './fixtures/agentMemories';

test.beforeEach(async ({ page }) => {
  await registerAgentMemoriesRoutes(page);
});

async function expectNoHorizontalOverflow(page: Page) {
  const overflowState = await page.evaluate(() => {
    const main = document.querySelector('main');
    const memoryCards = Array.from(document.querySelectorAll('article')).map((card) => {
      if (!(card instanceof HTMLElement)) {
        return false;
      }
      return card.scrollWidth <= card.clientWidth + 1;
    });

    return {
      documentFits: document.documentElement.scrollWidth <= window.innerWidth + 1,
      mainFits: main instanceof HTMLElement ? main.scrollWidth <= main.clientWidth + 1 : true,
      memoryCardCount: memoryCards.length,
      memoryCardsFit: memoryCards.every(Boolean),
    };
  });

  expect(overflowState).toEqual({
    documentFits: true,
    mainFits: true,
    memoryCardCount: 1,
    memoryCardsFit: true,
  });
}

test('agent memories browser journey covers search create edit export import and delete', async ({ page }) => {
  await page.goto('/memories');

  await expect(page.getByRole('heading', { name: 'Agent Memories' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Agent Memories' })).toHaveAttribute('aria-current', 'page');

  await page.getByLabel('Optional agent ID').fill('agent_browser_memory');
  await page.getByLabel('Memory type').selectOption('long_term');
  await page.getByLabel('Result limit').fill('5');
  await page.getByLabel('Search query').fill('rollout');
  await page.getByRole('button', { name: 'Search memories' }).click();

  const searchedMemory = page.locator('article').filter({ hasText: 'Prefer concise rollout notes.' });
  await expect(searchedMemory).toBeVisible();
  await expect(searchedMemory.getByText('Long term')).toBeVisible();
  await expect(searchedMemory.getByText('Agent: agent_browser_memory')).toBeVisible();
  await expect(searchedMemory.getByLabel('Importance 4 of 5')).toBeVisible();
  await expect(searchedMemory.getByText('Metadata: source=workflow, topic=release')).toBeVisible();
  await expect(page.getByText('1 memory')).toBeVisible();

  await page.getByRole('button', { name: 'Export memories' }).click();
  await expect(page.getByRole('link', { name: 'Download memory export' })).toHaveAttribute(
    'href',
    /^blob:/
  );

  await page.getByLabel('Memory importance').selectOption('5');
  await page.getByLabel('Memory content').fill('Always include rollback notes.');
  await page.getByRole('button', { name: 'Create memory' }).click();

  const createdMemory = page.locator('article').filter({ hasText: 'Always include rollback notes.' });
  await expect(createdMemory).toBeVisible();
  await expect(createdMemory.getByText('User managed')).toBeVisible();
  await expect(createdMemory.getByText('Agent: agent_browser_memory')).toBeVisible();
  await expect(createdMemory.getByLabel('Importance 5 of 5')).toBeVisible();
  await expect(page.getByLabel('Memory content')).toHaveValue('');
  await expect(page.getByText('2 memories')).toBeVisible();

  await createdMemory.getByRole('button', { name: 'Edit memory' }).click();
  await page.getByLabel('Edit memory content').fill('Always include rollback notes and owner evidence.');
  await page.getByLabel('Edit memory importance').selectOption('4');
  await page.getByRole('button', { name: 'Save memory' }).click();

  const updatedMemory = page.locator('article').filter({ hasText: 'Always include rollback notes and owner evidence.' });
  await expect(updatedMemory).toBeVisible();
  await expect(updatedMemory.getByLabel('Importance 4 of 5')).toBeVisible();

  await page.getByLabel('Import memories JSON').setInputFiles({
    name: 'agent-memories.json',
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify([
      {
        agentId: 'agent_imported',
        content: 'Imported escalation preference.',
        importance: 5,
        metadata: { imported: true },
        type: 'user_managed',
      },
    ])),
  });

  const importedMemory = page.locator('article').filter({ hasText: 'Imported escalation preference.' });
  await expect(importedMemory).toBeVisible();
  await expect(importedMemory.getByText('Agent: agent_imported')).toBeVisible();
  await expect(importedMemory.getByText('Metadata: imported=true')).toBeVisible();
  await expect(page.getByText('3 memories')).toBeVisible();

  await updatedMemory.getByRole('button', { name: 'Delete memory' }).click();
  await expect(updatedMemory).toHaveCount(0);
  await expect(importedMemory).toBeVisible();
  await expect(searchedMemory).toBeVisible();
  await expect(page.getByText('2 memories')).toBeVisible();
});

test('agent memories mobile layout keeps long memory evidence contained', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/memories');

  await expect(page.getByRole('heading', { name: 'Agent Memories' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Agent Memories' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('main')).toHaveCount(1);

  await page.getByLabel('Optional agent ID').fill('agentmemoriesmobilewithoutbreaks20260624');
  await page.getByLabel('Memory type').selectOption('user_managed');
  await page.getByLabel('Result limit').fill('3');
  await page.getByLabel('Search query').fill('memorymobilewithoutbreaks20260624');
  await page.getByRole('button', { name: 'Search memories' }).click();

  const mobileMemory = page.locator('article').filter({
    hasText: 'memorycontentmobilewithoutbreaks20260624memorycontentmobilewithoutbreaks20260624',
  });
  await expect(mobileMemory).toBeVisible();
  await expect(mobileMemory.getByText('User managed')).toBeVisible();
  await expect(mobileMemory.getByText('Agent: agentmemoriesmobilewithoutbreaks20260624')).toBeVisible();
  await expect(mobileMemory.getByLabel('Importance 5 of 5')).toBeVisible();
  await expect(mobileMemory.getByText('Metadata: evidence=metadatamobilewithoutbreaks20260624')).toBeVisible();
  await expect(mobileMemory.getByRole('button', { name: 'Edit memory' })).toBeVisible();
  await expect(mobileMemory.getByRole('button', { name: 'Delete memory' })).toBeVisible();

  await expectNoHorizontalOverflow(page);
});
