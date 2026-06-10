import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createPublishingChannelsApi, type PublishingChannel } from './publishingChannelsApi';

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

const channel: PublishingChannel = {
  config: { secret: '********', url: 'https://hooks.example/ops' },
  id: 'channel_1',
  name: 'Ops Webhook',
  organization_id: 'org_1',
  status: 'degraded',
  type: 'webhook'
};

describe('createPublishingChannelsApi', () => {
  it('lists publishing channels from the workspace channel endpoint', async () => {
    const get = vi.fn().mockResolvedValue([channel]);
    const api = createPublishingChannelsApi(createClient({ get }));

    await expect(api.listChannels()).resolves.toEqual([expect.objectContaining({ id: 'channel_1' })]);

    expect(get).toHaveBeenCalledWith('/api/v1/channels');
  });

  it('creates publishing channel configs', async () => {
    const post = vi.fn().mockResolvedValue({ ...channel, id: 'channel_created', status: 'active' });
    const api = createPublishingChannelsApi(createClient({ post }));
    const payload = {
      config: { secret: 'shared-secret', url: 'https://hooks.example/support' },
      name: 'Support Webhook',
      type: 'webhook' as const
    };

    await expect(api.createChannel(payload)).resolves.toEqual(expect.objectContaining({ id: 'channel_created' }));

    expect(post).toHaveBeenCalledWith('/api/v1/channels', payload);
  });

  it('updates and deletes publishing channel configs through item routes', async () => {
    const put = vi.fn().mockResolvedValue({ ...channel, name: 'Ops Webhook Renamed', status: 'active' });
    const deleteRequest = vi.fn().mockResolvedValue({ ...channel, status: 'disabled' });
    const api = createPublishingChannelsApi(createClient({ delete: deleteRequest, put }));
    const payload = {
      config: { secret: '********', url: 'https://hooks.example/ops-renamed' },
      name: 'Ops Webhook Renamed',
      status: 'active' as const,
      type: 'webhook' as const
    };

    await expect(api.updateChannel('channel_1', payload)).resolves.toEqual(expect.objectContaining({ name: 'Ops Webhook Renamed' }));
    await expect(api.deleteChannel('channel_1')).resolves.toEqual(expect.objectContaining({ status: 'disabled' }));

    expect(put).toHaveBeenCalledWith('/api/v1/channels/channel_1', payload);
    expect(deleteRequest).toHaveBeenCalledWith('/api/v1/channels/channel_1');
  });

  it('updates status through the status endpoint without resending redacted config', async () => {
    const put = vi.fn().mockResolvedValue({ ...channel, status: 'active' });
    const request = vi.fn().mockResolvedValue({ ...channel, status: 'active' });
    const api = createPublishingChannelsApi(createClient({ put, request }));

    await expect(api.updateChannelStatus('channel_1', 'active')).resolves.toEqual(expect.objectContaining({ status: 'active' }));

    expect(request).toHaveBeenCalledWith('/api/v1/channels/channel_1/status', {
      body: JSON.stringify({ status: 'active' }),
      method: 'PATCH'
    });
    expect(put).not.toHaveBeenCalled();
  });

  it('tests and sends channel messages through channel action routes', async () => {
    const post = vi
      .fn()
      .mockResolvedValueOnce({ channel_id: 'channel_1', message: 'channel adapter is available', status: 'success', type: 'webhook' })
      .mockResolvedValueOnce({ id: 'log_1', status: 'recorded', transform_success: true });
    const api = createPublishingChannelsApi(createClient({ post }));
    const message = {
      conversation_id: 'conversation_1',
      role: 'assistant' as const,
      text: 'Delivery recovered'
    };

    await expect(api.testChannel('channel_1')).resolves.toEqual(expect.objectContaining({ status: 'success' }));
    await expect(api.sendChannelMessage('channel_1', message)).resolves.toEqual(expect.objectContaining({ id: 'log_1' }));

    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/channels/channel_1/test');
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/channels/channel_1/send', { message });
  });

  it('lists recent message logs and failed retry messages for a channel', async () => {
    const get = vi
      .fn()
      .mockResolvedValueOnce([{ id: 'channel_message_recent', direction: 'inbound', status: 'recorded' }])
      .mockResolvedValueOnce([{ id: 'channel_message_failed', direction: 'outbound', status: 'retry_pending', retry_count: 2 }]);
    const api = createPublishingChannelsApi(createClient({ get }));

    await expect(api.listChannelMessages('channel_1')).resolves.toEqual([expect.objectContaining({ id: 'channel_message_recent' })]);
    await expect(api.listFailedChannelMessages('channel_1')).resolves.toEqual([expect.objectContaining({ retry_count: 2 })]);

    expect(get).toHaveBeenNthCalledWith(1, '/api/v1/channels/channel_1/messages');
    expect(get).toHaveBeenNthCalledWith(2, '/api/v1/channels/channel_1/failed-messages');
  });

  it('retries failed channel messages with fallback and limit controls', async () => {
    const post = vi.fn().mockResolvedValue({ claimed: 10, failed: 1, permanent_failures: 2, succeeded: 7 });
    const api = createPublishingChannelsApi(createClient({ post }));

    await expect(
      api.retryFailedChannelMessages('channel_1', {
        fallback_channel_id: 'channel_2',
        force: true,
        limit: 10
      })
    ).resolves.toEqual(expect.objectContaining({ claimed: 10, permanentFailures: 2, succeeded: 7 }));

    expect(post).toHaveBeenCalledWith('/api/v1/channels/channel_1/retry-failed-messages', {
      fallback_channel_id: 'channel_2',
      force: true,
      limit: 10
    });
  });
});
