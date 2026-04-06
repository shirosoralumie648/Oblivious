import { useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { useAppContext } from '../../app/providers';

export function OnboardingPage() {
  const navigate = useNavigate();
  const { updatePreferences } = useAppContext();
  const [defaultMode, setDefaultMode] = useState<'chat' | 'solo' | null>(null);
  const [modelStrategy, setModelStrategy] = useState('balanced');

  const handleContinue = async () => {
    if (defaultMode === null) {
      return;
    }

    await updatePreferences({
      defaultMode,
      modelStrategy,
      networkEnabledHint: false,
      onboardingCompleted: true
    });

    navigate(defaultMode === 'solo' ? '/solo/new' : '/chat');
  };

  const handleSkip = () => {
    navigate('/chat');
  };

  return (
    <section>
      <h1>Onboarding</h1>
      <p>Choose how you want to start working in the workspace.</p>
      <button onClick={() => setDefaultMode('chat')} type="button">
        Start with Chat
      </button>
      <button onClick={() => setDefaultMode('solo')} type="button">
        Start with SOLO
      </button>
      <button onClick={handleSkip} type="button">
        Skip for now
      </button>
      {defaultMode !== null ? (
        <div>
          <p>Default model strategy</p>
          <label>
            Model strategy
            <select onChange={(event) => setModelStrategy(event.target.value)} value={modelStrategy}>
              <option value="balanced">balanced</option>
              <option value="quality">quality</option>
              <option value="cost">cost</option>
            </select>
          </label>
          <button onClick={() => void handleContinue()} type="button">
            Continue to workspace
          </button>
        </div>
      ) : null}
    </section>
  );
}
