import { describe, expect, it } from 'vitest';

import { resolveWorkspaceLandingPath } from './workspaceLanding';

describe('resolveWorkspaceLandingPath', () => {
  it('routes incomplete onboarding users to onboarding', () => {
    expect(
      resolveWorkspaceLandingPath({
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: false,
        onboardingCompleted: false
      })
    ).toBe('/onboarding');
  });

  it('routes completed chat users to /chat', () => {
    expect(
      resolveWorkspaceLandingPath({
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: false,
        onboardingCompleted: true
      })
    ).toBe('/chat');
  });

  it('routes completed solo users to /solo/new', () => {
    expect(
      resolveWorkspaceLandingPath({
        defaultMode: 'solo',
        modelStrategy: 'quality',
        networkEnabledHint: true,
        onboardingCompleted: true
      })
    ).toBe('/solo/new');
  });

  it('routes completed agent users to /agents', () => {
    expect(
      resolveWorkspaceLandingPath({
        defaultMode: 'agent',
        modelStrategy: 'balanced',
        networkEnabledHint: true,
        onboardingCompleted: true
      } as never)
    ).toBe('/agents');
  });
});
