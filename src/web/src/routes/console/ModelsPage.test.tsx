import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const getModels = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getModels
  })
}));

import { ModelsPage } from './ModelsPage';

describe('ModelsPage', () => {
  afterEach(() => {
    getModels.mockReset();
  });

  it('loads and renders model summaries', async () => {
    getModels.mockResolvedValue([
      { id: 'balanced-chat', label: 'balanced-chat', requests: 2 },
      { id: 'quality-chat', label: 'quality-chat', requests: 1 }
    ]);

    render(<ModelsPage />);

    expect(screen.getByText('Loading model summaries…')).toBeInTheDocument();
    expect(await screen.findByText('balanced-chat')).toBeInTheDocument();
    expect(screen.getByText('Requests: 2')).toBeInTheDocument();
    expect(screen.getByText('quality-chat')).toBeInTheDocument();
  });

  it('renders a fallback message when model summaries fail to load', async () => {
    getModels.mockRejectedValue(new Error('network unavailable'));

    render(<ModelsPage />);

    expect(screen.getByText('Loading model summaries…')).toBeInTheDocument();
    expect(await screen.findByText('Unable to load model summaries.')).toBeInTheDocument();
  });
});
