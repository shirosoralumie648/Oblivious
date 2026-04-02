import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi, beforeEach } from 'vitest';

import type { UserPreferences } from '../../types/api';

const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'chat' as const,
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    } as UserPreferences,
    status: 'authenticated' as const,
    user: { email: 'user@example.com', id: 'u1' }
  },
  updatePreferences: vi.fn(async (preferences) => preferences)
}));

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

import { SettingsPage } from './SettingsPage';

describe('SettingsPage', () => {
  beforeEach(() => {
    appContext.authState.preferences = {
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    } as UserPreferences;
    appContext.updatePreferences.mockClear();
  });

  it('renders the current workspace preferences', () => {
    appContext.authState.preferences = {
      defaultMode: 'solo',
      modelStrategy: 'quality',
      networkEnabledHint: true,
      onboardingCompleted: true
    } as UserPreferences;

    render(<SettingsPage />);

    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument();
    expect(screen.getByLabelText('Default mode')).toHaveValue('solo');
    expect(screen.getByLabelText('Model strategy')).toHaveValue('quality');
    expect(screen.getByLabelText('Enable web suggestions')).toBeChecked();
    expect(screen.getByText('Onboarding complete')).toBeInTheDocument();
  });

  it('saves updated preferences', async () => {
    render(<SettingsPage />);

    fireEvent.change(screen.getByLabelText('Default mode'), { target: { value: 'solo' } });
    fireEvent.change(screen.getByLabelText('Model strategy'), { target: { value: 'cost' } });
    fireEvent.click(screen.getByLabelText('Enable web suggestions'));
    fireEvent.click(screen.getByRole('button', { name: 'Save preferences' }));

    await waitFor(() => {
      expect(appContext.updatePreferences).toHaveBeenCalledWith({
        defaultMode: 'solo',
        modelStrategy: 'cost',
        networkEnabledHint: true,
        onboardingCompleted: true
      });
    });

    expect(screen.getByText('Preferences saved.')).toBeInTheDocument();
  });
});
