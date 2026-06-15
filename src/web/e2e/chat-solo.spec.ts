import { expect, test } from '@playwright/test';

import { registerChatSoloRealtime, registerChatSoloRoutes } from './fixtures/chatSolo';

let realtime: Awaited<ReturnType<typeof registerChatSoloRealtime>>;

test.beforeEach(async ({ page }) => {
  realtime = await registerChatSoloRealtime(page);
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

test('chat realtime websocket joins, sends typing, and applies live events in the browser', async ({ page }) => {
  await page.goto('/chat/conversation_browser_solo');

  await expect(page.getByRole('heading', { name: 'Chat workspace' })).toBeVisible();
  await expect(page.getByText('Existing launch context from the browser journey.')).toBeVisible();

  await expect
    .poll(() => realtime.sentMessages)
    .toEqual(
      expect.arrayContaining([
        {
          conversationId: 'conversation_browser_solo',
          type: 'chat_join',
        },
      ])
    );

  await page.getByLabel('Message draft').fill('Co-edit this answer.');
  await expect
    .poll(() => realtime.sentMessages)
    .toEqual(
      expect.arrayContaining([
        {
          conversationId: 'conversation_browser_solo',
          isTyping: true,
          type: 'chat_typing',
        },
      ])
    );

  realtime.emit({
    category: 'chat',
    payload: {
      conversationId: 'conversation_browser_solo',
      messages: [
        {
          content: 'Existing launch context from the browser journey.',
          createdAt: '2026-06-14T12:00:00Z',
          id: 'msg_existing_browser_solo',
          role: 'assistant',
        },
        {
          content: 'Browser realtime collaborative note.',
          createdAt: '2026-06-14T12:01:00Z',
          id: 'msg_realtime_browser',
          role: 'user',
        },
      ],
    },
    type: 'chat_messages_synced',
  });
  await expect(page.getByText('Browser realtime collaborative note.')).toBeVisible();

  realtime.emit({
    category: 'chat',
    payload: {
      conversationId: 'conversation_browser_solo',
      isTyping: true,
      userId: 'user_collaborator',
    },
    type: 'chat_typing',
  });
  await expect(page.getByRole('status')).toHaveText('A collaborator is typing...');

  realtime.emit({
    category: 'chat',
    payload: {
      conversationId: 'conversation_browser_solo',
      message: {
        content: 'Browser realtime collaborative note updated.',
        createdAt: '2026-06-14T12:01:00Z',
        id: 'msg_realtime_browser',
        role: 'user',
      },
      messageId: 'msg_realtime_browser',
    },
    type: 'chat_message_updated',
  });
  await expect(page.getByText('Browser realtime collaborative note updated.')).toBeVisible();

  realtime.emit({
    category: 'chat',
    payload: {
      conversationId: 'conversation_browser_solo',
      messageId: 'msg_realtime_browser',
    },
    type: 'chat_message_deleted',
  });
  await expect(page.getByText('Browser realtime collaborative note updated.')).not.toBeVisible();
  await expect.poll(() => realtime.violations).toEqual([]);
});
