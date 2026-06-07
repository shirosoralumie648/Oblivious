import { FormEvent, useCallback, useEffect, useMemo, useReducer, useState } from 'react';
import { RiAlarmWarningLine, RiCheckLine, RiCloseLine, RiHistoryLine, RiRefreshLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { EmptyState } from '../../components/shared/EmptyState';
import {
  createAdminApi,
  type AdminObservabilityAlertDeliveryAttempt as RawAlertDeliveryAttempt,
  type AdminObservabilityAlertDeliveryChannel as AlertDeliveryChannel,
  type AdminObservabilityAlertProvider as AlertProvider,
  type AdminObservabilityAlertProviderKind as AlertProviderKind,
  type AdminObservabilityAlertProviderStatus as AlertProviderStatus,
  type AdminObservabilityAlertProviderTestResult as AlertProviderTestResult,
  type AdminObservabilityAlertRoutingRules as AlertRoutingRules,
  type AdminObservabilityAlertState as RawAlertState,
  type AdminObservabilityRecoveryAction as RawRecoveryAction,
} from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';

type AlertState = {
  id: string;
  title: string;
  severity: string;
  status: string;
  summary: string;
  component: string;
  lastOccurredAt: string;
  occurrenceCount: number;
};

type AlertDeliveryAttempt = {
  id: string;
  alertKey: string;
  channel: string;
  providerId: string;
  providerKind: string;
  delivered: boolean;
  error: string;
  attemptedAt: string;
};

type RecoveryAction = {
  id: string;
  policyName: string;
  alertKey: string;
  severity: string;
  component: string;
  type: string;
  status: string;
  reason: string;
  createdAt: string;
};

type AlertFilters = {
  severity: string;
  status: string;
  component: string;
};

type AlertProviderForm = {
  name: string;
  kind: AlertProviderKind;
  status: AlertProviderStatus;
  webhookURL: string;
  routingKey: string;
  apiKey: string;
  apiURL: string;
  accountSID: string;
  authToken: string;
  fromNumber: string;
  smsRecipients: string;
  accessKeyID: string;
  accessKeySecret: string;
  signName: string;
  templateCode: string;
  phoneProvider: string;
  phoneNumbers: string;
  smtpHost: string;
  smtpPort: string;
  username: string;
  password: string;
  fromEmail: string;
  recipients: string;
};

type State = {
  alerts: AlertState[];
  loading: boolean;
  error: string | null;
  actionError: string | null;
  actionLoading: string | null;
  deliveryAlertId: string | null;
  deliveryAlertTitle: string | null;
  deliveryAttempts: AlertDeliveryAttempt[];
  deliveryLoading: boolean;
  deliveryError: string | null;
};

type Action =
  | { type: 'LOADING' }
  | { type: 'SUCCESS'; alerts: AlertState[] }
  | { type: 'ERROR'; error: string }
  | { type: 'ACTION_START'; action: string }
  | { type: 'ACTION_SUCCESS' }
  | { type: 'ACTION_ERROR'; error: string }
  | { type: 'DELIVERY_START'; alertId: string; alertTitle: string }
  | { type: 'DELIVERY_SUCCESS'; alertId: string; alertTitle: string; attempts: AlertDeliveryAttempt[] }
  | { type: 'DELIVERY_ERROR'; alertId: string; alertTitle: string; error: string };

const initialState: State = {
  alerts: [],
  loading: true,
  error: null,
  actionError: null,
  actionLoading: null,
  deliveryAlertId: null,
  deliveryAlertTitle: null,
  deliveryAttempts: [],
  deliveryLoading: false,
  deliveryError: null,
};

const initialFilters: AlertFilters = {
  severity: '',
  status: '',
  component: '',
};

const severityOptions = ['', 'critical', 'warning', 'info', 'debug'];
const statusOptions = ['', 'open', 'acknowledged', 'resolved'];
const routingSeverities = ['debug', 'info', 'warning', 'critical'];
const routingChannels: AlertDeliveryChannel[] = ['email', 'im', 'sms', 'third_party', 'phone'];
const providerKindOptions: AlertProviderKind[] = [
  'slack_webhook',
  'smtp',
  'feishu_webhook',
  'dingtalk_webhook',
  'wecom_webhook',
  'pagerduty',
  'opsgenie',
  'twilio_sms',
  'aliyun_sms',
  'phone',
  'aliyun_monitor',
  'tencent_cloud_monitor',
];

const initialProviderForm: AlertProviderForm = {
  name: '',
  kind: 'slack_webhook',
  status: 'active',
  webhookURL: '',
  routingKey: '',
  apiKey: '',
  apiURL: '',
  accountSID: '',
  authToken: '',
  fromNumber: '',
  smsRecipients: '',
  accessKeyID: '',
  accessKeySecret: '',
  signName: '',
  templateCode: '',
  phoneProvider: 'twilio',
  phoneNumbers: '',
  smtpHost: '',
  smtpPort: '587',
  username: '',
  password: '',
  fromEmail: '',
  recipients: '',
};

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'LOADING':
      return { ...state, loading: true, error: null };
    case 'SUCCESS':
      return { ...state, alerts: action.alerts, loading: false, error: null, actionError: null, actionLoading: null };
    case 'ERROR':
      return { ...state, loading: false, error: action.error };
    case 'ACTION_START':
      return { ...state, actionError: null, actionLoading: action.action };
    case 'ACTION_SUCCESS':
      return { ...state, actionError: null, actionLoading: null };
    case 'ACTION_ERROR':
      return { ...state, actionError: action.error, actionLoading: null };
    case 'DELIVERY_START':
      return {
        ...state,
        deliveryAlertId: action.alertId,
        deliveryAlertTitle: action.alertTitle,
        deliveryAttempts: [],
        deliveryLoading: true,
        deliveryError: null,
      };
    case 'DELIVERY_SUCCESS':
      return {
        ...state,
        deliveryAlertId: action.alertId,
        deliveryAlertTitle: action.alertTitle,
        deliveryAttempts: action.attempts,
        deliveryLoading: false,
        deliveryError: null,
      };
    case 'DELIVERY_ERROR':
      return {
        ...state,
        deliveryAlertId: action.alertId,
        deliveryAlertTitle: action.alertTitle,
        deliveryAttempts: [],
        deliveryLoading: false,
        deliveryError: action.error,
      };
    default:
  return state;
  }
}

function normalizeAlert(raw: RawAlertState): AlertState {
  const id = raw.key ?? raw.Key ?? raw.id ?? 'unknown-alert';
  return {
    id,
    title: raw.name ?? raw.title ?? raw.Title ?? id,
    severity: raw.severity ?? raw.Severity ?? raw.originalSeverity ?? raw.OriginalSeverity ?? 'info',
    status: raw.status ?? raw.Status ?? 'open',
    summary: raw.summary ?? raw.message ?? raw.Message ?? '',
    component: raw.source ?? raw.component ?? raw.Component ?? 'system',
    lastOccurredAt: raw.lastTriggeredAt ?? raw.lastOccurredAt ?? raw.LastOccurredAt ?? raw.openedAt ?? raw.OpenedAt ?? '',
    occurrenceCount: raw.occurrenceCount ?? raw.OccurrenceCount ?? 1,
  };
}

function normalizeDeliveryAttempt(raw: RawAlertDeliveryAttempt): AlertDeliveryAttempt {
  const alertKey = raw.alertKey ?? raw.AlertKey ?? '';
  const channel = raw.channel ?? raw.Channel ?? 'unknown';
  const attemptedAt = raw.attemptedAt ?? raw.AttemptedAt ?? '';
  return {
    id: raw.id ?? raw.ID ?? `${alertKey}:${channel}:${attemptedAt}`,
    alertKey,
    channel,
    providerId: raw.providerId ?? raw.providerID ?? raw.ProviderID ?? '',
    providerKind: raw.providerKind ?? raw.ProviderKind ?? '',
    delivered: raw.delivered ?? raw.Delivered ?? false,
    error: raw.error ?? raw.Error ?? '',
    attemptedAt,
  };
}

function normalizeRecoveryAction(raw: RawRecoveryAction): RecoveryAction {
  const id = raw.id ?? raw.ID ?? 'unknown-recovery-action';
  const alertKey = raw.alertKey ?? raw.AlertKey ?? '';
  return {
    id,
    policyName: raw.policyName ?? raw.PolicyName ?? id,
    alertKey,
    severity: raw.severity ?? raw.Severity ?? 'info',
    component: raw.component ?? raw.Component ?? 'system',
    type: raw.type ?? raw.Type ?? 'recovery',
    status: raw.status ?? raw.Status ?? 'recorded',
    reason: raw.reason ?? raw.Reason ?? '',
    createdAt: raw.createdAt ?? raw.CreatedAt ?? '',
  };
}

function formatDate(value: string) {
  if (!value) {
    return 'Not recorded';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function badgeClass(value: string) {
  switch (value.toLowerCase()) {
    case 'critical':
    case 'open':
    case 'firing':
      return 'border-destructive/40 bg-destructive/10 text-destructive';
    case 'warning':
    case 'acknowledged':
      return 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300';
    case 'resolved':
      return 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300';
    default:
  return 'border-border bg-muted text-muted-foreground';
  }
}

function optionLabel(value: string, fallback: string) {
  if (!value) {
    return fallback;
  }
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function channelLabel(value: string) {
  return value.replace(/_/g, ' ');
}

function channelDisplayLabel(value: string) {
  switch (value) {
    case 'im':
      return 'IM';
    case 'sms':
      return 'SMS';
    default:
      return optionLabel(channelLabel(value), value);
  }
}

function normalizeRoutingRules(rules: AlertRoutingRules | null | undefined): AlertRoutingRules {
  const normalized: AlertRoutingRules = {};
  for (const severity of routingSeverities) {
    normalized[severity] = Array.isArray(rules?.[severity]) ? [...rules[severity]] : [];
  }
  return normalized;
}

function routingSummary(channels: AlertDeliveryChannel[]) {
  if (channels.length === 0) {
    return 'log only';
  }
  return channels.map(channelLabel).join(' + ');
}

function providerKindLabel(kind: string) {
  return kind.replace(/_/g, ' ');
}

function deliveryProviderLabel(attempt: AlertDeliveryAttempt) {
  if (!attempt.providerId && !attempt.providerKind) {
    return 'System sink';
  }
  if (!attempt.providerKind) {
    return attempt.providerId;
  }
  if (!attempt.providerId) {
    return attempt.providerKind;
  }
  return `${attempt.providerId} ${attempt.providerKind}`;
}

function isWebhookProvider(kind: AlertProviderKind) {
  return kind.endsWith('_webhook') || kind === 'aliyun_monitor' || kind === 'tencent_cloud_monitor';
}

function isPagerDutyProvider(kind: AlertProviderKind) {
  return kind === 'pagerduty';
}

function isOpsgenieProvider(kind: AlertProviderKind) {
  return kind === 'opsgenie';
}

function isProviderAPIURLProvider(kind: AlertProviderKind) {
  return isPagerDutyProvider(kind) || isOpsgenieProvider(kind) || kind === 'twilio_sms' || kind === 'aliyun_sms' || kind === 'phone';
}

function isTwilioCredentialProvider(kind: AlertProviderKind) {
  return kind === 'twilio_sms' || kind === 'phone';
}

function isAliyunSMSProvider(kind: AlertProviderKind) {
  return kind === 'aliyun_sms';
}

function providerConfigEntries(provider: AlertProvider) {
  return Object.entries(provider.config ?? {}).filter(([, value]) => value !== undefined && value !== null && value !== '');
}

function buildProviderConfig(form: AlertProviderForm) {
  if (form.kind === 'smtp') {
    return {
      smtp_host: form.smtpHost.trim(),
      smtp_port: form.smtpPort.trim(),
      username: form.username.trim(),
      password: form.password,
      from_email: form.fromEmail.trim(),
      recipients: form.recipients.trim(),
    };
  }
  if (form.kind === 'pagerduty') {
    return {
      routing_key: form.routingKey.trim(),
      api_url: form.apiURL.trim(),
    };
  }
  if (form.kind === 'opsgenie') {
    return {
      api_key: form.apiKey.trim(),
      api_url: form.apiURL.trim(),
    };
  }
  if (form.kind === 'twilio_sms') {
    return {
      account_sid: form.accountSID.trim(),
      auth_token: form.authToken,
      from_number: form.fromNumber.trim(),
      recipients: form.smsRecipients.trim(),
      api_url: form.apiURL.trim(),
    };
  }
  if (form.kind === 'aliyun_sms') {
    return {
      access_key_id: form.accessKeyID.trim(),
      access_key_secret: form.accessKeySecret,
      sign_name: form.signName.trim(),
      template_code: form.templateCode.trim(),
      recipients: form.smsRecipients.trim(),
      api_url: form.apiURL.trim(),
    };
  }
  if (form.kind === 'phone') {
    return {
      provider: form.phoneProvider.trim(),
      account_sid: form.accountSID.trim(),
      auth_token: form.authToken,
      from_number: form.fromNumber.trim(),
      phone_numbers: form.phoneNumbers.trim(),
      api_url: form.apiURL.trim(),
    };
  }

  return {
    webhook_url: form.webhookURL.trim(),
  };
}

export function AdminAlertsPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const [filters, setFilters] = useState<AlertFilters>(initialFilters);
  const [draftFilters, setDraftFilters] = useState<AlertFilters>(initialFilters);
  const [recoveryActions, setRecoveryActions] = useState<RecoveryAction[]>([]);
  const [recoveryLoading, setRecoveryLoading] = useState(false);
  const [recoveryError, setRecoveryError] = useState<string | null>(null);
  const [routingRules, setRoutingRules] = useState<AlertRoutingRules | null>(null);
  const [routingDraft, setRoutingDraft] = useState<AlertRoutingRules>(normalizeRoutingRules(null));
  const [routingError, setRoutingError] = useState<string | null>(null);
  const [routingSaving, setRoutingSaving] = useState(false);
  const [alertProviders, setAlertProviders] = useState<AlertProvider[]>([]);
  const [providerLoading, setProviderLoading] = useState(false);
  const [providerError, setProviderError] = useState<string | null>(null);
  const [providerSaving, setProviderSaving] = useState(false);
  const [providerTesting, setProviderTesting] = useState<string | null>(null);
  const [providerForm, setProviderForm] = useState<AlertProviderForm>(initialProviderForm);
  const [providerTestResult, setProviderTestResult] = useState<{
    providerName: string;
    result: AlertProviderTestResult;
  } | null>(null);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadAlerts = useCallback(async () => {
    dispatch({ type: 'LOADING' });
    try {
      const alerts = await api.listObservabilityAlerts(filters);
      dispatch({ type: 'SUCCESS', alerts: alerts.map(normalizeAlert) });
    } catch (error) {
      dispatch({
        type: 'ERROR',
        error: error instanceof Error ? error.message : 'Unable to load alerts.',
      });
    }
  }, [api, filters]);

  const loadRecoveryActions = useCallback(async () => {
    setRecoveryLoading(true);
    try {
      const actions = await api.listObservabilityRecoveryActions();
      setRecoveryActions(actions.map(normalizeRecoveryAction));
      setRecoveryError(null);
    } catch (error) {
      setRecoveryError(error instanceof Error ? error.message : 'Unable to load recovery actions.');
    } finally {
      setRecoveryLoading(false);
    }
  }, [api]);

  const loadRoutingRules = useCallback(async () => {
    try {
      const payload = await api.getObservabilityAlertRoutingRules();
      const normalized = normalizeRoutingRules(payload);
      setRoutingRules(normalized);
      setRoutingDraft(normalized);
      setRoutingError(null);
    } catch (error) {
      setRoutingError(error instanceof Error ? error.message : 'Unable to load notification routing.');
    }
  }, [api]);

  const loadAlertProviders = useCallback(async () => {
    setProviderLoading(true);
    try {
      const providers = await api.listObservabilityAlertProviders();
      setAlertProviders(providers);
      setProviderError(null);
    } catch (error) {
      setProviderError(error instanceof Error ? error.message : 'Unable to load alert providers.');
    } finally {
      setProviderLoading(false);
    }
  }, [api]);

  const toggleRoutingChannel = useCallback((severity: string, channel: AlertDeliveryChannel) => {
    setRoutingDraft((current) => {
      const next = normalizeRoutingRules(current);
      const channels = next[severity] ?? [];
      next[severity] = channels.includes(channel) ? channels.filter((item) => item !== channel) : [...channels, channel];
      return next;
    });
  }, []);

  const saveRoutingRules = useCallback(async () => {
    setRoutingSaving(true);
    setRoutingError(null);
    try {
      const payload = await api.updateObservabilityAlertRoutingRules(routingDraft);
      const normalized = normalizeRoutingRules(payload);
      setRoutingRules(normalized);
      setRoutingDraft(normalized);
    } catch (error) {
      setRoutingError(error instanceof Error ? error.message : 'Unable to save notification routing.');
    } finally {
      setRoutingSaving(false);
    }
  }, [api, routingDraft]);

  const createAlertProvider = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setProviderSaving(true);
      setProviderError(null);
      try {
        const provider = await api.createObservabilityAlertProvider({
          kind: providerForm.kind,
          name: providerForm.name.trim(),
          status: providerForm.status,
          config: buildProviderConfig(providerForm),
        });
        setAlertProviders((current) => [...current, provider]);
        setProviderForm(initialProviderForm);
      } catch (error) {
        setProviderError(error instanceof Error ? error.message : 'Unable to create alert provider.');
      } finally {
        setProviderSaving(false);
      }
    },
    [api, providerForm]
  );

  const testAlertProvider = useCallback(
    async (provider: AlertProvider) => {
      setProviderTesting(provider.id);
      setProviderError(null);
      try {
        const result = await api.testObservabilityAlertProvider(provider.id);
        setProviderTestResult({ providerName: provider.name, result });
      } catch (error) {
        setProviderError(error instanceof Error ? error.message : 'Unable to test alert provider.');
      } finally {
        setProviderTesting(null);
      }
    },
    [api]
  );

  const updateAlert = useCallback(
    async (alert: AlertState, action: 'acknowledge' | 'resolve') => {
      const actionKey = `${alert.id}:${action}`;
      dispatch({ type: 'ACTION_START', action: actionKey });
      try {
        if (action === 'acknowledge') {
          await api.acknowledgeObservabilityAlert(alert.id);
        } else {
          await api.resolveObservabilityAlert(alert.id);
        }
        dispatch({ type: 'ACTION_SUCCESS' });
        await loadAlerts();
      } catch (error) {
        dispatch({
          type: 'ACTION_ERROR',
          error: error instanceof Error ? error.message : `Unable to ${action} alert.`,
        });
      }
    },
    [api, loadAlerts]
  );

  const loadDeliveries = useCallback(
    async (alert: AlertState) => {
      dispatch({ type: 'DELIVERY_START', alertId: alert.id, alertTitle: alert.title });
      try {
        const attempts = await api.listObservabilityAlertDeliveries(alert.id);
        dispatch({
          type: 'DELIVERY_SUCCESS',
          alertId: alert.id,
          alertTitle: alert.title,
          attempts: attempts.map(normalizeDeliveryAttempt),
        });
      } catch (error) {
        dispatch({
          type: 'DELIVERY_ERROR',
          alertId: alert.id,
          alertTitle: alert.title,
          error: error instanceof Error ? error.message : 'Unable to load delivery history.',
        });
      }
    },
    [api]
  );

  useEffect(() => {
    void loadAlerts();
  }, [loadAlerts]);

  useEffect(() => {
    void loadRoutingRules();
  }, [loadRoutingRules]);

  useEffect(() => {
    void loadAlertProviders();
  }, [loadAlertProviders]);

  useEffect(() => {
    void loadRecoveryActions();
  }, [loadRecoveryActions]);

  const applyFilters = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setFilters({
        severity: draftFilters.severity,
        status: draftFilters.status,
        component: draftFilters.component.trim(),
      });
    },
    [draftFilters]
  );

  const clearFilters = useCallback(() => {
    setDraftFilters(initialFilters);
    setFilters(initialFilters);
  }, []);

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">Alerts</h1>
          <p className="mt-1 text-sm text-muted-foreground">Observability signals requiring administrator review.</p>
        </div>
        <Button type="button" variant="outline" onClick={loadAlerts} disabled={state.loading}>
          <RiRefreshLine className="mr-2 size-4" aria-hidden="true" />
          Refresh
        </Button>
      </header>

      <form onSubmit={applyFilters} className="grid gap-3 rounded-lg border bg-card p-4 md:grid-cols-[160px_170px_minmax(180px,1fr)_auto_auto]" aria-label="Alert filters">
        <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Severity
          <select
            className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none focus:border-ring focus:ring-[3px] focus:ring-ring/30"
            value={draftFilters.severity}
            onChange={(event) => setDraftFilters((current) => ({ ...current, severity: event.target.value }))}
          >
            {severityOptions.map((option) => (
              <option key={option || 'all'} value={option}>
                {optionLabel(option, 'All severities')}
              </option>
            ))}
          </select>
        </label>
        <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Status
          <select
            className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none focus:border-ring focus:ring-[3px] focus:ring-ring/30"
            value={draftFilters.status}
            onChange={(event) => setDraftFilters((current) => ({ ...current, status: event.target.value }))}
          >
            {statusOptions.map((option) => (
              <option key={option || 'all'} value={option}>
                {optionLabel(option, 'All statuses')}
              </option>
            ))}
          </select>
        </label>
        <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Component
          <input
            className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
            value={draftFilters.component}
            placeholder="relay, workflow, channel"
            onChange={(event) => setDraftFilters((current) => ({ ...current, component: event.target.value }))}
          />
        </label>
        <Button type="submit" variant="outline" className="self-end" aria-label="Apply alert filters" disabled={state.loading}>
          Apply
        </Button>
        <Button type="button" variant="ghost" className="self-end" onClick={clearFilters} disabled={state.loading}>
          Clear
        </Button>
      </form>

      <section className="rounded-lg border bg-card p-4" aria-label="Notification routing">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold text-foreground">Notification routing</h2>
            <p className="mt-1 text-xs text-muted-foreground">Severity-based delivery channels for alert escalation.</p>
          </div>
          <Button type="button" variant="outline" onClick={() => void saveRoutingRules()} disabled={routingSaving || routingRules === null}>
            Save notification routing
          </Button>
        </div>
        {routingError ? (
          <div role="alert" className="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {routingError}
          </div>
        ) : null}
        <div className="mt-4 grid gap-3">
          {routingSeverities.map((severity) => (
            <div key={severity} className="grid gap-3 rounded-md border bg-background/50 p-3 lg:grid-cols-[110px_minmax(0,1fr)_minmax(180px,260px)] lg:items-center">
              <span className={`w-fit rounded-full border px-2.5 py-1 text-xs font-medium ${badgeClass(severity)}`}>{optionLabel(severity, severity)} alerts</span>
              <div className="flex flex-wrap gap-2">
                {routingChannels.map((channel) => {
                  const checked = routingDraft[severity]?.includes(channel) ?? false;
                  return (
                    <label key={`${severity}:${channel}`} className="flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs text-muted-foreground">
                      <input
                        type="checkbox"
                        aria-label={`Route ${severity} alerts to ${channelLabel(channel)}`}
                        checked={checked}
                        onChange={() => toggleRoutingChannel(severity, channel)}
                        disabled={routingRules === null || routingSaving}
                      />
                      {channelDisplayLabel(channel)}
                    </label>
                  );
                })}
              </div>
              <span className="text-sm text-muted-foreground">{routingSummary(routingDraft[severity] ?? [])}</span>
            </div>
          ))}
        </div>
      </section>

      <section className="overflow-hidden rounded-lg border bg-card" aria-label="Alert notification providers">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
          <div>
            <h2 className="text-sm font-semibold text-foreground">Alert notification providers</h2>
            <p className="mt-1 text-xs text-muted-foreground">Outbound provider configs used by alert routing channels.</p>
          </div>
          {providerLoading ? <span className="text-xs text-muted-foreground">Loading...</span> : null}
        </div>

        {providerError ? (
          <div role="alert" className="border-b border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {providerError}
          </div>
        ) : null}

        {providerTestResult ? (
          <div className="border-b border-emerald-500/30 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-700 dark:text-emerald-300">
            {providerTestResult.providerName}: {providerTestResult.result.message}
          </div>
        ) : null}

        <form onSubmit={createAlertProvider} className="grid gap-3 border-b px-4 py-4 lg:grid-cols-[minmax(160px,1fr)_180px_minmax(220px,1.4fr)_auto] lg:items-end">
          <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
            Provider name
            <input
              className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
              value={providerForm.name}
              placeholder="Slack Ops"
              onChange={(event) => setProviderForm((current) => ({ ...current, name: event.target.value }))}
            />
          </label>
          <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
            Provider kind
            <select
              className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none focus:border-ring focus:ring-[3px] focus:ring-ring/30"
              value={providerForm.kind}
              onChange={(event) => setProviderForm((current) => ({ ...current, kind: event.target.value as AlertProviderKind }))}
            >
              {providerKindOptions.map((kind) => (
                <option key={kind} value={kind}>
                  {providerKindLabel(kind)}
                </option>
              ))}
            </select>
          </label>
          {providerForm.kind === 'smtp' ? (
            <div className="grid gap-2 md:grid-cols-[minmax(160px,1fr)_90px_minmax(140px,1fr)_minmax(120px,1fr)] xl:grid-cols-[minmax(150px,1fr)_80px_minmax(130px,1fr)_minmax(120px,1fr)_minmax(150px,1fr)_minmax(180px,1.2fr)]">
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                SMTP host
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.smtpHost}
                  placeholder="smtp.example.com"
                  onChange={(event) => setProviderForm((current) => ({ ...current, smtpHost: event.target.value }))}
                />
              </label>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                SMTP port
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.smtpPort}
                  onChange={(event) => setProviderForm((current) => ({ ...current, smtpPort: event.target.value }))}
                />
              </label>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                SMTP username
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.username}
                  onChange={(event) => setProviderForm((current) => ({ ...current, username: event.target.value }))}
                />
              </label>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                SMTP password
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.password}
                  type="password"
                  onChange={(event) => setProviderForm((current) => ({ ...current, password: event.target.value }))}
                />
              </label>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                From email
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.fromEmail}
                  placeholder="alerts@example.com"
                  onChange={(event) => setProviderForm((current) => ({ ...current, fromEmail: event.target.value }))}
                />
              </label>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Recipients
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.recipients}
                  placeholder="ops@example.com,oncall@example.com"
                  onChange={(event) => setProviderForm((current) => ({ ...current, recipients: event.target.value }))}
                />
              </label>
            </div>
          ) : null}
          {isPagerDutyProvider(providerForm.kind) ? (
            <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Routing key
              <input
                className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                value={providerForm.routingKey}
                type="password"
                placeholder="PagerDuty Events API key"
                onChange={(event) => setProviderForm((current) => ({ ...current, routingKey: event.target.value }))}
              />
            </label>
          ) : null}
          {isOpsgenieProvider(providerForm.kind) ? (
            <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              API key
              <input
                className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                value={providerForm.apiKey}
                type="password"
                placeholder="Opsgenie API key"
                onChange={(event) => setProviderForm((current) => ({ ...current, apiKey: event.target.value }))}
              />
            </label>
          ) : null}
          {providerForm.kind === 'phone' ? (
            <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Phone provider
              <select
                className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                value={providerForm.phoneProvider}
                onChange={(event) => setProviderForm((current) => ({ ...current, phoneProvider: event.target.value }))}
              >
                <option value="twilio">Twilio</option>
              </select>
            </label>
          ) : null}
          {isTwilioCredentialProvider(providerForm.kind) ? (
            <>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Account SID
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.accountSID}
                  placeholder="AC..."
                  onChange={(event) => setProviderForm((current) => ({ ...current, accountSID: event.target.value }))}
                />
              </label>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Auth token
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.authToken}
                  type="password"
                  onChange={(event) => setProviderForm((current) => ({ ...current, authToken: event.target.value }))}
                />
              </label>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                From number
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.fromNumber}
                  placeholder="+15550000000"
                  onChange={(event) => setProviderForm((current) => ({ ...current, fromNumber: event.target.value }))}
                />
              </label>
            </>
          ) : null}
          {isAliyunSMSProvider(providerForm.kind) ? (
            <>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Access key ID
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.accessKeyID}
                  onChange={(event) => setProviderForm((current) => ({ ...current, accessKeyID: event.target.value }))}
                />
              </label>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Access key secret
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.accessKeySecret}
                  type="password"
                  onChange={(event) => setProviderForm((current) => ({ ...current, accessKeySecret: event.target.value }))}
                />
              </label>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Sign name
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.signName}
                  placeholder="Oblivious"
                  onChange={(event) => setProviderForm((current) => ({ ...current, signName: event.target.value }))}
                />
              </label>
              <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Template code
                <input
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                  value={providerForm.templateCode}
                  placeholder="SMS_123"
                  onChange={(event) => setProviderForm((current) => ({ ...current, templateCode: event.target.value }))}
                />
              </label>
            </>
          ) : null}
          {providerForm.kind === 'twilio_sms' || providerForm.kind === 'aliyun_sms' ? (
            <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              SMS recipients
              <input
                className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                value={providerForm.smsRecipients}
                placeholder="+15550000001,+15550000002"
                onChange={(event) => setProviderForm((current) => ({ ...current, smsRecipients: event.target.value }))}
              />
            </label>
          ) : null}
          {providerForm.kind === 'phone' ? (
            <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Phone numbers
              <input
                className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                value={providerForm.phoneNumbers}
                placeholder="+15550000003"
                onChange={(event) => setProviderForm((current) => ({ ...current, phoneNumbers: event.target.value }))}
              />
            </label>
          ) : null}
          {isProviderAPIURLProvider(providerForm.kind) ? (
            <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Provider API URL
              <input
                className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                value={providerForm.apiURL}
                placeholder="https://api.example/alerts"
                onChange={(event) => setProviderForm((current) => ({ ...current, apiURL: event.target.value }))}
              />
            </label>
          ) : null}
          {isWebhookProvider(providerForm.kind) ? (
            <label className="grid gap-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Provider webhook URL
              <input
                className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal normal-case tracking-normal text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30"
                value={providerForm.webhookURL}
                placeholder="https://hooks.example/ops"
                onChange={(event) => setProviderForm((current) => ({ ...current, webhookURL: event.target.value }))}
              />
            </label>
          ) : null}
          <Button type="submit" variant="outline" className="self-end" disabled={providerSaving || providerForm.name.trim() === ''}>
            Create alert provider
          </Button>
        </form>

        {!providerLoading && !providerError && alertProviders.length === 0 ? (
          <p className="px-4 py-4 text-sm text-muted-foreground">No alert notification providers configured.</p>
        ) : null}

        {alertProviders.length > 0 ? (
          <div className="divide-y">
            <div className="grid gap-3 px-4 py-3 text-xs font-medium uppercase tracking-wide text-muted-foreground md:grid-cols-[minmax(140px,1fr)_150px_90px_100px_minmax(180px,1.4fr)_150px]">
              <span>Name</span>
              <span>Kind</span>
              <span>Channel</span>
              <span>Status</span>
              <span>Config</span>
              <span>Actions</span>
            </div>
            {alertProviders.map((provider) => (
              <div
                key={provider.id}
                className="grid gap-3 px-4 py-3 text-sm md:grid-cols-[minmax(140px,1fr)_150px_90px_100px_minmax(180px,1.4fr)_150px] md:items-center"
              >
                <div className="min-w-0">
                  <span className="block truncate font-medium text-foreground">{provider.name}</span>
                  <span className="block truncate text-xs text-muted-foreground">{provider.id}</span>
                </div>
                <span className="text-muted-foreground">{providerKindLabel(provider.kind)}</span>
                <span className="text-muted-foreground">{provider.channel}</span>
                <span className={`w-fit rounded-full border px-2.5 py-1 text-xs font-medium ${badgeClass(provider.status)}`}>{provider.status}</span>
                <div className="min-w-0 space-y-1 text-xs text-muted-foreground">
                  {providerConfigEntries(provider).length === 0 ? (
                    <span>No config values</span>
                  ) : (
                    providerConfigEntries(provider).map(([key, value]) => (
                      <div key={`${provider.id}:${key}`} className="grid min-w-0 gap-1 sm:grid-cols-[110px_minmax(0,1fr)]">
                        <span className="truncate font-medium">{key}</span>
                        <span className="truncate text-foreground">{String(value)}</span>
                      </div>
                    ))
                  )}
                </div>
                <Button
                  type="button"
                  size="xs"
                  variant="outline"
                  aria-label={`Test provider ${provider.name}`}
                  onClick={() => void testAlertProvider(provider)}
                  disabled={providerTesting !== null}
                >
                  Test provider
                </Button>
              </div>
            ))}
          </div>
        ) : null}
      </section>

      <section className="overflow-hidden rounded-lg border bg-card" aria-label="Recovery actions">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
          <div>
            <h2 className="text-sm font-semibold text-foreground">Recovery actions</h2>
            <p className="mt-1 text-xs text-muted-foreground">Recorded automatic recovery decisions for incidents.</p>
          </div>
          {recoveryLoading ? <span className="text-xs text-muted-foreground">Loading...</span> : null}
        </div>

        {recoveryError ? (
          <div role="alert" className="border-b border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {recoveryError}
          </div>
        ) : null}

        {!recoveryLoading && !recoveryError && recoveryActions.length === 0 ? (
          <p className="px-4 py-4 text-sm text-muted-foreground">No recovery actions recorded yet.</p>
        ) : null}

        {recoveryActions.length > 0 ? (
          <div className="divide-y">
            <div className="grid gap-3 px-4 py-3 text-xs font-medium uppercase tracking-wide text-muted-foreground md:grid-cols-[minmax(140px,1fr)_minmax(140px,1fr)_100px_110px_120px_170px]">
              <span>Policy</span>
              <span>Alert</span>
              <span>Action</span>
              <span>Status</span>
              <span>Component</span>
              <span>Recorded</span>
            </div>
            {recoveryActions.map((action) => (
              <div
                key={action.id}
                className="grid gap-3 px-4 py-3 text-sm md:grid-cols-[minmax(140px,1fr)_minmax(140px,1fr)_100px_110px_120px_170px] md:items-center"
              >
                <span className="min-w-0 font-medium text-foreground">{action.policyName}</span>
                <div className="min-w-0">
                  <span className="block truncate text-foreground">{action.alertKey || 'unknown alert'}</span>
                  {action.reason ? <span className="block truncate text-xs text-muted-foreground">{action.reason}</span> : null}
                </div>
                <span className={`w-fit rounded-full border px-2.5 py-1 text-xs font-medium ${badgeClass(action.severity)}`}>{action.type}</span>
                <span className={`w-fit rounded-full border px-2.5 py-1 text-xs font-medium ${badgeClass(action.status)}`}>{action.status}</span>
                <span className="text-muted-foreground">{action.component}</span>
                <span className="text-muted-foreground">{formatDate(action.createdAt)}</span>
              </div>
            ))}
          </div>
        ) : null}
      </section>

      {state.loading ? (
        <div aria-busy="true" className="rounded-lg border bg-card p-6 text-sm text-muted-foreground">
          Loading alerts...
        </div>
      ) : null}

      {!state.loading && state.error ? (
        <EmptyState
          icon={<RiAlarmWarningLine className="size-7" aria-hidden="true" />}
          title="Unable to load alerts."
          description={state.error}
          action={{ label: 'Try Again', onClick: loadAlerts }}
        />
      ) : null}

      {!state.loading && !state.error && state.alerts.length === 0 ? (
        <EmptyState
          icon={<RiAlarmWarningLine className="size-7" aria-hidden="true" />}
          title="No alerts to review."
          description="Open, acknowledged, and resolved alert state will appear here once the observability pipeline records it."
          action={{ label: 'Refresh', onClick: loadAlerts }}
        />
      ) : null}

      {!state.loading && !state.error && state.alerts.length > 0 ? (
        <>
          <section className="overflow-hidden rounded-lg border bg-card" aria-label="Alert list">
            {state.actionError ? (
              <div role="alert" className="border-b border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                {state.actionError}
              </div>
            ) : null}
            <div className="grid gap-3 border-b px-4 py-3 text-xs font-medium uppercase tracking-wide text-muted-foreground md:grid-cols-[minmax(0,1fr)_100px_120px_120px_150px_260px]">
              <span>Alert</span>
              <span>Severity</span>
              <span>Status</span>
              <span>Component</span>
              <span>Last seen</span>
              <span>Actions</span>
            </div>
            <div className="divide-y">
              {state.alerts.map((alert) => (
                <article key={alert.id} className="grid gap-3 px-4 py-4 md:grid-cols-[minmax(0,1fr)_100px_120px_120px_150px_260px] md:items-center">
                  <div className="min-w-0 space-y-1">
                    <h2 className="truncate text-sm font-semibold text-foreground">{alert.title}</h2>
                    {alert.summary ? <p className="text-sm text-muted-foreground">{alert.summary}</p> : null}
                    <p className="text-xs text-muted-foreground">
                      {alert.id} · {alert.occurrenceCount} occurrence{alert.occurrenceCount === 1 ? '' : 's'}
                    </p>
                  </div>
                  <span className={`w-fit rounded-full border px-2.5 py-1 text-xs font-medium ${badgeClass(alert.severity)}`}>{alert.severity}</span>
                  <span className={`w-fit rounded-full border px-2.5 py-1 text-xs font-medium ${badgeClass(alert.status)}`}>{alert.status}</span>
                  <span className="text-sm text-muted-foreground">{alert.component}</span>
                  <span className="text-sm text-muted-foreground">{formatDate(alert.lastOccurredAt)}</span>
                  <div className="flex flex-wrap gap-2">
                    <Button
                      type="button"
                      size="xs"
                      variant="outline"
                      aria-label={`Acknowledge ${alert.title}`}
                      onClick={() => void updateAlert(alert, 'acknowledge')}
                      disabled={state.actionLoading !== null || alert.status.toLowerCase() === 'acknowledged' || alert.status.toLowerCase() === 'resolved'}
                    >
                      <RiCheckLine className="size-3" aria-hidden="true" />
                      Acknowledge
                    </Button>
                    <Button
                      type="button"
                      size="xs"
                      variant="outline"
                      aria-label={`Resolve ${alert.title}`}
                      onClick={() => void updateAlert(alert, 'resolve')}
                      disabled={state.actionLoading !== null || alert.status.toLowerCase() === 'resolved'}
                    >
                      <RiCloseLine className="size-3" aria-hidden="true" />
                      Resolve
                    </Button>
                    <Button
                      type="button"
                      size="xs"
                      variant="outline"
                      aria-label={`View deliveries ${alert.title}`}
                      onClick={() => void loadDeliveries(alert)}
                      disabled={state.deliveryLoading && state.deliveryAlertId === alert.id}
                    >
                      <RiHistoryLine className="size-3" aria-hidden="true" />
                      Deliveries
                    </Button>
                  </div>
                </article>
              ))}
            </div>
          </section>

          {state.deliveryAlertId ? (
            <section className="overflow-hidden rounded-lg border bg-card" aria-label="Notification delivery history">
              <div className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
                <div>
                  <h2 className="text-sm font-semibold text-foreground">Notification delivery history</h2>
                  <p className="mt-1 text-xs text-muted-foreground">{state.deliveryAlertTitle ?? state.deliveryAlertId}</p>
                </div>
                {state.deliveryLoading ? <span className="text-xs text-muted-foreground">Loading...</span> : null}
              </div>

              {state.deliveryError ? (
                <div role="alert" className="border-b border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                  {state.deliveryError}
                </div>
              ) : null}

              {!state.deliveryLoading && !state.deliveryError && state.deliveryAttempts.length === 0 ? (
                <p className="px-4 py-4 text-sm text-muted-foreground">No delivery attempts recorded for this alert.</p>
              ) : null}

              {state.deliveryAttempts.length > 0 ? (
                <div className="divide-y">
                  <div className="grid gap-3 px-4 py-3 text-xs font-medium uppercase tracking-wide text-muted-foreground md:grid-cols-[100px_minmax(160px,1fr)_100px_minmax(0,1fr)_170px]">
                    <span>Channel</span>
                    <span>Provider</span>
                    <span>Status</span>
                    <span>Error</span>
                    <span>Attempted</span>
                  </div>
                  {state.deliveryAttempts.map((attempt, index) => (
                    <div
                      key={attempt.id || `${attempt.alertKey}:${attempt.channel}:${index}`}
                      className="grid gap-3 px-4 py-3 text-sm md:grid-cols-[100px_minmax(160px,1fr)_100px_minmax(0,1fr)_170px] md:items-center"
                    >
                      <span className="font-medium text-foreground">{attempt.channel}</span>
                      <span className="min-w-0 text-muted-foreground">
                        {attempt.providerId || attempt.providerKind ? (
                          <>
                            {attempt.providerId ? <span className="block truncate">{attempt.providerId}</span> : null}
                            {attempt.providerKind ? <span className="block truncate text-xs">{attempt.providerKind}</span> : null}
                          </>
                        ) : (
                          deliveryProviderLabel(attempt)
                        )}
                      </span>
                      <span className={`w-fit rounded-full border px-2.5 py-1 text-xs font-medium ${badgeClass(attempt.delivered ? 'resolved' : 'open')}`}>
                        {attempt.delivered ? 'delivered' : 'failed'}
                      </span>
                      <span className="min-w-0 text-muted-foreground">{attempt.error || 'None'}</span>
                      <span className="text-muted-foreground">{formatDate(attempt.attemptedAt)}</span>
                    </div>
                  ))}
                </div>
              ) : null}
            </section>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
