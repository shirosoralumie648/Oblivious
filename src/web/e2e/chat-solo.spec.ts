import { expect, type Page, test } from '@playwright/test';

import { registerChatSoloRealtime, registerChatSoloRoutes } from './fixtures/chatSolo';

let realtime: Awaited<ReturnType<typeof registerChatSoloRealtime>>;

async function expectNoHorizontalOverflow(page: Page) {
  const overflowState = await page.evaluate(() => {
    const workspaceCanvas = document.querySelector('.workspace-canvas');
    const visibleMessageActionGroups = Array.from(document.querySelectorAll('[aria-label^="Actions for message "]')).filter(
      (element) => {
        const rect = element.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0;
      }
    );

    return {
      actionGroupsFit: visibleMessageActionGroups.every((element) => element.scrollWidth <= element.clientWidth + 1),
      documentFits: document.documentElement.scrollWidth <= window.innerWidth + 1,
      workspaceCanvasFits:
        workspaceCanvas instanceof HTMLElement ? workspaceCanvas.scrollWidth <= workspaceCanvas.clientWidth + 1 : true,
    };
  });

  expect(overflowState).toEqual({
    actionGroupsFit: true,
    documentFits: true,
    workspaceCanvasFits: true,
  });
}

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

test('chat message actions and mobile rail stay usable in the browser', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: async (text: string) => {
          (window as typeof window & { __chatSoloCopiedText?: string }).__chatSoloCopiedText = text;
        },
      },
    });
  });

  await page.goto('/chat/conversation_browser_solo');

  await expect(page.getByRole('heading', { name: 'Chat workspace' })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Workspace navigation' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Conversations' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Copy message msg_existing_browser_solo' })).toBeVisible();
  await expectNoHorizontalOverflow(page);

  await page.getByRole('button', { name: 'Conversations' }).click();
  await expect(page.getByLabel('Conversation rail')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Open conversation Browser SOLO launch thread' })).toBeVisible();
  await expectNoHorizontalOverflow(page);
  await page.getByRole('button', { name: 'Close conversations' }).click();
  await expect(page.getByLabel('Conversation rail')).not.toBeVisible();
  await expectNoHorizontalOverflow(page);

  await page.getByRole('button', { name: 'Copy message msg_existing_browser_solo' }).click();
  await expect
    .poll(() =>
      page.evaluate(() => (window as typeof window & { __chatSoloCopiedText?: string }).__chatSoloCopiedText ?? '')
    )
    .toBe('Existing launch context from the browser journey.');

  await page.getByRole('button', { name: 'Edit message msg_existing_browser_solo' }).click();
  await page.getByLabel('Edit message msg_existing_browser_solo content').fill('Browser action edited launch context.');
  await page.getByRole('button', { name: 'Save edit for message msg_existing_browser_solo' }).click();
  await expect(page.getByText('Browser action edited launch context.')).toBeVisible();
  await expectNoHorizontalOverflow(page);

  await page.getByRole('button', { name: 'Bookmark message msg_existing_browser_solo' }).click();
  await expect(page.getByText('Bookmarked')).toBeVisible();

  await page.getByLabel('Share expiration for msg_existing_browser_solo').fill('2026-06-18T12:00:00Z');
  await page.getByRole('button', { name: 'Share message msg_existing_browser_solo' }).click();
  await expect(page.getByText('https://share.example.test/message_action')).toBeVisible();

  await page.getByRole('button', { name: 'Fork conversation from message msg_existing_browser_solo' }).click();
  await expect(page).toHaveURL(/\/chat\/conversation_browser_solo_fork$/);
  await expect(page.getByRole('heading', { name: 'Chat workspace' })).toBeVisible();
  await expect(page.getByText('Forked browser action context.')).toBeVisible();

  await page.getByRole('button', { name: 'Regenerate response for message msg_fork_assistant' }).click();
  await expect(page.getByText('Browser action regenerated launch answer.')).toBeVisible();
  await expectNoHorizontalOverflow(page);

  await page.getByRole('button', { name: 'Delete message msg_fork_assistant' }).click();
  await expect(page.getByText('Browser action regenerated launch answer.')).not.toBeVisible();
  await expectNoHorizontalOverflow(page);
});
