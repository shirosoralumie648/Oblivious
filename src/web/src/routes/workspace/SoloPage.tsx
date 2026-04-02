import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { useAppContext } from '../../app/providers';
import { createChatApi } from '../../features/chat/api';
import { createHttpClient } from '../../services/http/client';
import type { ConversationSummary } from '../../types/api';

export function SoloPage() {
  const { authState } = useAppContext();
  const navigate = useNavigate();
  const chatApi = useMemo(() => createChatApi(createHttpClient()), []);
  const [conversations, setConversations] = useState<ConversationSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    const loadConversations = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const nextConversations = await chatApi.listConversations();
        if (!cancelled) {
          setConversations(nextConversations);
        }
      } catch {
        if (!cancelled) {
          setConversations([]);
          setError('Unable to load solo workspace.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadConversations();

    return () => {
      cancelled = true;
    };
  }, [chatApi]);

  const latestConversation = conversations[0] ?? null;

  const handleStartSoloRun = async () => {
    setIsCreating(true);
    setError(null);

    try {
      const conversation = await chatApi.createConversation({ title: 'SOLO run' });
      navigate(`/chat/${conversation.id}`);
    } catch {
      setError('Unable to start a solo run.');
    } finally {
      setIsCreating(false);
    }
  };

  const handleContinueLatest = () => {
    if (!latestConversation) {
      return;
    }

    navigate(`/chat/${latestConversation.id}`);
  };

  return (
    <section>
      <h1>SOLO</h1>
      <p>Launch a focused run with your current workspace defaults, then continue execution in chat.</p>
      {isLoading ? <p>Loading solo workspace…</p> : null}
      {error ? <p>{error}</p> : null}
      <p>Default mode: {authState.preferences?.defaultMode ?? 'chat'}</p>
      <p>Model strategy: {authState.preferences?.modelStrategy ?? 'balanced'}</p>
      <p>Web suggestions: {authState.preferences?.networkEnabledHint ? 'Enabled' : 'Disabled'}</p>
      <button disabled={isCreating} onClick={() => void handleStartSoloRun()} type="button">
        Start solo run
      </button>
      {latestConversation ? (
        <section>
          <h2>Recent workspace thread</h2>
          <p>{latestConversation.title}</p>
          <button onClick={handleContinueLatest} type="button">
            Continue latest thread
          </button>
        </section>
      ) : !isLoading ? (
        <p>No existing threads yet. Start a solo run to create your first workspace thread.</p>
      ) : null}
    </section>
  );
}
