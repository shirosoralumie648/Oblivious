import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createChatApi } from './api';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    request: vi.fn(),
    ...overrides
  };
  return client;
}

describe('createChatApi', () => {
  it('normalizes backend conversation share payload fields', async () => {
    const post = vi.fn().mockResolvedValue({
      shareId: 'share_conversation_1',
      shareUrl: '/api/v1/app/conversation-shares/share_conversation_1'
    });
    const api = createChatApi(createClient({ post }));

    await expect(api.createConversationShare('conversation_1')).resolves.toEqual({
      id: 'share_conversation_1',
      shareId: 'share_conversation_1',
      shareUrl: '/api/v1/app/conversation-shares/share_conversation_1',
      url: '/api/v1/app/conversation-shares/share_conversation_1'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/share');
  });

  it('sends a conversation share range and expiration when provided', async () => {
    const post = vi.fn().mockResolvedValue({
      shareId: 'share_conversation_1',
      shareUrl: '/api/v1/app/conversation-shares/share_conversation_1'
    });
    const api = createChatApi(createClient({ post }));

    await api.createConversationShare('conversation_1', {
      endMessageId: 'message_3',
      expiresAt: '2026-06-05T12:00:00Z',
      startMessageId: 'message_1'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/share', {
      endMessageId: 'message_3',
      expiresAt: '2026-06-05T12:00:00Z',
      startMessageId: 'message_1'
    });
  });

  it('normalizes backend message share payload fields', async () => {
    const post = vi.fn().mockResolvedValue({
      shareId: 'share_message_1',
      shareUrl: '/api/v1/app/message-shares/share_message_1'
    });
    const api = createChatApi(createClient({ post }));

    await expect(api.createMessageShare('conversation_1', 'message_1')).resolves.toEqual({
      id: 'share_message_1',
      shareId: 'share_message_1',
      shareUrl: '/api/v1/app/message-shares/share_message_1',
      url: '/api/v1/app/message-shares/share_message_1'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/messages/message_1/share');
  });

  it('normalizes forked conversation parent identifiers from backend payloads', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'conversation_2',
      parent_id: 'conversation_1',
      title: 'Forked launch review'
    });
    const api = createChatApi(createClient({ post }));

    await expect(api.forkConversation('conversation_1', { messageId: 'message_3', title: 'Forked launch review' })).resolves.toEqual({
      id: 'conversation_2',
      parentId: 'conversation_1',
      parent_id: 'conversation_1',
      title: 'Forked launch review'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/fork', {
      messageId: 'message_3',
      title: 'Forked launch review'
    });
  });

  it('reads, updates, and deletes app-scoped conversations by id', async () => {
    const get = vi.fn().mockResolvedValue({ id: 'conversation_1', title: 'Launch review' });
    const put = vi.fn().mockResolvedValue({ id: 'conversation_1', title: 'Renamed launch review' });
    const deleteRequest = vi.fn().mockResolvedValue(undefined);
    const api = createChatApi(createClient({ delete: deleteRequest, get, put }));

    await expect(api.getConversation('conversation_1')).resolves.toEqual({
      id: 'conversation_1',
      title: 'Launch review'
    });
    await expect(api.updateConversation('conversation_1', { title: 'Renamed launch review' })).resolves.toEqual({
      id: 'conversation_1',
      title: 'Renamed launch review'
    });
    await expect(api.deleteConversation('conversation_1')).resolves.toBeUndefined();

    expect(get).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1');
    expect(put).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1', { title: 'Renamed launch review' });
    expect(deleteRequest).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1');
  });

  it('sends a message share expiration when provided', async () => {
    const post = vi.fn().mockResolvedValue({
      shareId: 'share_message_1',
      shareUrl: '/api/v1/app/message-shares/share_message_1'
    });
    const api = createChatApi(createClient({ post }));

    await api.createMessageShare('conversation_1', 'message_1', {
      expiresAt: '2026-06-05T12:00:00Z'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/messages/message_1/share', {
      expiresAt: '2026-06-05T12:00:00Z'
    });
  });

  it('downloads a conversation markdown export as text', async () => {
    const get = vi.fn().mockResolvedValue('# Launch Review\n');
    const api = createChatApi(createClient({ get }));

    await expect(api.exportConversationMarkdown('conversation_1')).resolves.toBe('# Launch Review\n');

    expect(get).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/export.md');
  });

  it('lists chat personas from the app personas endpoint', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        id: 'persona_1',
        name: 'Launch reviewer',
        role: 'Launch reviewer',
        style: 'Direct',
        tone: 'Precise',
        constraints: 'Call out rollout risk.'
      }
    ]);
    const api = createChatApi(createClient({ get }));

    await expect(api.listPersonas()).resolves.toEqual([
      {
        id: 'persona_1',
        name: 'Launch reviewer',
        role: 'Launch reviewer',
        style: 'Direct',
        tone: 'Precise',
        constraints: 'Call out rollout risk.'
      }
    ]);

    expect(get).toHaveBeenCalledWith('/api/v1/app/personas');
  });

  it('creates, updates, and deletes chat personas through app persona endpoints', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'persona_1',
      name: 'Launch reviewer',
      suggestedQuestions: ['What is risky?']
    });
    const put = vi.fn().mockResolvedValue({
      id: 'persona_1',
      name: 'Launch reviewer updated',
      suggestedQuestions: ['What changed?']
    });
    const deleteRequest = vi.fn().mockResolvedValue({ status: 'deleted' });
    const api = createChatApi(createClient({ delete: deleteRequest, post, put }));

    await expect(
      api.createPersona({
        constraints: 'Call out rollout risk.',
        name: 'Launch reviewer',
        openingMessage: 'Ready to review.',
        role: 'Reviewer',
        style: 'Direct',
        suggestedQuestions: ['What is risky?'],
        tone: 'Precise'
      })
    ).resolves.toMatchObject({
      id: 'persona_1',
      suggestedQuestions: ['What is risky?']
    });
    await expect(
      api.updatePersona('persona_1', {
        constraints: 'Focus on release blockers.',
        name: 'Launch reviewer updated',
        suggestedQuestions: ['What changed?']
      })
    ).resolves.toMatchObject({
      id: 'persona_1',
      name: 'Launch reviewer updated',
      suggestedQuestions: ['What changed?']
    });
    await expect(api.deletePersona('persona_1')).resolves.toBeUndefined();

    expect(post).toHaveBeenCalledWith('/api/v1/app/personas', {
      constraints: 'Call out rollout risk.',
      name: 'Launch reviewer',
      openingMessage: 'Ready to review.',
      role: 'Reviewer',
      style: 'Direct',
      suggestedQuestions: ['What is risky?'],
      tone: 'Precise'
    });
    expect(put).toHaveBeenCalledWith('/api/v1/app/personas/persona_1', {
      constraints: 'Focus on release blockers.',
      name: 'Launch reviewer updated',
      suggestedQuestions: ['What changed?']
    });
    expect(deleteRequest).toHaveBeenCalledWith('/api/v1/app/personas/persona_1');
  });

  it('normalizes legacy newline persona questions into arrays', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        id: 'persona_legacy',
        name: 'Legacy reviewer',
        suggestedQuestions: 'What changed?\n\nWhat is risky?'
      }
    ]);
    const api = createChatApi(createClient({ get }));

    await expect(api.listPersonas()).resolves.toEqual([
      {
        id: 'persona_legacy',
        name: 'Legacy reviewer',
        suggestedQuestions: ['What changed?', 'What is risky?']
      }
    ]);
  });

  it('streams a chat message through the message stream endpoint', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(
        new ReadableStream<Uint8Array>({
          start(controller) {
            const encoder = new TextEncoder();
            controller.enqueue(encoder.encode('data: Hello\n\n'));
            controller.enqueue(encoder.encode('data:  world\n\n'));
            controller.enqueue(encoder.encode('data: [DONE]\n\n'));
            controller.close();
          }
        }),
        { status: 200, headers: { 'Content-Type': 'text/event-stream' } }
      )
    );
    const api = createChatApi(createClient(), { fetchFn: fetchFn as unknown as typeof fetch });
    const chunks: string[] = [];

    await api.sendMessageStream('conversation_1', { content: 'hello' }, { onChunk: (chunk) => chunks.push(chunk) });

    expect(fetchFn).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/messages/stream', {
      body: JSON.stringify({ content: 'hello' }),
      headers: {
        Accept: 'text/event-stream',
        'Content-Type': 'application/json'
      },
      method: 'POST',
      signal: undefined
    });
    expect(chunks).toEqual(['Hello', ' world']);
  });

  it('serializes stream message attachments in the JSON request body', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(
        new ReadableStream<Uint8Array>({
          start(controller) {
            const encoder = new TextEncoder();
            controller.enqueue(encoder.encode('data: [DONE]\n\n'));
            controller.close();
          }
        }),
        { status: 200, headers: { 'Content-Type': 'text/event-stream' } }
      )
    );
    const api = createChatApi(createClient(), { fetchFn: fetchFn as unknown as typeof fetch });

    await api.sendMessageStream(
      'conversation_1',
      {
        attachments: [
          {
            contentType: 'image/png',
            id: 'attachment_1',
            name: 'diagram.png',
            sizeBytes: 2048,
            type: 'image'
          }
        ],
        content: 'Review this diagram.'
      },
      { onChunk: vi.fn() }
    );

    expect(fetchFn).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/messages/stream', {
      body: JSON.stringify({
        attachments: [
          {
            contentType: 'image/png',
            id: 'attachment_1',
            name: 'diagram.png',
            sizeBytes: 2048,
            type: 'image'
          }
        ],
        content: 'Review this diagram.'
      }),
      headers: {
        Accept: 'text/event-stream',
        'Content-Type': 'application/json'
      },
      method: 'POST',
      signal: undefined
    });
  });
});
