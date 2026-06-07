import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { RiArrowLeftLine, RiSave3Line } from '@remixicon/react';

import { useAppContext } from '../../app/providers';
import { McpServersPanel } from '../../features/mcp/McpServersPanel';

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
    <section className="mx-auto max-w-6xl space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Workspace control</p>
        <h1 className="font-heading text-3xl font-semibold text-foreground">Settings</h1>
      </header>

      <section className="rounded-lg border border-border bg-card p-5">
        <div className="grid gap-4 lg:grid-cols-3">
          <label className="text-sm font-medium text-foreground">
            Default mode
            <select
              className="mt-2 min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 py-2 text-sm text-foreground"
              onChange={(event) => setDefaultMode(event.target.value as 'chat' | 'solo')}
              value={defaultMode}
            >
              <option value="chat">chat</option>
              <option value="solo">solo</option>
            </select>
          </label>
          <label className="text-sm font-medium text-foreground">
            Model strategy
            <select
              className="mt-2 min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 py-2 text-sm text-foreground"
              onChange={(event) => setModelStrategy(event.target.value)}
              value={modelStrategy}
            >
              <option value="balanced">balanced</option>
              <option value="quality">quality</option>
              <option value="cost">cost</option>
            </select>
          </label>
          <label className="flex min-h-[44px] items-center gap-3 self-end text-sm font-medium text-foreground">
            <input checked={networkEnabledHint} onChange={() => setNetworkEnabledHint((current) => !current)} type="checkbox" />
            Enable web suggestions
          </label>
        </div>
        <div className="mt-5 flex flex-wrap items-center gap-3">
          <p className="text-sm text-muted-foreground">{preferences.onboardingCompleted ? 'Onboarding complete' : 'Onboarding pending'}</p>
          <button
            className="inline-flex min-h-[44px] items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground"
            onClick={() => void handleSave()}
            type="button"
          >
            <RiSave3Line className="size-4" aria-hidden="true" />
            Save preferences
          </button>
          <button
            className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-input bg-input/30 px-4 py-2 text-sm font-semibold text-foreground"
            onClick={() => navigate('/chat')}
            type="button"
          >
            <RiArrowLeftLine className="size-4" aria-hidden="true" />
            Return to chat
          </button>
          {savedMessage ? <p className="text-sm font-medium text-primary">{savedMessage}</p> : null}
        </div>
      </section>

      <McpServersPanel />
    </section>
  );
}
