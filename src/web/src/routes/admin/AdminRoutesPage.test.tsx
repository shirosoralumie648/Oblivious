import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listRoutes = vi.fn();
const listChannels = vi.fn();
const deleteRoute = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listRoutes,
    listChannels,
    createRoute: vi.fn(),
    updateRoute: vi.fn(),
    deleteRoute,
  }),
}));

import { AdminRoutesPage } from './AdminRoutesPage';

describe('AdminRoutesPage', () => {
  beforeEach(() => {
    listRoutes.mockReset();
    listChannels.mockReset();
    deleteRoute.mockReset();
  });

  it('renders route table with target channel and opens route drawer', async () => {
    listRoutes.mockResolvedValue([
      {
        id: 'rt_1',
        model: 'gpt-4*',
        strategy: 'single',
        channels: [{ channelID: 'ch_1', channelName: 'OpenAI Primary', weight: 100, priority: 1, enabled: true }],
        createdAt: '2026-01-01T00:00:00Z',
      },
    ]);
    listChannels.mockResolvedValue({ data: [], total: 0 });

    render(<AdminRoutesPage />);

    expect(await screen.findByRole('heading', { name: 'Model Routes' })).toBeInTheDocument();
    expect(await screen.findByText('gpt-4*')).toBeInTheDocument();
    expect(await screen.findByText('OpenAI Primary')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Add Route' }));
    expect(await screen.findByRole('heading', { name: 'Add Route' })).toBeInTheDocument();
  });

  it('confirms route deletion', async () => {
    listRoutes.mockResolvedValue([
      {
        id: 'rt_1',
        model: 'gpt-4*',
        strategy: 'single',
        channels: [{ channelID: 'ch_1', channelName: 'OpenAI Primary', weight: 100, priority: 1, enabled: true }],
        createdAt: '2026-01-01T00:00:00Z',
      },
    ]);
    listChannels.mockResolvedValue({ data: [], total: 0 });
    deleteRoute.mockResolvedValue(undefined);

    render(<AdminRoutesPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Delete route gpt-4*' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Delete Route' }));

    await waitFor(() => expect(deleteRoute).toHaveBeenCalledWith('rt_1'));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });
});
