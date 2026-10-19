import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listChannels = vi.fn();
const listChannelProviders = vi.fn();
const listChannelStats = vi.fn();
const testChannel = vi.fn();
const syncChannelModels = vi.fn();
const refreshChannelBalance = vi.fn();
const detectChannelModelUpdates = vi.fn();
const applyChannelModelUpdates = vi.fn();
const batchUpdateChannels = vi.fn();
const createChannel = vi.fn();
const updateChannel = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listChannels,
    listChannelProviders,
    listChannelStats,
    testChannel,
    syncChannelModels,
    refreshChannelBalance,
    detectChannelModelUpdates,
    applyChannelModelUpdates,
    batchUpdateChannels,
    getChannelHealth: vi.fn(),
    createChannel,
    updateChannel,
    deleteChannel: vi.fn(),
  }),
}));

import { AdminChannelsPage } from './AdminChannelsPage';

describe('AdminChannelsPage', () => {
  beforeEach(() => {
    listChannels.mockReset();
    listChannelProviders.mockReset();
    listChannelStats.mockReset();
    testChannel.mockReset();
    syncChannelModels.mockReset();
    refreshChannelBalance.mockReset();
    detectChannelModelUpdates.mockReset();
    applyChannelModelUpdates.mockReset();
    batchUpdateChannels.mockReset();
    createChannel.mockReset();
    updateChannel.mockReset();
    listChannelProviders.mockResolvedValue([
      { id: 'openai', displayName: 'OpenAI', kind: 'openai_compatible', status: 'supported', defaultBaseURL: 'https://api.openai.com' },
      { id: 'claude', displayName: 'Claude', kind: 'native', status: 'supported', defaultBaseURL: 'https://api.anthropic.com' },
      { id: 'gemini', displayName: 'Gemini', kind: 'native', status: 'supported', defaultBaseURL: 'https://generativelanguage.googleapis.com' },
      { id: 'deepseek', displayName: 'DeepSeek', kind: 'openai_compatible', status: 'supported', defaultBaseURL: 'https://api.deepseek.com' },
      { id: 'openrouter', displayName: 'OpenRouter', kind: 'openai_compatible', status: 'supported', defaultBaseURL: 'https://openrouter.ai/api/v1' },
      { id: 'ollama', displayName: 'Ollama', kind: 'openai_compatible', status: 'supported', defaultBaseURL: 'http://localhost:11434/v1' },
      { id: 'vertex', displayName: 'Vertex AI', kind: 'native', status: 'supported', defaultBaseURL: '' },
      { id: 'bedrock', displayName: 'Amazon Bedrock', kind: 'native', status: 'supported', defaultBaseURL: '' },
      { id: 'azure-openai', displayName: 'Azure OpenAI', kind: 'openai_compatible', status: 'planned', defaultBaseURL: '' },
    ]);
    listChannelStats.mockResolvedValue([]);
  });

  it('renders channels table and supports test connection plus batch disable', async () => {
    listChannels.mockResolvedValue({
      data: [
        {
          id: 'ch_1',
          name: 'OpenAI Primary',
          provider: 'openai',
          baseURL: 'https://api.openai.com/v1',
          models: ['gpt-4o'],
          rpm: 100,
          tpm: 1000,
          priority: 1,
          enabled: true,
          status: 'online',
          latency: 120,
          createdAt: '2026-01-01T00:00:00Z',
          updatedAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
    });
    testChannel.mockResolvedValue({
      success: true,
      latency: 118,
      provider: 'openai',
      models: ['gpt-4o-mini', 'text-embedding-3-small'],
      balance: {
        amount: 12.5,
        currency: 'USD',
        source: 'openai_credit_grants',
      },
      health: {
        status: 'online',
        checkedAt: '2026-01-01T00:00:00Z',
      },
    });
    batchUpdateChannels.mockResolvedValue(undefined);

    render(<AdminChannelsPage />);

    expect(await screen.findByRole('heading', { name: 'Channels' })).toBeInTheDocument();
    expect(await screen.findByText('OpenAI Primary')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Test connection for OpenAI Primary' }));
    await waitFor(() => expect(testChannel).toHaveBeenCalledWith('ch_1'));
    expect(await screen.findByText('OpenAI Primary diagnostics')).toBeInTheDocument();
    expect(screen.getByText('gpt-4o-mini')).toBeInTheDocument();
    expect(screen.getByText('USD 12.50')).toBeInTheDocument();
    expect(screen.getByText('Health online')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('checkbox', { name: 'Select row OpenAI Primary' }));
    fireEvent.click(screen.getByRole('button', { name: 'Batch Disable' }));
    await waitFor(() => expect(batchUpdateChannels).toHaveBeenCalledWith(['ch_1'], 'disable'));
    await waitFor(() => expect(listChannels).toHaveBeenCalledTimes(2));
  });

  it('syncs upstream models back to the channel', async () => {
    listChannels.mockResolvedValue({
      data: [
        {
          id: 'ch_1',
          name: 'OpenAI Primary',
          provider: 'openai',
          baseURL: 'https://api.openai.com/v1',
          models: ['gpt-4o'],
          rpm: 100,
          tpm: 1000,
          priority: 1,
          enabled: true,
          status: 'online',
          latency: 120,
          createdAt: '2026-01-01T00:00:00Z',
          updatedAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
    });
    syncChannelModels.mockResolvedValue({
      channel: { id: 'ch_1', models: ['gpt-4o-mini', 'text-embedding-3-small'] },
      testResult: {
        success: true,
        latency: 118,
        provider: 'openai',
        models: ['gpt-4o-mini', 'text-embedding-3-small'],
      },
    });

    render(<AdminChannelsPage />);

    expect(await screen.findByText('OpenAI Primary')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Sync models for OpenAI Primary' }));

    await waitFor(() => expect(syncChannelModels).toHaveBeenCalledWith('ch_1'));
    expect(await screen.findByText('OpenAI Primary diagnostics')).toBeInTheDocument();
    expect(screen.getByText('text-embedding-3-small')).toBeInTheDocument();
    await waitFor(() => expect(listChannels).toHaveBeenCalledTimes(2));
  });

  it('refreshes and displays upstream channel balance', async () => {
    listChannels.mockResolvedValue({
      data: [
        {
          id: 'ch_1',
          name: 'OpenAI Primary',
          provider: 'openai',
          baseURL: 'https://api.openai.com/v1',
          models: ['gpt-4o'],
          rpm: 100,
          tpm: 1000,
          priority: 1,
          enabled: true,
          status: 'online',
          latency: 120,
          createdAt: '2026-01-01T00:00:00Z',
          updatedAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
    });
    refreshChannelBalance.mockResolvedValue({
      id: 'ch_1',
      balance: {
        amount: 18.25,
        currency: 'USD',
        source: 'provider_balance',
      },
      testResult: {
        success: true,
        latency: 91,
        provider: 'openai',
        balance: {
          amount: 18.25,
          currency: 'USD',
          source: 'provider_balance',
        },
        health: {
          status: 'online',
          checkedAt: '2026-01-01T00:00:00Z',
        },
      },
    });

    render(<AdminChannelsPage />);

    expect(await screen.findByText('OpenAI Primary')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Refresh balance for OpenAI Primary' }));

    await waitFor(() => expect(refreshChannelBalance).toHaveBeenCalledWith('ch_1'));
    expect(await screen.findByText('OpenAI Primary diagnostics')).toBeInTheDocument();
    expect(screen.getByText('USD 18.25')).toBeInTheDocument();
    await waitFor(() => expect(listChannels).toHaveBeenCalledTimes(2));
  });

  it('displays backend runtime stats for each channel', async () => {
    listChannels.mockResolvedValue({
      data: [
        {
          id: 'ch_1',
          name: 'OpenAI Primary',
          provider: 'openai',
          baseURL: 'https://api.openai.com/v1',
          models: ['gpt-4o'],
          rpm: 100,
          tpm: 1000,
          priority: 1,
          enabled: true,
          status: 'online',
          latency: 120,
          createdAt: '2026-01-01T00:00:00Z',
          updatedAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
    });
    listChannelStats.mockResolvedValue([
      {
        channelID: 'ch_1',
        rpmCurrent: 7,
        tpmCurrent: 321,
        totalRequests: 12,
        successCount: 10,
        failureCount: 2,
        avgLatencyMs: 300,
        rateLimitedUntil: '2026-06-04T12:30:00Z',
        affinityConversationCount: 4,
      },
    ]);
    const rateLimitedLabel = `Limited until ${new Date('2026-06-04T12:30:00Z').toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      hour12: false,
    })}`;

    render(<AdminChannelsPage />);

    const row = await screen.findByRole('row', { name: /OpenAI Primary/i });
    await waitFor(() => expect(listChannelStats).toHaveBeenCalled());
    expect(within(row).getByText('7 RPM')).toBeInTheDocument();
    expect(within(row).getByText('321 TPM')).toBeInTheDocument();
    expect(within(row).getByText('300ms avg')).toBeInTheDocument();
    expect(within(row).getByText(rateLimitedLabel)).toBeInTheDocument();

    expect(screen.getByText('Runtime diagnostics')).toBeInTheDocument();
    expect(screen.getByText('12 requests')).toBeInTheDocument();
    expect(screen.getByText('10 ok / 2 failed')).toBeInTheDocument();
    expect(screen.getByText('Sticky conversations')).toBeInTheDocument();
    expect(screen.getByText('4 active')).toBeInTheDocument();
  });

  it('detects and applies upstream model updates', async () => {
    listChannels.mockResolvedValue({
      data: [
        {
          id: 'ch_1',
          name: 'OpenAI Primary',
          provider: 'openai',
          baseURL: 'https://api.openai.com/v1',
          models: ['gpt-4o', 'legacy-model'],
          rpm: 100,
          tpm: 1000,
          priority: 1,
          enabled: true,
          status: 'online',
          latency: 120,
          createdAt: '2026-01-01T00:00:00Z',
          updatedAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
    });
    detectChannelModelUpdates.mockResolvedValue({
      id: 'ch_1',
      currentModels: ['gpt-4o', 'legacy-model'],
      upstreamModels: ['gpt-4o', 'gpt-4.1'],
      added: ['gpt-4.1'],
      removed: ['legacy-model'],
      unchanged: ['gpt-4o'],
      testResult: {
        success: true,
        latency: 86,
        models: ['gpt-4o', 'gpt-4.1'],
      },
    });
    applyChannelModelUpdates.mockResolvedValue({
      mode: 'merge',
      appliedModels: ['gpt-4o', 'legacy-model', 'gpt-4.1'],
    });

    render(<AdminChannelsPage />);

    expect(await screen.findByText('OpenAI Primary')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Detect model updates for OpenAI Primary' }));

    await waitFor(() => expect(detectChannelModelUpdates).toHaveBeenCalledWith('ch_1'));
    expect(await screen.findByText('Model updates for OpenAI Primary')).toBeInTheDocument();
    expect(screen.getByText('Added 1')).toBeInTheDocument();
    expect(screen.getByText('Removed 1')).toBeInTheDocument();
    expect(within(screen.getByLabelText('Model updates for OpenAI Primary')).getByText('gpt-4.1')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Apply model updates for OpenAI Primary' }));
    await waitFor(() => expect(applyChannelModelUpdates).toHaveBeenCalledWith('ch_1', { mode: 'merge' }));
    await waitFor(() => expect(listChannels).toHaveBeenCalledTimes(2));
  });

  it('opens the add channel drawer', async () => {
    listChannels.mockResolvedValue({ data: [], total: 0 });

    render(<AdminChannelsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Add Channel' }));

    expect(await screen.findByRole('heading', { name: 'Add Channel' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Name')).toBeInTheDocument();
  });

  it('exposes the canonical relay provider catalog in filters and channel forms', async () => {
    listChannels.mockResolvedValue({ data: [], total: 0 });

    render(<AdminChannelsPage />);

    const providerFilter = await screen.findByLabelText('Provider filter');
    await waitFor(() => expect(listChannelProviders).toHaveBeenCalled());
    for (const label of ['OpenAI', 'Claude', 'Gemini', 'DeepSeek', 'OpenRouter', 'Ollama', 'Vertex AI', 'Amazon Bedrock']) {
      expect(providerFilter).toHaveTextContent(label);
    }
    expect(providerFilter).not.toHaveTextContent('Azure OpenAI');

    fireEvent.click(screen.getByRole('button', { name: 'Add Channel' }));
    const providerSelect = await screen.findByLabelText('Provider');
    for (const label of ['OpenAI', 'Claude', 'Gemini', 'DeepSeek', 'OpenRouter', 'Ollama', 'Vertex AI', 'Amazon Bedrock']) {
      expect(providerSelect).toHaveTextContent(label);
    }
    expect(providerSelect).not.toHaveTextContent('Azure OpenAI');
  });

  it('submits channel groups when creating a channel', async () => {
    listChannels.mockResolvedValue({ data: [], total: 0 });
    createChannel.mockResolvedValue({ id: 'ch_new' });

    render(<AdminChannelsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Add Channel' }));
    fireEvent.change(await screen.findByLabelText('Name'), { target: { value: 'VIP OpenAI' } });
    fireEvent.change(screen.getByLabelText('API Key'), { target: { value: 'sk-test' } });
    fireEvent.change(screen.getByLabelText('Base URL'), { target: { value: 'https://api.openai.com/v1' } });
    fireEvent.change(screen.getByLabelText('Models'), { target: { value: 'gpt-4o, gpt-4o-mini' } });
    fireEvent.change(screen.getByLabelText('Groups'), { target: { value: 'vip, enterprise' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Channel' }));

    await waitFor(() => expect(createChannel).toHaveBeenCalledWith(expect.objectContaining({
      groups: ['vip', 'enterprise'],
      models: ['gpt-4o', 'gpt-4o-mini'],
      name: 'VIP OpenAI',
    })));
  });

  it('submits channel cost metadata when creating a channel', async () => {
    listChannels.mockResolvedValue({ data: [], total: 0 });
    createChannel.mockResolvedValue({ id: 'ch_cost' });

    render(<AdminChannelsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Add Channel' }));
    fireEvent.change(await screen.findByLabelText('Name'), { target: { value: 'Costed OpenAI' } });
    fireEvent.change(screen.getByLabelText('API Key'), { target: { value: 'sk-test' } });
    fireEvent.change(screen.getByLabelText('Base URL'), { target: { value: 'https://api.openai.com/v1' } });
    fireEvent.change(screen.getByLabelText('Estimated Cost per 1K'), { target: { value: '1.25' } });
    fireEvent.change(screen.getByLabelText('Cost Multiplier'), { target: { value: '0.5' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Channel' }));

    await waitFor(() => expect(createChannel).toHaveBeenCalledWith(expect.objectContaining({
      costMultiplier: 0.5,
      estimatedCostPer1K: 1.25,
      name: 'Costed OpenAI',
    })));
  });

  it('submits rate limits and routing weight when creating a channel', async () => {
    listChannels.mockResolvedValue({ data: [], total: 0 });
    createChannel.mockResolvedValue({ id: 'ch_limits' });

    render(<AdminChannelsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Add Channel' }));
    fireEvent.change(await screen.findByLabelText('Name'), { target: { value: 'Limited OpenAI' } });
    fireEvent.change(screen.getByLabelText('API Key'), { target: { value: 'sk-test' } });
    fireEvent.change(screen.getByLabelText('Base URL'), { target: { value: 'https://api.openai.com/v1' } });
    fireEvent.change(screen.getByLabelText('RPM Limit'), { target: { value: '120' } });
    fireEvent.change(screen.getByLabelText('TPM Limit'), { target: { value: '240000' } });
    fireEvent.change(screen.getByLabelText('Weight'), { target: { value: '35' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Channel' }));

    await waitFor(() => expect(createChannel).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Limited OpenAI',
      rpmLimit: 120,
      tpmLimit: 240000,
      weight: 35,
    })));
  });

  it('edits and submits channel rate limits without requiring a new API key', async () => {
    listChannels.mockResolvedValue({
      data: [
        {
          id: 'ch_1',
          name: 'OpenAI Primary',
          provider: 'openai',
          baseURL: 'https://api.openai.com/v1',
          models: ['gpt-4o'],
          groups: ['default'],
          rpm: 250,
          tpm: 500000,
          priority: 1,
          enabled: true,
          status: 'online',
          latency: 120,
          createdAt: '2026-01-01T00:00:00Z',
          updatedAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
    });
    updateChannel.mockResolvedValue({ id: 'ch_1' });

    render(<AdminChannelsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Edit channel OpenAI Primary' }));
    expect(await screen.findByLabelText('RPM Limit')).toHaveValue(250);
    expect(screen.getByLabelText('TPM Limit')).toHaveValue(500000);

    fireEvent.change(screen.getByLabelText('RPM Limit'), { target: { value: '300' } });
    fireEvent.change(screen.getByLabelText('TPM Limit'), { target: { value: '750000' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    await waitFor(() => expect(updateChannel).toHaveBeenCalledWith('ch_1', expect.objectContaining({
      rpmLimit: 300,
      tpmLimit: 750000,
    })));
    expect(updateChannel.mock.calls[0][1]).not.toHaveProperty('apiKey');
  });
});
