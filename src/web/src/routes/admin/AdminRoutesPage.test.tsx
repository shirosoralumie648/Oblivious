import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listRoutes = vi.fn();
const listChannels = vi.fn();
const createRoute = vi.fn();
const updateRoute = vi.fn();
const deleteRoute = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listRoutes,
    listChannels,
    createRoute,
    updateRoute,
    deleteRoute,
  }),
}));

import { AdminRoutesPage } from './AdminRoutesPage';

describe('AdminRoutesPage', () => {
  beforeEach(() => {
    listRoutes.mockReset();
    listChannels.mockReset();
    createRoute.mockReset();
    updateRoute.mockReset();
    deleteRoute.mockReset();
  });

  it('renders route table with strategy and opens route drawer', async () => {
    listRoutes.mockResolvedValue([
      {
        id: 'rt_1',
        model: 'gpt-4*',
        strategy: 'weighted',
        channels: [{ channelID: 'ch_1', channelName: 'OpenAI Primary', weight: 100, priority: 1, enabled: true }],
        createdAt: '2026-01-01T00:00:00Z',
      },
    ]);
    listChannels.mockResolvedValue({ data: [], total: 0 });

    render(<AdminRoutesPage />);

    expect(await screen.findByRole('heading', { name: 'Model Routes' })).toBeInTheDocument();
    expect(await screen.findByText('gpt-4*')).toBeInTheDocument();
    expect(await screen.findByText('OpenAI Primary')).toBeInTheDocument();
    expect(await screen.findByText('weighted')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Add Route' }));
    expect(await screen.findByRole('heading', { name: 'Add Route' })).toBeInTheDocument();
    expect(screen.getByLabelText('Strategy')).toHaveTextContent('Adaptive');
    expect(screen.getByLabelText('Strategy')).toHaveTextContent('Weighted');
    expect(screen.getByLabelText('Strategy')).toHaveTextContent('Priority');
    expect(screen.getByLabelText('Strategy')).toHaveTextContent('Cost aware');
  });

  it('submits a relay-recognized route strategy when creating a route', async () => {
    listRoutes.mockResolvedValue([]);
    listChannels.mockResolvedValue({
      data: [
        {
          id: 'ch_1',
          name: 'OpenAI Primary',
          provider: 'openai',
          baseURL: 'https://api.openai.com/v1',
          models: ['gpt-4o'],
          groups: ['default'],
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
    createRoute.mockResolvedValue({ id: 'rt_new' });

    render(<AdminRoutesPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Add Route' }));
    fireEvent.change(await screen.findByLabelText('Model Pattern'), { target: { value: 'gpt-4*' } });
    fireEvent.change(screen.getByLabelText('Target Channel 1'), { target: { value: 'ch_1' } });
    fireEvent.change(screen.getByLabelText('Strategy'), { target: { value: 'adaptive' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Route' }));

    await waitFor(() => expect(createRoute).toHaveBeenCalledWith(expect.objectContaining({
      model: 'gpt-4*',
      strategy: 'adaptive',
      channels: [
        expect.objectContaining({
          channelID: 'ch_1',
          priority: 1,
          weight: 100,
          enabled: true,
        }),
      ],
    })));
  });

  it('submits multiple weighted channel targets when creating a route', async () => {
    listRoutes.mockResolvedValue([]);
    listChannels.mockResolvedValue({
      data: [
        {
          id: 'ch_1',
          name: 'OpenAI Primary',
          provider: 'openai',
          baseURL: 'https://api.openai.com/v1',
          models: ['gpt-4o'],
          groups: ['default'],
          rpm: 100,
          tpm: 1000,
          priority: 1,
          enabled: true,
          status: 'online',
          latency: 120,
          createdAt: '2026-01-01T00:00:00Z',
          updatedAt: '2026-01-01T00:00:00Z',
        },
        {
          id: 'ch_2',
          name: 'Claude Fallback',
          provider: 'anthropic',
          baseURL: 'https://api.anthropic.com',
          models: ['claude-3-5-sonnet'],
          groups: ['default'],
          rpm: 60,
          tpm: 800,
          priority: 2,
          enabled: true,
          status: 'online',
          latency: 180,
          createdAt: '2026-01-01T00:00:00Z',
          updatedAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 2,
    });
    createRoute.mockResolvedValue({ id: 'rt_new' });

    render(<AdminRoutesPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Add Route' }));
    fireEvent.change(await screen.findByLabelText('Model Pattern'), { target: { value: 'gpt-4*' } });
    fireEvent.change(screen.getByLabelText('Strategy'), { target: { value: 'weighted' } });
    fireEvent.change(screen.getByLabelText('Target Channel 1'), { target: { value: 'ch_1' } });
    fireEvent.change(screen.getByLabelText('Weight 1'), { target: { value: '70' } });
    fireEvent.change(screen.getByLabelText('Priority 1'), { target: { value: '1' } });

    fireEvent.click(screen.getByRole('button', { name: 'Add channel target' }));
    fireEvent.change(await screen.findByLabelText('Target Channel 2'), { target: { value: 'ch_2' } });
    fireEvent.change(screen.getByLabelText('Weight 2'), { target: { value: '30' } });
    fireEvent.change(screen.getByLabelText('Priority 2'), { target: { value: '2' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Route' }));

    await waitFor(() => expect(createRoute).toHaveBeenCalledWith(expect.objectContaining({
      model: 'gpt-4*',
      strategy: 'weighted',
      channels: [
        expect.objectContaining({ channelID: 'ch_1', weight: 70, priority: 1, enabled: true }),
        expect.objectContaining({ channelID: 'ch_2', weight: 30, priority: 2, enabled: true }),
      ],
    })));
  });

  it('keeps the existing strategy selected when editing a route', async () => {
    listRoutes.mockResolvedValue([
      {
        id: 'rt_1',
        model: 'gpt-4*',
        strategy: 'cost_aware',
        channels: [{ channelID: 'ch_1', channelName: 'OpenAI Primary', weight: 100, priority: 1, enabled: true }],
        createdAt: '2026-01-01T00:00:00Z',
      },
    ]);
    listChannels.mockResolvedValue({
      data: [
        {
          id: 'ch_1',
          name: 'OpenAI Primary',
          provider: 'openai',
          baseURL: 'https://api.openai.com/v1',
          models: ['gpt-4o'],
          groups: ['default'],
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
    updateRoute.mockResolvedValue({ id: 'rt_1' });

    render(<AdminRoutesPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Edit route gpt-4*' }));
    expect(await screen.findByLabelText('Strategy')).toHaveValue('cost_aware');

    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    await waitFor(() => expect(updateRoute).toHaveBeenCalledWith('rt_1', expect.objectContaining({
      strategy: 'cost_aware',
    })));
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
