import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const createChannel = vi.fn();
const listChannelMessages = vi.fn();
const listChannels = vi.fn();
const listFailedChannelMessages = vi.fn();
const retryFailedChannelMessages = vi.fn();
const sendChannelMessage = vi.fn();
const testChannel = vi.fn();
const updateChannelStatus = vi.fn();

vi.mock('../../features/publishingChannels/publishingChannelsApi', () => ({
  createPublishingChannelsApi: () => ({
    createChannel,
    listChannelMessages,
    listChannels,
    listFailedChannelMessages,
    retryFailedChannelMessages,
    sendChannelMessage,
    testChannel,
    updateChannelStatus
  })
}));

import { PublishingChannelsPage } from './PublishingChannelsPage';

const opsChannel = {
  config: { secret: '********', url: 'https://hooks.example/ops' },
  created_at: '2026-06-04T12:00:00Z',
  id: 'channel_1',
  name: 'Ops Webhook',
  organization_id: 'org_1',
  status: 'degraded',
  type: 'webhook',
  updated_at: '2026-06-04T12:30:00Z'
};

describe('PublishingChannelsPage', () => {
  beforeEach(() => {
    createChannel.mockReset();
    listChannelMessages.mockReset();
    listChannels.mockReset();
    listFailedChannelMessages.mockReset();
    retryFailedChannelMessages.mockReset();
    sendChannelMessage.mockReset();
    testChannel.mockReset();
    updateChannelStatus.mockReset();
  });

  it('loads publishing channel configs with recent message logs and failed retry context', async () => {
    listChannels.mockResolvedValue([opsChannel]);
    listChannelMessages.mockResolvedValue([
      {
        created_at: '2026-06-04T12:40:00Z',
        direction: 'inbound',
        failure_reason: '',
        id: 'channel_message_recent',
        raw_message: { text: 'hello from webhook' },
        retry_count: 0,
        status: 'recorded',
        transform_success: true,
        transformed_message: { content: [{ text: 'hello from webhook', type: 'text' }], role: 'user' }
      }
    ]);
    listFailedChannelMessages.mockResolvedValue([
      {
        created_at: '2026-06-04T12:39:00Z',
        direction: 'outbound',
        failure_reason: 'upstream 503',
        id: 'channel_message_failed',
        next_retry_at: 'retry-window-soon',
        raw_message: { text: 'retry me' },
        retry_count: 2,
        status: 'retry_pending',
        transform_error: 'delivery failed',
        transform_success: false,
        transformed_message: { content: [{ text: 'retry me', type: 'text' }], role: 'assistant' }
      }
    ]);

    render(<PublishingChannelsPage />);

    expect(screen.getByText('Loading publishing channels...')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Publishing Channels' })).toBeInTheDocument();
    const list = screen.getByLabelText('Publishing channel list');
    expect(within(list).getByText('Ops Webhook')).toBeInTheDocument();
    expect(within(list).getByText('webhook')).toBeInTheDocument();
    expect(within(list).getByText('Degraded')).toBeInTheDocument();
    expect(within(list).getByText('https://hooks.example/ops')).toBeInTheDocument();
    await waitFor(() => {
      expect(listChannelMessages).toHaveBeenCalledWith('channel_1');
      expect(listFailedChannelMessages).toHaveBeenCalledWith('channel_1');
    });
    expect(screen.getByText('Recent messages')).toBeInTheDocument();
    expect(screen.getByText('channel_message_recent')).toBeInTheDocument();
    expect(screen.getByText('inbound')).toBeInTheDocument();
    expect(screen.getByText('Failed retry queue')).toBeInTheDocument();
    expect(screen.getByText('channel_message_failed')).toBeInTheDocument();
    expect(screen.getByText('retry_pending')).toBeInTheDocument();
    expect(screen.getByText('Retries 2')).toBeInTheDocument();
    expect(screen.getByText('Next retry: retry-window-soon')).toBeInTheDocument();
    expect(screen.getByText('upstream 503')).toBeInTheDocument();
  });

  it('creates a webhook channel and prepends it to the list', async () => {
    listChannels.mockResolvedValue([]);
    listChannelMessages
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([
        {
          created_at: '2026-06-04T12:44:00Z',
          direction: 'outbound',
          id: 'channel_message_fallback_recorded',
          raw_message: { text: 'retried through fallback' },
          retry_count: 3,
          status: 'recorded'
        }
      ]);
    listFailedChannelMessages.mockResolvedValue([]);
    createChannel.mockResolvedValue({
      ...opsChannel,
      config: { secret: 'shared-secret', url: 'https://hooks.example/support' },
      id: 'channel_created',
      name: 'Support Webhook',
      status: 'active'
    });

    render(<PublishingChannelsPage />);

    await screen.findByText('No publishing channels configured.');
    fireEvent.change(screen.getByLabelText('Channel name'), { target: { value: ' Support Webhook ' } });
    fireEvent.change(screen.getByLabelText('Channel type'), { target: { value: 'webhook' } });
    fireEvent.change(screen.getByLabelText('Endpoint URL'), { target: { value: ' https://hooks.example/support ' } });
    fireEvent.change(screen.getByLabelText('Shared secret'), { target: { value: ' shared-secret ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create channel' }));

    await waitFor(() => {
      expect(createChannel).toHaveBeenCalledWith({
        config: { secret: 'shared-secret', url: 'https://hooks.example/support' },
        name: 'Support Webhook',
        type: 'webhook'
      });
    });
    const list = screen.getByLabelText('Publishing channel list');
    expect(within(list).getByRole('heading', { name: 'Support Webhook' })).toBeInTheDocument();
    expect(within(list).getByText('Active')).toBeInTheDocument();
    expect(screen.getByLabelText('Channel name')).toHaveValue('');
  });

  it('offers Slack, Telegram, and Web embed channel contracts from the create form', async () => {
    listChannels.mockResolvedValue([]);
    listChannelMessages.mockResolvedValue([]);
    listFailedChannelMessages.mockResolvedValue([]);
    createChannel
      .mockResolvedValueOnce({
        config: { botToken: 'xoxb-secret', signingSecret: 'slack-signing-secret', url: 'https://hooks.slack.com/services/ops' },
        id: 'channel_slack',
        name: 'Ops Slack',
        status: 'active',
        type: 'slack'
      })
      .mockResolvedValueOnce({
        config: { botToken: 'telegram-token', url: 'https://api.telegram.org/bottelegram-token' },
        id: 'channel_telegram',
        name: 'Ops Telegram',
        status: 'active',
        type: 'telegram'
      })
      .mockResolvedValueOnce({
        config: { allowedOrigin: 'https://app.example', embedMode: 'iframe', sdkKey: 'sdk_public_123' },
        id: 'channel_web_embed',
        name: 'Website Embed',
        status: 'active',
        type: 'web_embed'
      });

    render(<PublishingChannelsPage />);

    await screen.findByText('No publishing channels configured.');
    const typeSelect = screen.getByLabelText('Channel type');
    expect(within(typeSelect).getByRole('option', { name: 'slack' })).toBeInTheDocument();
    expect(within(typeSelect).getByRole('option', { name: 'telegram' })).toBeInTheDocument();
    expect(within(typeSelect).getByRole('option', { name: 'web_embed' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Channel name'), { target: { value: 'Ops Slack' } });
    fireEvent.change(typeSelect, { target: { value: 'slack' } });
    fireEvent.change(screen.getByLabelText('Endpoint URL'), { target: { value: ' https://hooks.slack.com/services/ops ' } });
    fireEvent.change(screen.getByLabelText('Bot token'), { target: { value: ' xoxb-secret ' } });
    fireEvent.change(screen.getByLabelText('Signing secret'), { target: { value: ' slack-signing-secret ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create channel' }));
    await waitFor(() => {
      expect(createChannel).toHaveBeenNthCalledWith(1, {
        config: { botToken: 'xoxb-secret', signingSecret: 'slack-signing-secret', url: 'https://hooks.slack.com/services/ops' },
        name: 'Ops Slack',
        type: 'slack'
      });
    });

    fireEvent.change(screen.getByLabelText('Channel name'), { target: { value: 'Ops Telegram' } });
    fireEvent.change(typeSelect, { target: { value: 'telegram' } });
    fireEvent.change(screen.getByLabelText('Endpoint URL'), { target: { value: ' https://api.telegram.org/bottelegram-token ' } });
    fireEvent.change(screen.getByLabelText('Bot token'), { target: { value: ' telegram-token ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create channel' }));
    await waitFor(() => {
      expect(createChannel).toHaveBeenNthCalledWith(2, {
        config: { botToken: 'telegram-token', url: 'https://api.telegram.org/bottelegram-token' },
        name: 'Ops Telegram',
        type: 'telegram'
      });
    });

    fireEvent.change(screen.getByLabelText('Channel name'), { target: { value: 'Website Embed' } });
    fireEvent.change(typeSelect, { target: { value: 'web_embed' } });
    fireEvent.change(screen.getByLabelText('Allowed origin'), { target: { value: ' https://app.example ' } });
    fireEvent.change(screen.getByLabelText('SDK key'), { target: { value: ' sdk_public_123 ' } });
    expect(screen.getByText('iframe')).toBeInTheDocument();
    expect(screen.getByText('Web SDK')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Create channel' }));

    await waitFor(() => {
      expect(createChannel).toHaveBeenNthCalledWith(3, {
        config: { allowedOrigin: 'https://app.example', embedMode: 'iframe', sdkKey: 'sdk_public_123' },
        name: 'Website Embed',
        type: 'web_embed'
      });
    });
    const list = screen.getByLabelText('Publishing channel list');
    expect(within(list).getByText('web_embed')).toBeInTheDocument();
    expect(within(list).getByText('https://app.example')).toBeInTheDocument();
  });

  it('tests a channel, manually switches status, and sends a test message', async () => {
    listChannels.mockResolvedValue([opsChannel]);
    listChannelMessages.mockResolvedValue([]);
    listFailedChannelMessages.mockResolvedValue([]);
    testChannel.mockResolvedValue({ channel_id: 'channel_1', message: 'channel adapter is available', status: 'success', type: 'webhook' });
    updateChannelStatus.mockResolvedValue({ ...opsChannel, status: 'active' });
    sendChannelMessage.mockResolvedValue({ id: 'log_1', status: 'recorded', transform_success: true });

    render(<PublishingChannelsPage />);

    const list = await screen.findByLabelText('Publishing channel list');
    await within(list).findByRole('heading', { name: 'Ops Webhook' });
    fireEvent.click(screen.getByRole('button', { name: 'Test channel Ops Webhook' }));
    await waitFor(() => expect(testChannel).toHaveBeenCalledWith('channel_1'));
    expect(screen.getByText('channel adapter is available')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Activate Ops Webhook' }));
    await waitFor(() => expect(updateChannelStatus).toHaveBeenCalledWith('channel_1', 'active'));
    expect(screen.getByText('Active')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Conversation ID'), { target: { value: ' conversation_1 ' } });
    fireEvent.change(screen.getByLabelText('Message text'), { target: { value: ' Delivery recovered ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }));

    await waitFor(() => {
      expect(sendChannelMessage).toHaveBeenCalledWith('channel_1', {
        conversation_id: 'conversation_1',
        role: 'assistant',
        text: 'Delivery recovered'
      });
    });
    expect(screen.getByText('Last send: recorded')).toBeInTheDocument();
  });

  it('loads and displays message visibility for the manually selected channel', async () => {
    const fallbackChannel = {
      ...opsChannel,
      config: { secret: 'fallback-secret', url: 'https://hooks.example/fallback' },
      id: 'channel_2',
      name: 'Fallback Webhook',
      status: 'active'
    };
    listChannels.mockResolvedValue([opsChannel, fallbackChannel]);
    listChannelMessages
      .mockResolvedValueOnce([
        {
          created_at: '2026-06-04T12:40:00Z',
          direction: 'outbound',
          id: 'ops_recent_message',
          raw_message: { text: 'ops route response' },
          retry_count: 0,
          status: 'recorded'
        }
      ])
      .mockResolvedValueOnce([
        {
          created_at: '2026-06-04T12:45:00Z',
          direction: 'inbound',
          id: 'fallback_recent_message',
          raw_message: { text: 'fallback user message' },
          retry_count: 0,
          status: 'recorded'
        }
      ]);
    listFailedChannelMessages
      .mockResolvedValueOnce([
        {
          created_at: '2026-06-04T12:39:00Z',
          direction: 'outbound',
          failure_reason: 'ops upstream 503',
          id: 'ops_failed_message',
          raw_message: { text: 'ops failed delivery' },
          retry_count: 2,
          status: 'retry_pending'
        }
      ])
      .mockResolvedValueOnce([
        {
          created_at: '2026-06-04T12:44:00Z',
          direction: 'outbound',
          failure_reason: 'fallback signature mismatch',
          id: 'fallback_failed_message',
          raw_message: { text: 'fallback failed delivery' },
          retry_count: 1,
          status: 'retry_pending'
        }
      ]);

    render(<PublishingChannelsPage />);

    expect(await screen.findByText('ops_recent_message')).toBeInTheDocument();
    expect(screen.getByText('ops_failed_message')).toBeInTheDocument();
    expect(screen.getByText('ops upstream 503')).toBeInTheDocument();
    await waitFor(() => {
      expect(listChannelMessages).toHaveBeenCalledWith('channel_1');
      expect(listFailedChannelMessages).toHaveBeenCalledWith('channel_1');
    });

    fireEvent.change(screen.getByLabelText('Channel'), { target: { value: 'channel_2' } });

    expect(await screen.findByText('fallback_recent_message')).toBeInTheDocument();
    expect(screen.getByText('fallback_failed_message')).toBeInTheDocument();
    expect(screen.getByText('fallback signature mismatch')).toBeInTheDocument();
    await waitFor(() => {
      expect(listChannelMessages).toHaveBeenCalledWith('channel_2');
      expect(listFailedChannelMessages).toHaveBeenCalledWith('channel_2');
    });
    expect(screen.queryByText('ops_recent_message')).not.toBeInTheDocument();
    expect(screen.queryByText('ops_failed_message')).not.toBeInTheDocument();
  });

  it('retries the failed queue with a fallback channel, limit, refreshed failures, and result summary', async () => {
    const fallbackChannel = {
      ...opsChannel,
      config: { secret: 'fallback-secret', url: 'https://hooks.example/fallback' },
      id: 'channel_2',
      name: 'Fallback Webhook',
      status: 'active'
    };
    listChannels.mockResolvedValue([opsChannel, fallbackChannel]);
    listChannelMessages.mockResolvedValue([]);
    listFailedChannelMessages.mockResolvedValue([
      {
        created_at: '2026-06-04T12:39:00Z',
        direction: 'outbound',
        failure_reason: 'upstream 503',
        id: 'channel_message_failed',
        retry_count: 2,
        status: 'retry_pending'
      }
    ]);
    retryFailedChannelMessages.mockResolvedValue({ claimed: 5, failed: 1, permanentFailures: 1, succeeded: 3 });

    render(<PublishingChannelsPage />);

    await waitFor(() => expect(listFailedChannelMessages).toHaveBeenCalledWith('channel_1'));
    expect(screen.getAllByText('channel_message_failed')).toHaveLength(2);
    listChannelMessages.mockClear();
    listFailedChannelMessages.mockClear();
    listChannelMessages.mockResolvedValueOnce([
      {
        created_at: '2026-06-04T12:44:00Z',
        direction: 'outbound',
        id: 'channel_message_fallback_recorded',
        raw_message: { text: 'retried through fallback' },
        retry_count: 3,
        status: 'recorded'
      }
    ]);
    listFailedChannelMessages.mockResolvedValueOnce([]);
    const failedQueue = screen.getByLabelText('Failed retry queue controls');
    fireEvent.change(within(failedQueue).getByLabelText('Fallback channel'), { target: { value: 'channel_2' } });
    fireEvent.change(within(failedQueue).getByLabelText('Retry limit'), { target: { value: '5' } });
    fireEvent.click(within(failedQueue).getByRole('button', { name: 'Switch queue to fallback' }));

    await waitFor(() => {
      expect(retryFailedChannelMessages).toHaveBeenCalledWith('channel_1', {
        fallback_channel_id: 'channel_2',
        force: true,
        limit: 5
      });
    });
    await waitFor(() => expect(listFailedChannelMessages).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(listChannelMessages).toHaveBeenCalledTimes(1));
    expect(screen.getByText('Retry result: claimed 5, succeeded 3, failed 1, permanent failures 1')).toBeInTheDocument();
    expect(await screen.findByText('channel_message_fallback_recorded')).toBeInTheDocument();
    expect(screen.getByText('No failed retry messages waiting on this channel.')).toBeInTheDocument();
  });
});
