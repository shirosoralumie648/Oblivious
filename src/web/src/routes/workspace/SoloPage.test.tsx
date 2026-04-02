import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const navigate = vi.fn();
const createConversation = vi.fn();
const listConversations = vi.fn();

const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'solo' as const,
      modelStrategy: 'quality',
      networkEnabledHint: true,
      onboardingCompleted: true
    },
    status: 'authenticated' as const,
    user: { email: 'user@example.com', id: 'u1' }
  }
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');

  return {
    ...actual,
    useNavigate: () => navigate
  };
});

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

vi.mock('../../features/chat/api', () => ({
  createChatApi: () => ({
    createConversation,
    listConversations
  })
}));

import { SoloPage } from './SoloPage';

describe('SoloPage', () => {
  beforeEach(() => {
    appContext.authState.preferences = {
      defaultMode: 'solo',
      modelStrategy: 'quality',
      networkEnabledHint: true,
      onboardingCompleted: true
    };
    createConversation.mockReset();
    listConversations.mockReset();
    navigate.mockReset();
  });

  afterEach(() => {
    createConversation.mockReset();
    listConversations.mockReset();
    navigate.mockReset();
  });

  it('loads and renders the solo launch context', async () => {
    listConversations.mockResolvedValue([
      { id: 'conv_2', title: 'Research task' },
      { id: 'conv_1', title: 'Daily planning' }
    ]);

    render(<SoloPage />);

    expect(screen.getByText('Loading solo workspace…')).toBeInTheDocument();
    expect(await screen.findByText('Model strategy: quality')).toBeInTheDocument();
    expect(screen.getByText('Web suggestions: Enabled')).toBeInTheDocument();
    expect(screen.getByText('Research task')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Continue latest thread' })).toBeInTheDocument();
  });

  it('creates a dedicated solo thread and routes into chat', async () => {
    listConversations.mockResolvedValue([]);
    createConversation.mockResolvedValue({ id: 'conv_new', title: 'SOLO run' });

    render(<SoloPage />);

    await screen.findByRole('button', { name: 'Start solo run' });
    fireEvent.click(screen.getByRole('button', { name: 'Start solo run' }));

    await waitFor(() => {
      expect(createConversation).toHaveBeenCalledWith({ title: 'SOLO run' });
    });
    expect(navigate).toHaveBeenCalledWith('/chat/conv_new');
  });
});
