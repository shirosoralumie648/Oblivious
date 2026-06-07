import type { UserPreferences } from '../../types/api';

export function resolveWorkspaceLandingPath(preferences: UserPreferences | null | undefined) {
  if (!preferences || !preferences.onboardingCompleted) {
    return '/onboarding';
  }

  if (preferences.defaultMode === 'solo') {
    return '/solo/new';
  }
  if (String(preferences.defaultMode) === 'agent') {
    return '/agents';
  }
  return '/chat';
}
