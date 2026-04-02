import { useEffect, useMemo, useState } from 'react';

import { useAppContext } from '../../app/providers';
import type { UserPreferences } from '../../types/api';

const defaultPreferences: UserPreferences = {
  defaultMode: 'chat',
  modelStrategy: 'balanced',
  networkEnabledHint: false,
  onboardingCompleted: false
};

export function SettingsPage() {
  const { authState, updatePreferences } = useAppContext();
  const initialPreferences = useMemo(() => authState.preferences ?? defaultPreferences, [authState.preferences]);
  const [defaultMode, setDefaultMode] = useState<UserPreferences['defaultMode']>(initialPreferences.defaultMode);
  const [modelStrategy, setModelStrategy] = useState(initialPreferences.modelStrategy);
  const [networkEnabledHint, setNetworkEnabledHint] = useState(initialPreferences.networkEnabledHint);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  useEffect(() => {
    setDefaultMode(initialPreferences.defaultMode);
    setModelStrategy(initialPreferences.modelStrategy);
    setNetworkEnabledHint(initialPreferences.networkEnabledHint);
  }, [initialPreferences]);

  const handleSave = async () => {
    setError(null);
    setStatusMessage(null);
    setIsSubmitting(true);

    try {
      await updatePreferences({
        defaultMode,
        modelStrategy,
        networkEnabledHint,
        onboardingCompleted: initialPreferences.onboardingCompleted
      });
      setStatusMessage('Preferences saved.');
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : 'Unable to save preferences.');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <section>
      <h1>Settings</h1>
      <p>Update your default workspace behavior and response strategy.</p>
      <label>
        Default mode
        <select onChange={(event) => setDefaultMode(event.target.value as UserPreferences['defaultMode'])} value={defaultMode}>
          <option value="chat">Chat</option>
          <option value="solo">SOLO</option>
        </select>
      </label>
      <label>
        Model strategy
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
      <p>{initialPreferences.onboardingCompleted ? 'Onboarding complete' : 'Onboarding pending'}</p>
      {error ? <p>{error}</p> : null}
      {statusMessage ? <p>{statusMessage}</p> : null}
      <button disabled={isSubmitting} onClick={() => void handleSave()} type="button">
        Save preferences
      </button>
    </section>
  );
}
