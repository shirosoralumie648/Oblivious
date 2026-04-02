import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { useAppContext } from '../../app/providers';
import type { UserPreferences } from '../../types/api';

const defaultPreferences: UserPreferences = {
  defaultMode: 'chat',
  modelStrategy: 'balanced',
  networkEnabledHint: false,
  onboardingCompleted: false
};

export function OnboardingPage() {
  const navigate = useNavigate();
  const { authState, updatePreferences } = useAppContext();
  const initialPreferences = useMemo(
    () => authState.preferences ?? { ...defaultPreferences, defaultMode: '' as 'chat' | 'solo' },
    [authState.preferences]
  );
  const [defaultMode, setDefaultMode] = useState<UserPreferences['defaultMode'] | ''>(initialPreferences.defaultMode);
  const [modelStrategy, setModelStrategy] = useState(initialPreferences.modelStrategy);
  const [networkEnabledHint, setNetworkEnabledHint] = useState(initialPreferences.networkEnabledHint);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const hasSelectedMode = defaultMode === 'chat' || defaultMode === 'solo';

  const handleComplete = async (nextMode: UserPreferences['defaultMode']) => {
    setError(null);
    setIsSubmitting(true);

    try {
      await updatePreferences({
        defaultMode: nextMode,
        modelStrategy,
        networkEnabledHint,
        onboardingCompleted: true
      });
      navigate('/chat', { replace: true });
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : 'Unable to save onboarding');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleSkip = async () => {
    await handleComplete('chat');
  };

  return (
    <main>
      <h1>Onboarding</h1>
      <p>Choose how you want to start using the workspace.</p>
      <div>
        <button onClick={() => setDefaultMode('chat')} type="button">
          Start with Chat
        </button>
        <button onClick={() => setDefaultMode('solo')} type="button">
          Start with SOLO
        </button>
      </div>
      {hasSelectedMode ? (
        <section>
          <h2>Default model strategy</h2>
          <label>
            Strategy
            <select onChange={(event) => setModelStrategy(event.target.value)} value={modelStrategy}>
              <option value="balanced">Balanced</option>
              <option value="quality">High quality</option>
              <option value="cost">Low cost</option>
            </select>
          </label>
          <label>
            <input
              checked={networkEnabledHint}
              onChange={(event) => setNetworkEnabledHint(event.target.checked)}
              type="checkbox"
            />
            Enable web suggestions
          </label>
          {error ? <p>{error}</p> : null}
          <button
            disabled={isSubmitting || !hasSelectedMode}
            onClick={() => void handleComplete(defaultMode)}
            type="button"
          >
            Continue to workspace
          </button>
        </section>
      ) : null}
      <button disabled={isSubmitting} onClick={() => void handleSkip()} type="button">
        Skip for now
      </button>
    </main>
  );
}
