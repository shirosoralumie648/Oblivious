import { useEffect, useMemo, useState } from 'react';
import { RiLoader4Line, RiRefreshLine, RiSave3Line } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';

import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type { RelayPricingSettings, UsageLimitSettings, UsageLogEntry, UsageLogFilter } from '../../types/admin';

type SettingsState = {
  loading: boolean;
  saving: boolean;
  savingUsage: boolean;
  error: string | null;
  saved: string | null;
  modelText: string;
  groupText: string;
  usageLimits: UsageLimitSettings[];
  usageLimitSignals: Record<string, UsageLimitRuntimeSignal>;
  usageLimitSignalError: string | null;
  selectedUsageLimitKey: string | null;
  usageScopeType: string;
  usageScopeId: string;
  usageLimitType: string;
  usagePeriod: string;
  usageLimitValue: string;
  usageEnabled: boolean;
};

const initialState: SettingsState = {
  loading: true,
  saving: false,
  savingUsage: false,
  error: null,
  saved: null,
  modelText: '{}',
  groupText: '{}',
  usageLimits: [],
  usageLimitSignals: {},
  usageLimitSignalError: null,
  selectedUsageLimitKey: null,
  usageScopeType: 'organization',
  usageScopeId: '',
  usageLimitType: 'tokens',
  usagePeriod: 'minute',
  usageLimitValue: '1000',
  usageEnabled: true,
};

type EditableUsageLimitSettings = UsageLimitSettings & {
  id?: string;
  scopeType?: string;
  scope_type?: string;
  scopeId?: string;
  scope_id?: string;
  limitType?: string;
  limit_type?: string;
  period?: string;
  limitValue?: number;
  limit_value?: number;
  enabled?: boolean;
};

type UsageLimitFormPayload = {
  id?: string;
  scopeType: string;
  scopeId: string;
  limitType: string;
  period: string;
  limitValue: number;
  enabled: boolean;
};

type UsageLimitUpdatePayload = UsageLimitSettings & UsageLimitFormPayload;

type UsageLimitRuntimeSignal = {
  recentLimitHits: number;
  latestLimitHit?: UsageLogEntry;
  latestRecovery?: UsageLogEntry;
};

const builtInUsageLimitTypes = new Set(['tokens', 'request_tokens', 'requests', 'concurrent_requests']);
const usageLimitEnforcementCodes = new Set(['relay_rate_limited', 'rate_limited', 'usage_limit_exceeded']);

function prettyMap(input: Record<string, number> | undefined) {
  return JSON.stringify(input ?? {}, null, 2);
}

function parseMultiplierMap(label: string, value: string): Record<string, number> {
  let parsed: unknown;
  try {
    parsed = JSON.parse(value.trim() || '{}');
  } catch {
    throw new Error(`${label} must be valid JSON.`);
  }

  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(`${label} must be a JSON object.`);
  }

  return Object.entries(parsed as Record<string, unknown>).reduce<Record<string, number>>((acc, [key, raw]) => {
    const multiplier = typeof raw === 'number' ? raw : Number(raw);
    if (!Number.isFinite(multiplier) || multiplier < 0) {
      throw new Error(`${label} contains an invalid multiplier for ${key}.`);
    }
    acc[key] = multiplier;
    return acc;
  }, {});
}

function stateFromSettings(settings: RelayPricingSettings): Pick<SettingsState, 'modelText' | 'groupText'> {
  return {
    modelText: prettyMap(settings.modelMultipliers),
    groupText: prettyMap(settings.groupMultipliers),
  };
}

function parsePositiveInteger(label: string, value: string): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`${label} must be a positive integer.`);
  }
  return parsed;
}

function usageScopeType(settings: UsageLimitSettings): string {
  const editable = settings as EditableUsageLimitSettings;
  return editable.scopeType ?? editable.scope_type ?? (settings.userId || settings.userID ? 'user' : 'organization');
}

function usageScopeId(settings: UsageLimitSettings): string {
  const editable = settings as EditableUsageLimitSettings;
  return editable.scopeId ?? editable.scope_id ?? settings.userId ?? settings.userID ?? settings.organizationId ?? settings.organizationID ?? '';
}

function usageLimitType(settings: UsageLimitSettings): string {
  const editable = settings as EditableUsageLimitSettings;
  if (editable.limitType || editable.limit_type) {
    return editable.limitType ?? editable.limit_type ?? 'tokens';
  }
  if ((settings.maxTokensPerRequest ?? 0) > 0) {
    return 'request_tokens';
  }
  if ((settings.maxConcurrentRequests ?? 0) > 0 && (settings.maxTokensPerWindow ?? 0) <= 0) {
    return 'concurrent_requests';
  }
  return 'tokens';
}

function usageLimitPeriod(settings: UsageLimitSettings): string {
  const editable = settings as EditableUsageLimitSettings;
  return editable.period ?? (settings.windowSeconds ? `${settings.windowSeconds}s` : 'minute');
}

function usageLimitValue(settings: UsageLimitSettings): number {
  const editable = settings as EditableUsageLimitSettings;
  const explicitValue = editable.limitValue ?? editable.limit_value;
  if (explicitValue !== undefined) {
    return explicitValue;
  }
  switch (usageLimitType(settings)) {
    case 'request_tokens':
      return settings.maxTokensPerRequest ?? 0;
    case 'concurrent_requests':
      return settings.maxConcurrentRequests;
    default:
      return settings.maxTokensPerWindow;
  }
}

function usageLimitEnabled(settings: UsageLimitSettings): boolean {
  const editable = settings as EditableUsageLimitSettings;
  return editable.enabled ?? true;
}

function usageScopeKey(settings: UsageLimitSettings): string {
  const editable = settings as EditableUsageLimitSettings;
  return editable.id ?? `${usageScopeType(settings)}:${usageScopeId(settings)}:${usageLimitType(settings)}:${usageLimitPeriod(settings)}`;
}

function usageScopeLabel(settings: UsageLimitSettings): string {
  const scopeId = usageScopeId(settings);
  return scopeId ? `${usageScopeType(settings)} ${scopeId}` : 'Organization scope';
}

function usageQuotaModeLabel(settings: UsageLimitSettings): string {
  return settings.quotaMode || (settings.userId || settings.userID ? 'user' : 'organization');
}

function usageEditLabel(settings: UsageLimitSettings): string {
  return `Edit ${usageScopeLabel(settings)} ${usageLimitType(settings)} ${usageLimitPeriod(settings)}`;
}

function usageLimitPayload(input: UsageLimitFormPayload, existing?: UsageLimitSettings): UsageLimitUpdatePayload {
  const payload: UsageLimitUpdatePayload = {
    ...input,
    userId: input.scopeType === 'user' ? input.scopeId : undefined,
    quotaMode: input.scopeType === 'user' ? 'user' : 'organization',
    maxConcurrentRequests: existing?.maxConcurrentRequests ?? 0,
    windowSeconds: usagePeriodSeconds(input.period),
    maxTokensPerWindow: existing?.maxTokensPerWindow ?? 0,
    maxTokensPerRequest: existing?.maxTokensPerRequest ?? 0,
  };
  switch (input.limitType) {
    case 'concurrent_requests':
      payload.maxConcurrentRequests = input.limitValue;
      break;
    case 'request_tokens':
      payload.maxTokensPerRequest = input.limitValue;
      break;
    default:
      payload.maxTokensPerWindow = input.limitValue;
      break;
  }
  return payload;
}

function usageLimitLogFilter(settings: UsageLimitSettings, status: 'error' | 'success'): UsageLogFilter {
  const scopeType = usageScopeType(settings);
  const scopeId = usageScopeId(settings);
  const limitType = usageLimitType(settings);
  const filter: UsageLogFilter = {
    organizationID: settings.organizationId ?? settings.organizationID,
    status,
    limit: status === 'error' ? 50 : 1,
  };
  if (scopeType === 'organization' && scopeId) {
    filter.organizationID = scopeId;
  }
  if (scopeType === 'user' && scopeId) {
    filter.userID = scopeId;
  }
  if (!builtInUsageLimitTypes.has(limitType)) {
    filter.featureType = limitType;
  }
  return filter;
}

function isUsageLimitHit(log: UsageLogEntry) {
  return log.statusCode === 429 || usageLimitEnforcementCodes.has(log.errorCode ?? '');
}

function isAfter(left?: string, right?: string) {
  if (!left || !right) {
    return false;
  }
  return new Date(left).getTime() > new Date(right).getTime();
}

function usageLimitRuntimeStatus(signal?: UsageLimitRuntimeSignal) {
  if (!signal) {
    return 'No signal';
  }
  if (signal.recentLimitHits > 0) {
    return isAfter(signal.latestRecovery?.createdAt, signal.latestLimitHit?.createdAt) ? 'Recovered' : 'Enforcing';
  }
  return signal.latestRecovery ? 'Clear' : 'No traffic';
}

function usageLimitRuntimeDetail(signal?: UsageLimitRuntimeSignal) {
  if (!signal) {
    return 'Runtime logs unavailable';
  }
  if (signal.latestLimitHit) {
    const code = signal.latestLimitHit.errorCode || `HTTP ${signal.latestLimitHit.statusCode ?? 429}`;
    return `${signal.recentLimitHits} recent hit${signal.recentLimitHits === 1 ? '' : 's'} - ${code}`;
  }
  if (signal.latestRecovery) {
    return `Latest success ${signal.latestRecovery.requestId || signal.latestRecovery.id}`;
  }
  return 'No recent relay traffic';
}

async function loadUsageLimitRuntimeSignals(
  api: Pick<ReturnType<typeof createAdminApi>, 'listUsageLogs'>,
  usageLimits: UsageLimitSettings[]
): Promise<Record<string, UsageLimitRuntimeSignal>> {
  const entries = await Promise.all(usageLimits.map(async (settings) => {
    const [errors, successes] = await Promise.all([
      api.listUsageLogs(usageLimitLogFilter(settings, 'error')),
      api.listUsageLogs(usageLimitLogFilter(settings, 'success')),
    ]);
    const limitHits = errors.data.filter(isUsageLimitHit);
    return [
      usageScopeKey(settings),
      {
        recentLimitHits: limitHits.length,
        latestLimitHit: limitHits[0],
        latestRecovery: successes.data[0],
      } satisfies UsageLimitRuntimeSignal,
    ] as const;
  }));
  return Object.fromEntries(entries);
}

function usagePeriodSeconds(period: string): number {
  switch (period) {
    case 'hour':
      return 3600;
    case 'day':
      return 86400;
    case 'month':
      return 2592000;
    default:
      return 60;
  }
}

export function AdminSettingsPage() {
  const [state, setState] = useState<SettingsState>(initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadSettings = async () => {
    setState((current) => ({ ...current, loading: true, error: null, saved: null }));
    try {
      const [settings, usageLimits] = await Promise.all([
        api.getRelayPricingSettings(),
        api.getUsageLimitSettings(),
      ]);
      let usageLimitSignals: Record<string, UsageLimitRuntimeSignal> = {};
      let usageLimitSignalError: string | null = null;
      try {
        usageLimitSignals = await loadUsageLimitRuntimeSignals(api, usageLimits);
      } catch (error) {
        usageLimitSignalError = error instanceof Error ? error.message : 'Unable to load usage-limit runtime signals.';
      }
      setState((current) => ({
        ...current,
        ...stateFromSettings(settings),
        usageLimits,
        usageLimitSignals,
        usageLimitSignalError,
        loading: false,
      }));
    } catch (error) {
      setState((current) => ({
        ...current,
        loading: false,
        error: error instanceof Error ? error.message : 'Unable to load settings.',
      }));
    }
  };

  useEffect(() => {
    void loadSettings();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const saveSettings = async () => {
    setState((current) => ({ ...current, saving: true, error: null, saved: null }));
    try {
      const input: RelayPricingSettings = {
        modelMultipliers: parseMultiplierMap('Model multipliers JSON', state.modelText),
        groupMultipliers: parseMultiplierMap('Group multipliers JSON', state.groupText),
      };
      const updated = await api.updateRelayPricingSettings(input);
      setState((current) => ({
        ...current,
        ...stateFromSettings(updated),
        saving: false,
        saved: 'Settings saved.',
      }));
    } catch (error) {
      setState((current) => ({
        ...current,
        saving: false,
        error: error instanceof Error ? error.message : 'Unable to save settings.',
      }));
    }
  };

  const saveUsageLimit = async () => {
    setState((current) => ({ ...current, savingUsage: true, error: null, saved: null }));
    try {
      const selectedLimit = state.usageLimits.find((settings) => usageScopeKey(settings) === state.selectedUsageLimitKey) as EditableUsageLimitSettings | undefined;
      const input: UsageLimitFormPayload = {
        id: selectedLimit?.id,
        scopeType: state.usageScopeType,
        scopeId: state.usageScopeId.trim(),
        limitType: state.usageLimitType,
        period: state.usagePeriod,
        limitValue: parsePositiveInteger('Limit value', state.usageLimitValue),
        enabled: state.usageEnabled,
      };
      const updated = await api.updateUsageLimitSettings(usageLimitPayload(input, selectedLimit));
      setState((current) => {
        const updatedKey = usageScopeKey(updated);
        const previousKey = current.selectedUsageLimitKey;
        const nextLimits = current.usageLimits.filter((settings) => {
          const key = usageScopeKey(settings);
          return key !== updatedKey && key !== previousKey;
        });
        return {
          ...current,
          usageLimits: [...nextLimits, updated],
          selectedUsageLimitKey: updatedKey,
          savingUsage: false,
          saved: 'Usage limit saved.',
        };
      });
    } catch (error) {
      setState((current) => ({
        ...current,
        savingUsage: false,
        error: error instanceof Error ? error.message : 'Unable to save usage limit.',
      }));
    }
  };

  const editUsageLimit = (settings: UsageLimitSettings) => {
    setState((current) => ({
      ...current,
      selectedUsageLimitKey: usageScopeKey(settings),
      usageScopeType: usageScopeType(settings),
      usageScopeId: usageScopeId(settings),
      usageLimitType: usageLimitType(settings),
      usagePeriod: usageLimitPeriod(settings),
      usageLimitValue: String(usageLimitValue(settings)),
      usageEnabled: usageLimitEnabled(settings),
      saved: null,
    }));
  };

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">Settings</h1>
          <p className="mt-1 text-sm text-muted-foreground">Relay pricing</p>
        </div>
        <div className="flex gap-2">
          <Button type="button" variant="outline" onClick={loadSettings} disabled={state.loading || state.saving || state.savingUsage}>
            <RiRefreshLine className="mr-2 size-4" aria-hidden="true" />
            Refresh
          </Button>
          <Button type="button" onClick={saveSettings} disabled={state.loading || state.saving || state.savingUsage}>
            {state.saving ? <RiLoader4Line className="mr-2 size-4 animate-spin" aria-hidden="true" /> : <RiSave3Line className="mr-2 size-4" aria-hidden="true" />}
            Save Settings
          </Button>
        </div>
      </header>

      {state.error ? (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">{state.error}</div>
      ) : null}
      {state.saved ? (
        <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-700 dark:text-emerald-300">{state.saved}</div>
      ) : null}

      <section className="grid gap-4 xl:grid-cols-2">
        <div className="space-y-3 rounded-lg border bg-card p-4">
          <label htmlFor="relay-model-multipliers" className="text-sm font-medium">
            Model multipliers JSON
          </label>
          <Textarea
            id="relay-model-multipliers"
            className="min-h-80 font-mono text-sm"
            value={state.modelText}
            onChange={(event) => setState((current) => ({ ...current, modelText: event.target.value, saved: null }))}
            spellCheck={false}
          />
        </div>
        <div className="space-y-3 rounded-lg border bg-card p-4">
          <label htmlFor="relay-group-multipliers" className="text-sm font-medium">
            Group multipliers JSON
          </label>
          <Textarea
            id="relay-group-multipliers"
            className="min-h-80 font-mono text-sm"
            value={state.groupText}
            onChange={(event) => setState((current) => ({ ...current, groupText: event.target.value, saved: null }))}
            spellCheck={false}
          />
        </div>
      </section>

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="space-y-3 rounded-lg border bg-card p-4">
          <div>
            <h2 className="text-base font-semibold">Usage limits</h2>
            <p className="mt-1 text-sm text-muted-foreground">Concurrency and token windows by organization or user.</p>
          </div>
          <div className="divide-y rounded-lg border">
            {state.usageLimits.length === 0 ? (
              <div className="px-3 py-4 text-sm text-muted-foreground">No usage limits configured.</div>
            ) : (
              state.usageLimits.map((settings) => (
                <div key={usageScopeKey(settings)} className="grid gap-2 px-3 py-3 text-sm md:grid-cols-[minmax(0,1fr)_repeat(3,120px)_minmax(140px,180px)_80px]">
                  <div>
                    <div className="font-medium">{usageScopeLabel(settings)}</div>
                    <div className="text-xs text-muted-foreground">{settings.organizationId ?? settings.organizationID}</div>
                    <div className="text-xs text-muted-foreground">{`Mode: ${usageQuotaModeLabel(settings)}`}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground">Type</div>
                    <div>{usageLimitType(settings)}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground">Period</div>
                    <div>{usageLimitPeriod(settings)}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground">Limit</div>
                    <div>{usageLimitValue(settings)}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground">Runtime</div>
                    <div>{usageLimitRuntimeStatus(state.usageLimitSignals[usageScopeKey(settings)])}</div>
                    <div className="text-xs text-muted-foreground">{usageLimitRuntimeDetail(state.usageLimitSignals[usageScopeKey(settings)])}</div>
                    {state.usageLimitSignals[usageScopeKey(settings)]?.latestRecovery ? (
                      <div className="text-xs text-muted-foreground">{`Recovery: ${state.usageLimitSignals[usageScopeKey(settings)]?.latestRecovery?.requestId || state.usageLimitSignals[usageScopeKey(settings)]?.latestRecovery?.id}`}</div>
                    ) : null}
                  </div>
                  <div className="flex items-center justify-end">
                    <Button type="button" variant="outline" size="sm" onClick={() => editUsageLimit(settings)} aria-label={usageEditLabel(settings)}>
                      Edit
                    </Button>
                  </div>
                </div>
              ))
            )}
          </div>
          {state.usageLimitSignalError ? (
            <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-800 dark:text-amber-200">{state.usageLimitSignalError}</div>
          ) : null}
        </div>

        <div className="space-y-4 rounded-lg border bg-card p-4">
          <h2 className="text-base font-semibold">Edit limit</h2>
          <div className="space-y-2">
            <label htmlFor="usage-scope-type" className="text-sm font-medium">
              Scope type
            </label>
            <select
              id="usage-scope-type"
              className="min-h-11 w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
              value={state.usageScopeType}
              onChange={(event) => setState((current) => ({ ...current, usageScopeType: event.target.value, saved: null }))}
            >
              <option value="organization">organization</option>
              <option value="user">user</option>
              <option value="feature">feature</option>
              <option value="time">time</option>
            </select>
          </div>
          <div className="space-y-2">
            <label htmlFor="usage-scope-id" className="text-sm font-medium">
              Scope ID
            </label>
            <Input
              id="usage-scope-id"
              className="rounded-lg"
              value={state.usageScopeId}
              onChange={(event) => setState((current) => ({ ...current, usageScopeId: event.target.value, saved: null }))}
              placeholder="org_1, user_1, or feature key"
            />
          </div>
          <div className="space-y-2">
            <label htmlFor="usage-limit-type" className="text-sm font-medium">
              Limit type
            </label>
            <select
              id="usage-limit-type"
              className="min-h-11 w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
              value={state.usageLimitType}
              onChange={(event) => setState((current) => ({ ...current, usageLimitType: event.target.value, saved: null }))}
            >
              <option value="tokens">tokens</option>
              <option value="request_tokens">request_tokens</option>
              <option value="requests">requests</option>
              <option value="concurrent_requests">concurrent_requests</option>
              <option value="workspace_chat">workspace_chat</option>
              <option value="image_generation">image_generation</option>
            </select>
          </div>
          <div className="space-y-2">
            <label htmlFor="usage-period" className="text-sm font-medium">
              Period
            </label>
            <select
              id="usage-period"
              className="min-h-11 w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
              value={state.usagePeriod}
              onChange={(event) => setState((current) => ({ ...current, usagePeriod: event.target.value, saved: null }))}
            >
              <option value="minute">minute</option>
              <option value="hour">hour</option>
              <option value="day">day</option>
              <option value="month">month</option>
            </select>
          </div>
          <div className="space-y-2">
            <label htmlFor="usage-limit-value" className="text-sm font-medium">
              Limit value
            </label>
            <Input
              id="usage-limit-value"
              className="rounded-lg"
              type="number"
              min={1}
              value={state.usageLimitValue}
              onChange={(event) => setState((current) => ({ ...current, usageLimitValue: event.target.value, saved: null }))}
            />
          </div>
          <div className="flex items-center gap-2">
            <input
              id="usage-enabled"
              type="checkbox"
              className="size-4 rounded border-input"
              checked={state.usageEnabled}
              onChange={(event) => setState((current) => ({ ...current, usageEnabled: event.target.checked, saved: null }))}
            />
            <label htmlFor="usage-enabled" className="text-sm font-medium">
              Enabled
            </label>
          </div>
          <Button type="button" className="w-full" onClick={saveUsageLimit} disabled={state.loading || state.saving || state.savingUsage}>
            {state.savingUsage ? <RiLoader4Line className="mr-2 size-4 animate-spin" aria-hidden="true" /> : <RiSave3Line className="mr-2 size-4" aria-hidden="true" />}
            Save Usage Limit
          </Button>
        </div>
      </section>
    </div>
  );
}
