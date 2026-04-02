import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const getBilling = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getBilling
  })
}));

import { BillingPage } from './BillingPage';

describe('BillingPage', () => {
  afterEach(() => {
    getBilling.mockReset();
  });

  it('loads and renders the billing summary', async () => {
    getBilling.mockResolvedValue({
      period: '30d',
      requests: 5,
      inputTokens: 120,
      outputTokens: 80,
      estimatedCostUsd: 0.0004
    });

    render(<BillingPage />);

    expect(screen.getByText('Loading billing summary…')).toBeInTheDocument();
    expect(await screen.findByText('Requests: 5')).toBeInTheDocument();
    expect(screen.getByText('Input tokens: 120')).toBeInTheDocument();
    expect(screen.getByText('Output tokens: 80')).toBeInTheDocument();
    expect(screen.getByText('Estimated cost: $0.0004')).toBeInTheDocument();
  });
});
