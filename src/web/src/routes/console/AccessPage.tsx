import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary, ConsoleApiTokenUsageItem, RelayApiToken } from '../../types/api';

export function AccessPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [apiTokens, setApiTokens] = useState<RelayApiToken[]>([]);
  const [allowedModels, setAllowedModels] = useState('gpt-4o,gpt-4o-mini');
  const [createdRawToken, setCreatedRawToken] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [quotaLimit, setQuotaLimit] = useState('');
  const [selectedUsageTokenId, setSelectedUsageTokenId] = useState<string | null>(null);
  const [tokenError, setTokenError] = useState<string | null>(null);
  const [tokenGroup, setTokenGroup] = useState('');
  const [tokenName, setTokenName] = useState('');
  const [tokenUsage, setTokenUsage] = useState<Record<string, ConsoleApiTokenUsageItem[]>>({});
  const [usageLoadingTokenId, setUsageLoadingTokenId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadAccess = async () => {
      try {
        const [summary, tokens] = await Promise.all([consoleApi.getAccess(), consoleApi.listApiTokens()]);
        if (!cancelled) {
          setAccessSummary(summary);
          setApiTokens(tokens);
          setLoadError(null);
          setIsLoading(false);
        }
      } catch {
        if (!cancelled) {
          setAccessSummary(null);
          setLoadError('Unable to load access summary.');
          setIsLoading(false);
        }
      }
    };

    void loadAccess();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  const handleCreateToken = async () => {
    const models = allowedModels
      .split(',')
      .map((model) => model.trim())
      .filter(Boolean);
    const normalizedQuotaLimit = quotaLimit.trim();
    let parsedQuotaLimit: number | undefined;
    if (normalizedQuotaLimit !== '') {
      parsedQuotaLimit = Number(normalizedQuotaLimit);
    }
    if (parsedQuotaLimit !== undefined && (!Number.isFinite(parsedQuotaLimit) || parsedQuotaLimit <= 0)) {
      setTokenError('Quota limit must be greater than zero.');
      return;
    }
    const normalizedExpiresAt = expiresAt.trim();
    try {
      const created = await consoleApi.createApiToken({
        ...(normalizedExpiresAt ? { expiresAt: normalizedExpiresAt } : {}),
        modelLimits: models,
        modelLimitsEnabled: models.length > 0,
        name: tokenName.trim(),
        ...(typeof parsedQuotaLimit === 'number' ? { quotaLimit: parsedQuotaLimit } : {}),
        userGroup: tokenGroup.trim()
      });
      setApiTokens((current) => [created.token, ...current]);
      setCreatedRawToken(created.rawToken);
      setExpiresAt('');
      setQuotaLimit('');
      setTokenError(null);
      setTokenGroup('');
      setTokenName('');
    } catch {
      setTokenError('Unable to create API token.');
    }
  };

  const handleRevokeToken = async (tokenId: string) => {
    try {
      await consoleApi.revokeApiToken(tokenId);
      setApiTokens((current) => current.map((token) => (token.id === tokenId ? { ...token, status: 'revoked' } : token)));
      setTokenError(null);
    } catch {
      setTokenError('Unable to revoke API token.');
    }
  };

  const handleViewTokenUsage = async (tokenId: string) => {
    setSelectedUsageTokenId(tokenId);
    if (tokenUsage[tokenId]) {
      return;
    }
    setUsageLoadingTokenId(tokenId);
    try {
      const usage = await consoleApi.listApiTokenUsage(tokenId);
      setTokenUsage((current) => ({ ...current, [tokenId]: usage }));
      setTokenError(null);
    } catch {
      setTokenError('Unable to load API token usage.');
    } finally {
      setUsageLoadingTokenId(null);
    }
  };

  return (
    <ConsoleWorkbenchLayout
      accessSummary={accessSummary}
      description="Review the exact scope and session context behind this console."
      errorMessage={loadError}
      siblingLinks={[{ label: 'Open models', to: '/console/models' }]}
      title="Access"
    >
      {isLoading ? (
        <p>Loading access summary…</p>
      ) : accessSummary ? (
        <>
          <p>This console reflects the active workspace and current session.</p>
          <p className="break-words">{`User: ${accessSummary.userEmail}`}</p>
          <p className="break-words">{`Workspace: ${accessSummary.workspaceId}`}</p>
          <p className="break-words">{`Session: ${accessSummary.sessionId}`}</p>
          <p>{`Default mode: ${accessSummary.defaultMode}`}</p>
          <section className="min-w-0">
            <h2>API tokens</h2>
            {tokenError ? <p role="alert">{tokenError}</p> : null}
            {createdRawToken ? <p className="break-words">{createdRawToken}</p> : null}
            <div className="grid min-w-0 gap-3">
              <label>
                Token name
                <input className="max-w-full" value={tokenName} onChange={(event) => setTokenName(event.target.value)} />
              </label>
              <label>
                Allowed models
                <input className="max-w-full" value={allowedModels} onChange={(event) => setAllowedModels(event.target.value)} />
              </label>
              <label>
                Routing group
                <input className="max-w-full" value={tokenGroup} onChange={(event) => setTokenGroup(event.target.value)} />
              </label>
              <label>
                Quota limit
                <input className="max-w-full" inputMode="decimal" value={quotaLimit} onChange={(event) => setQuotaLimit(event.target.value)} />
              </label>
              <label>
                Expires at
                <input
                  className="max-w-full"
                  placeholder="2026-06-30T00:00:00Z"
                  value={expiresAt}
                  onChange={(event) => setExpiresAt(event.target.value)}
                />
              </label>
              <button disabled={tokenName.trim() === ''} onClick={handleCreateToken} type="button">
                Create API token
              </button>
            </div>
            {apiTokens.length === 0 ? (
              <p>No API tokens yet.</p>
            ) : (
              <ul className="min-w-0">
                {apiTokens.map((token) => (
                  <li className="min-w-0" key={token.id}>
                    <span className="break-words">{token.name}</span>
                    <span className="break-words">{token.tokenPrefix}</span>
                    <span>{token.status}</span>
                    <span className="break-words">{token.userGroup || 'default group'}</span>
                    <span className="break-words">{token.modelLimitsEnabled ? token.modelLimits.join(', ') : 'all models'}</span>
                    <span className="break-words">{formatTokenQuota(token)}</span>
                    <span className="break-words">{formatTokenExpiry(token)}</span>
                    <button aria-label={`View usage for ${token.name}`} onClick={() => void handleViewTokenUsage(token.id)} type="button">
                      Usage
                    </button>
                    {token.status === 'active' ? (
                      <button onClick={() => void handleRevokeToken(token.id)} type="button">
                        Revoke
                      </button>
                    ) : null}
                    {selectedUsageTokenId === token.id ? (
                      <TokenUsageList isLoading={usageLoadingTokenId === token.id} usage={tokenUsage[token.id] ?? []} />
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
          </section>
        </>
      ) : (
        <p>Access summary unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}

function formatTokenQuota(token: RelayApiToken) {
  if (typeof token.quotaLimit === 'number') {
    return `${formatNumber(token.usedQuota)} / ${formatNumber(token.quotaLimit)} quota`;
  }
  return `${formatNumber(token.usedQuota)} quota used`;
}

function formatTokenExpiry(token: RelayApiToken) {
  if (!token.expiresAt) {
    return 'no expiry';
  }
  return `expires ${token.expiresAt}`;
}

function TokenUsageList({ isLoading, usage }: { isLoading: boolean; usage: ConsoleApiTokenUsageItem[] }) {
  if (isLoading) {
    return <p>Loading token usage…</p>;
  }
  if (usage.length === 0) {
    return <p>No usage recorded for this token.</p>;
  }
  return (
    <ul className="min-w-0">
      {usage.map((item) => (
        <li className="min-w-0" key={item.id}>
          <span className="break-words">{item.requestId || item.id}</span>
          <span className="break-words">{item.model}</span>
          <span className="break-words">{item.apiType || 'unknown api'}</span>
          <span>{item.status}</span>
          <span>{`${item.totalTokens} tokens`}</span>
          <span>{formatCurrency(item.cost)}</span>
          <span>{`${item.latencyMs} ms`}</span>
        </li>
      ))}
    </ul>
  );
}

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : String(value);
}

function formatCurrency(value: number) {
  return `$${value}`;
}
