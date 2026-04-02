import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const getUsage = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getUsage
  })
}));

import { UsagePage } from './UsagePage';

describe('UsagePage', () => {
  afterEach(() => {
    getUsage.mockReset();
  });

  it('loads and renders the usage summary', async () => {
    getUsage.mockResolvedValue({
      period: '7d',
      requests: 3
    });

    render(<UsagePage />);

    expect(screen.getByText('Loading usage summary…')).toBeInTheDocument();
    expect(await screen.findByText('Requests: 3')).toBeInTheDocument();
    expect(screen.getByText('Period: 7d')).toBeInTheDocument();
  });
});
