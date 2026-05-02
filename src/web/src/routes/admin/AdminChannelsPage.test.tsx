import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listChannels = vi.fn();
const testChannel = vi.fn();
const batchUpdateChannels = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listChannels,
    testChannel,
    batchUpdateChannels,
    getChannelHealth: vi.fn(),
    createChannel: vi.fn(),
    updateChannel: vi.fn(),
    deleteChannel: vi.fn(),
  }),
}));

import { AdminChannelsPage } from './AdminChannelsPage';

describe('AdminChannelsPage', () => {
  beforeEach(() => {
    listChannels.mockReset();
    testChannel.mockReset();
    batchUpdateChannels.mockReset();
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
    testChannel.mockResolvedValue({ success: true, latency: 118 });
    batchUpdateChannels.mockResolvedValue(undefined);

    render(<AdminChannelsPage />);

    expect(await screen.findByRole('heading', { name: 'Channels' })).toBeInTheDocument();
    expect(await screen.findByText('OpenAI Primary')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Test connection for OpenAI Primary' }));
    await waitFor(() => expect(testChannel).toHaveBeenCalledWith('ch_1'));

    fireEvent.click(screen.getByRole('checkbox', { name: 'Select row OpenAI Primary' }));
    fireEvent.click(screen.getByRole('button', { name: 'Batch Disable' }));
    await waitFor(() => expect(batchUpdateChannels).toHaveBeenCalledWith(['ch_1'], 'disable'));
    await waitFor(() => expect(listChannels).toHaveBeenCalledTimes(2));
  });

  it('opens the add channel drawer', async () => {
    listChannels.mockResolvedValue({ data: [], total: 0 });

    render(<AdminChannelsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Add Channel' }));

    expect(await screen.findByRole('heading', { name: 'Add Channel' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Name')).toBeInTheDocument();
  });
});
