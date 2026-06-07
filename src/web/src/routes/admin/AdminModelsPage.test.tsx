import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listModelInventory = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listModelInventory,
  }),
}));

import { AdminModelsPage } from './AdminModelsPage';

describe('AdminModelsPage', () => {
  beforeEach(() => {
    listModelInventory.mockReset();
  });

  it('renders model inventory with providers, channels, costs, and usage', async () => {
    listModelInventory.mockResolvedValue({
      data: [
        {
          model: 'gpt-4o',
          providers: ['openai', 'azure'],
          groups: ['default', 'vip'],
          channelCount: 2,
          enabledChannelCount: 1,
          disabledChannelCount: 1,
          minEstimatedCostPer1K: 0.02,
          maxEstimatedCostPer1K: 0.05,
          avgCostMultiplier: 1.2,
          requestCount: 30,
          totalCost: 1.23,
          totalChannelCost: 0.61,
          channels: [
            {
              id: 'ch_1',
              name: 'OpenAI primary',
              provider: 'openai',
              groups: ['default', 'vip'],
              enabled: true,
              priority: 10,
              estimatedCostPer1K: 0.02,
              costMultiplier: 1.1,
            },
          ],
        },
      ],
      total: 1,
    });

    render(<AdminModelsPage />);

    expect(await screen.findByRole('heading', { name: 'Models' })).toBeInTheDocument();
    expect(await screen.findByText('gpt-4o')).toBeInTheDocument();
    expect(screen.getByText('openai')).toBeInTheDocument();
    expect(screen.getByText('azure')).toBeInTheDocument();
    expect(screen.getByText('default')).toBeInTheDocument();
    expect(screen.getByText('vip')).toBeInTheDocument();
    expect(screen.getByText('1 / 2 enabled')).toBeInTheDocument();
    expect(screen.getByText('$0.0200 - $0.0500')).toBeInTheDocument();
    expect(screen.getByText('1.20x')).toBeInTheDocument();
    expect(screen.getByText('30')).toBeInTheDocument();
    expect(screen.getByText('$1.2300')).toBeInTheDocument();
    expect(screen.getByText('$0.6100')).toBeInTheDocument();
    expect(screen.getByText('$0.6200')).toBeInTheDocument();
    expect(screen.getByText('OpenAI primary')).toBeInTheDocument();
  });

  it('passes filters to listModelInventory', async () => {
    listModelInventory.mockResolvedValue({ data: [], total: 0 });

    render(<AdminModelsPage />);

    fireEvent.change(await screen.findByLabelText('Provider filter'), { target: { value: 'openai' } });
    fireEvent.change(screen.getByLabelText('Group filter'), { target: { value: 'vip' } });
    fireEvent.change(screen.getByLabelText('Status filter'), { target: { value: 'enabled' } });
    fireEvent.change(screen.getByLabelText('Search models'), { target: { value: 'gpt' } });

    await waitFor(() =>
      expect(listModelInventory).toHaveBeenLastCalledWith(
        expect.objectContaining({
          provider: 'openai',
          group: 'vip',
          status: 'enabled',
          search: 'gpt',
          limit: 50,
        })
      )
    );
  });

  it('passes model ranking sort to listModelInventory', async () => {
    listModelInventory.mockResolvedValue({
      data: [
        {
          model: 'gpt-4o',
          providers: ['openai'],
          groups: ['default'],
          channelCount: 1,
          enabledChannelCount: 1,
          disabledChannelCount: 0,
          minEstimatedCostPer1K: 0.02,
          maxEstimatedCostPer1K: 0.02,
          avgCostMultiplier: 1,
          requestCount: 30,
          totalCost: 1.23,
          totalChannelCost: 0.61,
          channels: [],
        },
      ],
      total: 1,
    });

    render(<AdminModelsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Sort by Requests' }));

    await waitFor(() =>
      expect(listModelInventory).toHaveBeenLastCalledWith(
        expect.objectContaining({
          sort: 'requestCount:desc',
          limit: 50,
        })
      )
    );
  });
});
