import { expect, test } from '@playwright/test';

import { registerChatSoloRoutes } from './fixtures/chatSolo';

test.beforeEach(async ({ page }) => {
  await registerChatSoloRoutes(page);
});

test('chat browser journey saves settings streams reply and hands off to SOLO', async ({ page }) => {
  await page.goto('/chat/conversation_browser_solo');

  await expect(page.getByRole('heading', { name: 'Chat workspace' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Chat' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('link', { name: 'Open conversation Browser SOLO launch thread' })).toHaveAttribute(
    'aria-current',
    'page'
  );
  await expect(page.getByText('Existing launch context from the browser journey.')).toBeVisible();

  await page.getByLabel('Temperature').fill('0.7');
  await page.getByLabel('Max output tokens').fill('1400');
  await page.getByLabel('System prompt override').fill('Prefer browser SOLO handoff bullets.');
  await page.getByLabel('Enable tools for this conversation').check();
  await page.getByRole('button', { name: 'Save conversation settings' }).click();

  await page.getByLabel('Message draft').fill('Summarize the launch handoff risk.');
  await page.getByRole('button', { name: 'Send message' }).click();
  await expect(page.getByText('Browser streamed launch handoff answer with saved settings.')).toBeVisible();

  await page.getByRole('button', { name: 'Hand off to SOLO' }).click();
  await expect(page.getByRole('heading', { name: 'Convert to SOLO task' })).toBeVisible();
  await expect(page.getByLabel('SOLO task goal')).toHaveValue('Draft a launch checklist from the browser conversation.');
  await expect(page.getByLabel('Authorization scope for SOLO')).toHaveValue('workspace_tools');
  await expect(page.getByLabel('Use knowledge base Browser Research Vault in SOLO')).toBeChecked();
  await expect(page.getByLabel('Use knowledge base Browser Runbooks in SOLO')).not.toBeChecked();

  await page.getByLabel('Authorization scope for SOLO').selectOption('full_access');
  await page.getByLabel('Allowed tools for SOLO').fill('browser, shell');
  await page.getByLabel('Blocked tools for SOLO').fill('email');
  await page.getByLabel('Use knowledge base Browser Runbooks in SOLO').check();
  await page.getByRole('button', { name: 'Start in SOLO' }).click();

  await expect(page).toHaveURL(/\/solo\?taskId=task_browser_solo&returnTo=%2Fchat%2Fconversation_browser_solo$/);
  await expect(page.getByRole('heading', { name: 'SOLO' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Back to chat' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Commercial run readiness' })).toBeVisible();
  await expect(page.getByText('Authorization scope: full_access')).toBeVisible();
  await expect(page.getByText('Knowledge scope: 2 sources selected')).toBeVisible();
  await expect(page.getByText('Allowed tool boundary: browser, shell')).toBeVisible();
  await expect(page.getByText('Blocked tool boundary: email')).toBeVisible();
  await expect(page.getByText('SOLO browser task started with Chat return context.', { exact: true })).toBeVisible();

  await page.getByRole('button', { name: 'Back to chat' }).click();
  await expect(page).toHaveURL(/\/chat\/conversation_browser_solo$/);
  await expect(page.getByRole('heading', { name: 'Chat workspace' })).toBeVisible();
  await expect(page.getByText('Browser streamed launch handoff answer with saved settings.')).toBeVisible();
});
