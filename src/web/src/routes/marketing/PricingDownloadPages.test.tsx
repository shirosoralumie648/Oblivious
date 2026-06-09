import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { DownloadPage } from './DownloadPage';
import { PricingPage } from './PricingPage';

describe('Marketing utility pages', () => {
  it('renders the pricing page offer copy', () => {
    render(<PricingPage />);

    expect(screen.getByRole('heading', { name: 'Pricing' })).toBeInTheDocument();
    expect(screen.getByText('Choose a plan that matches your workload.')).toBeInTheDocument();
  });

  it('renders the download page client selection copy', () => {
    render(<DownloadPage />);

    expect(screen.getByRole('heading', { name: 'Download' })).toBeInTheDocument();
    expect(screen.getByText('Pick the client that fits your device and workflow.')).toBeInTheDocument();
  });
});
