import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import type { SessionResponse, UserPreferences } from '../types/api';
import { AppContextProvider, useAppContext } from './appContext';

function TestConsumer() {
  const { authState, updatePreferences } = useAppContext();

  return (
    <div>
      <p>Status: {authState.status}</p>
      <p>User: {authState.user?.email ?? 'anonymous'}</p>
      <p>Mode: {authState.preferences?.defaultMode ?? 'none'}</p>
      <p>Strategy: {authState.preferences?.modelStrategy ?? 'none'}</p>
      <button
        onClick={() => {
          void updatePreferences({
            defaultMode: 'solo',
            modelStrategy: 'quality',
            networkEnabledHint: true,
            onboardingCompleted: true
          });
        }}
        type="button"
      >
        Update preferences
      </button>
    </div>
  );
}

describe('AppContextProvider', () => {
  it('provides authState and updatePreferences via app context', async () => {
    const session: SessionResponse = {
      onboardingCompleted: false,
      preferences: {
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: false,
        onboardingCompleted: false
      },
      session: {
        expiresAt: '2026-04-06T00:00:00Z',
        id: 'session_1'
      },
      user: {
        email: 'user@example.com',
        id: 'u1'
      },
      workspace: {
        id: 'workspace_1'
      }
    };
    const updatePreferencesRequest = vi.fn(async (preferences: UserPreferences) => preferences);

    render(
      <AppContextProvider authApi={{ me: vi.fn(async () => session) }} updatePreferencesRequest={updatePreferencesRequest}>
        <TestConsumer />
      </AppContextProvider>
    );

    expect(screen.getByText('Status: loading')).toBeInTheDocument();
    expect(await screen.findByText('User: user@example.com')).toBeInTheDocument();
    expect(screen.getByText('Mode: chat')).toBeInTheDocument();
    expect(screen.getByText('Strategy: balanced')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Update preferences' }));

    await waitFor(() => {
      expect(updatePreferencesRequest).toHaveBeenCalledWith({
        defaultMode: 'solo',
        modelStrategy: 'quality',
        networkEnabledHint: true,
        onboardingCompleted: true
      });
    });

    expect(screen.getByText('Mode: solo')).toBeInTheDocument();
    expect(screen.getByText('Strategy: quality')).toBeInTheDocument();
  });
});
