import type { UserPreferences } from '../../types/api';

export function resolveWorkspaceLandingPath(preferences: UserPreferences | null | undefined) {
  if (!preferences || !preferences.onboardingCompleted) {
    return '/onboarding';
  }

  return preferences.defaultMode === 'solo' ? '/solo/new' : '/chat';
}
