import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { useAppContext } from '../../app/providers';

export function SettingsPage() {
  const navigate = useNavigate();
  const { authState, updatePreferences } = useAppContext();
  const preferences = authState.preferences ?? {
    defaultMode: 'chat' as const,
    modelStrategy: 'balanced',
    networkEnabledHint: false,
    onboardingCompleted: false
  };
  const [defaultMode, setDefaultMode] = useState(preferences.defaultMode);
  const [modelStrategy, setModelStrategy] = useState(preferences.modelStrategy);
  const [networkEnabledHint, setNetworkEnabledHint] = useState(preferences.networkEnabledHint);
  const [savedMessage, setSavedMessage] = useState('');

  useEffect(() => {
    setDefaultMode(preferences.defaultMode);
    setModelStrategy(preferences.modelStrategy);
    setNetworkEnabledHint(preferences.networkEnabledHint);
    setSavedMessage('');
  }, [preferences.defaultMode, preferences.modelStrategy, preferences.networkEnabledHint]);

  const handleSave = async () => {
    await updatePreferences({
      defaultMode,
      modelStrategy,
      networkEnabledHint,
      onboardingCompleted: preferences.onboardingCompleted
    });
    setSavedMessage('Preferences saved.');
  };

  return (
    <section>
      <h1>Settings</h1>
      <label>
        Default mode
        <select onChange={(event) => setDefaultMode(event.target.value as 'chat' | 'solo')} value={defaultMode}>
          <option value="chat">chat</option>
          <option value="solo">solo</option>
        </select>
      </label>
      <label>
        Model strategy
        <select onChange={(event) => setModelStrategy(event.target.value)} value={modelStrategy}>
          <option value="balanced">balanced</option>
          <option value="quality">quality</option>
          <option value="cost">cost</option>
        </select>
      </label>
      <label>
        Enable web suggestions
        <input checked={networkEnabledHint} onChange={() => setNetworkEnabledHint((current) => !current)} type="checkbox" />
      </label>
      <p>{preferences.onboardingCompleted ? 'Onboarding complete' : 'Onboarding pending'}</p>
      <button onClick={() => void handleSave()} type="button">
        Save preferences
      </button>
      <button onClick={() => navigate('/chat')} type="button">
        Return to chat
      </button>
      {savedMessage ? <p>{savedMessage}</p> : null}
    </section>
  );
}
