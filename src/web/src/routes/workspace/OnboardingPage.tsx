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
    <section className="mx-auto max-w-5xl space-y-6" data-gsap-scope="onboarding">
      <div className="rounded-lg border border-[#d7d2c4] bg-white p-6 shadow-sm" data-gsap-item>
        <p className="text-sm font-semibold text-[#1a614f]">First-run setup</p>
        <h1 className="mt-2 font-heading text-3xl font-semibold">Onboarding</h1>
        <p className="mt-3 max-w-2xl text-sm leading-6 text-[#625b4f]">Choose how you want to start working in the workspace. This sets the first commercial path without hiding Relay, quota, or tool boundaries.</p>
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <button
          aria-label="Start with Chat"
          className={`rounded-lg border p-5 text-left transition ${defaultMode === 'chat' ? 'border-[#1a614f] bg-[#e9f2ee]' : 'border-[#d7d2c4] bg-white hover:border-[#1a614f]/40'}`}
          data-gsap-item
          data-gsap-magnetic
          onClick={() => setDefaultMode('chat')}
          type="button"
        >
          <span className="block text-lg font-semibold">Start with Chat</span>
          <span className="mt-2 block text-sm leading-6 text-[#625b4f]">Open the Relay-backed conversation workspace with Knowledge binding and SOLO handoff.</span>
        </button>
        <button
          aria-label="Start with SOLO"
          className={`rounded-lg border p-5 text-left transition ${defaultMode === 'solo' ? 'border-[#1a614f] bg-[#e9f2ee]' : 'border-[#d7d2c4] bg-white hover:border-[#1a614f]/40'}`}
          data-gsap-item
          data-gsap-magnetic
          onClick={() => setDefaultMode('solo')}
          type="button"
        >
          <span className="block text-lg font-semibold">Start with SOLO</span>
          <span className="mt-2 block text-sm leading-6 text-[#625b4f]">Begin with agent runs, budget limits, approvals, retries, and tool scopes visible.</span>
        </button>
      </div>
      <button className="min-h-[44px] rounded-lg border border-[#d7d2c4] bg-white px-4 text-sm font-semibold transition hover:bg-[#fbfaf7]" data-gsap-item onClick={handleSkip} type="button">
        Skip for now
      </button>
      {defaultMode !== null ? (
        <div className="rounded-lg border border-[#d7d2c4] bg-white p-5 shadow-sm" data-gsap-item>
          <p className="font-semibold">Default model strategy</p>
          <label className="mt-4 block space-y-2 text-sm">
            <span>Model strategy</span>
            <select className="min-h-[44px] rounded-lg border border-[#cfc8b7] bg-white px-3" onChange={(event) => setModelStrategy(event.target.value)} value={modelStrategy}>
              <option value="balanced">balanced</option>
              <option value="quality">quality</option>
              <option value="cost">cost</option>
            </select>
          </label>
          <button className="mt-4 min-h-[44px] rounded-lg bg-[#1a614f] px-4 font-semibold text-white transition hover:bg-[#207a63]" data-gsap-magnetic onClick={() => void handleContinue()} type="button">
            Continue to workspace
          </button>
        </div>
      ) : null}
    </section>
  );
}
