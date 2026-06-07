import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AdminAlertsPage } from './AdminAlertsPage';

const fetchMock = vi.fn();
const alertRoutingPath = '/api/v1/admin/observability/alert-routing';
const alertProvidersPath = '/api/v1/admin/observability/alert-providers';
const alertsPath = '/api/v1/admin/observability/alerts';
const recoveryActionsPath = '/api/v1/admin/observability/recovery-actions';
const defaultRoutingRules = {
  debug: [],
  info: ['email'],
  warning: ['email', 'im'],
  critical: ['email', 'im', 'sms', 'third_party'],
};

function okResponse(data: unknown) {
  return {
    ok: true,
    status: 200,
    json: () => Promise.resolve({ ok: true, data, error: null }),
  };
}

type FetchRoute = {
  path: string;
  method?: string;
  response: {
    ok: boolean;
    status: number;
    statusText?: string;
    json: () => Promise<unknown>;
  };
};

function errorResponse(status: number, message: string) {
  return {
    ok: false,
    status,
    statusText: message,
    json: () => Promise.resolve({ error: { message } }),
  };
}

function mockFetchRoutes(routes: FetchRoute[]) {
  const pending = [...routes];
  fetchMock.mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    const method = init?.method ?? 'GET';
    const index = pending.findIndex((route) => route.path === path && (route.method ?? 'GET') === method);
    if (index >= 0) {
      const [route] = pending.splice(index, 1);
      return Promise.resolve(route.response);
    }
    if (path === alertRoutingPath && method === 'GET') {
      return Promise.resolve(okResponse(defaultRoutingRules));
    }
    if (path === recoveryActionsPath && method === 'GET') {
      return Promise.resolve(okResponse([]));
    }
    if (path === alertProvidersPath && method === 'GET') {
      return Promise.resolve(okResponse([]));
    }
    throw new Error(`Unhandled fetch ${method} ${path}`);
  });
}

function mockAlertsResponse(alerts: unknown[]) {
  mockFetchRoutes([{ path: alertsPath, response: okResponse(alerts) }]);
}

describe('AdminAlertsPage', () => {
  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
  });

  it('loads and updates alert notification routing rules', async () => {
    mockFetchRoutes([
      { path: alertsPath, response: okResponse([]) },
      { path: alertRoutingPath, response: okResponse(defaultRoutingRules) },
      {
        path: alertRoutingPath,
        method: 'PUT',
        response: okResponse({
          debug: [],
          info: ['email'],
          warning: ['email', 'im', 'sms'],
          critical: ['email', 'im', 'sms', 'third_party'],
        }),
      },
    ]);

    render(<AdminAlertsPage />);

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/observability/alert-routing', expect.objectContaining({ method: 'GET' }))
    );
    expect(await screen.findByText('Notification routing')).toBeInTheDocument();
    expect(screen.getByText('Warning alerts')).toBeInTheDocument();
    expect(screen.getByText('email + im')).toBeInTheDocument();
    expect(screen.getByText('Critical alerts')).toBeInTheDocument();
    expect(screen.getByText('email + im + sms + third party')).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Route warning alerts to sms'));
    fireEvent.click(screen.getByRole('button', { name: 'Save notification routing' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/admin/observability/alert-routing',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({
            rules: {
              debug: [],
              info: ['email'],
              warning: ['email', 'im', 'sms'],
              critical: ['email', 'im', 'sms', 'third_party'],
            },
          }),
        })
      )
    );
    expect(await screen.findByText('email + im + sms')).toBeInTheDocument();
  });

  it('manages alert notification providers with redacted secrets and test actions', async () => {
    mockFetchRoutes([
      { path: alertsPath, response: okResponse([]) },
      {
        path: alertProvidersPath,
        response: okResponse([
          {
            id: 'alert_provider_smtp',
            kind: 'smtp',
            channel: 'email',
            name: 'Primary SMTP',
            status: 'active',
            config: {
              smtp_host: 'smtp.example.com',
              smtp_port: '587',
              username: 'alerts@example.com',
              password: '********',
              from_email: 'alerts@example.com',
              recipients: 'ops@example.com,oncall@example.com',
            },
          },
        ]),
      },
      {
        path: alertProvidersPath,
        method: 'POST',
        response: okResponse({
          id: 'alert_provider_slack',
          kind: 'slack_webhook',
          channel: 'im',
          name: 'Slack Ops',
          status: 'active',
          config: {
            webhook_url: '********',
          },
        }),
      },
      {
        path: '/api/v1/admin/observability/alert-providers/alert_provider_smtp/test',
        method: 'POST',
        response: okResponse({
          providerId: 'alert_provider_smtp',
          kind: 'smtp',
          channel: 'email',
          ok: true,
          message: 'provider configuration validated',
          testedAt: '2026-06-07T08:00:00Z',
        }),
      },
    ]);

    render(<AdminAlertsPage />);

    const providerPanel = await screen.findByLabelText('Alert notification providers');
    expect(within(providerPanel).getByText('Primary SMTP')).toBeInTheDocument();
    expect(within(providerPanel).getByText('smtp.example.com')).toBeInTheDocument();
    expect(within(providerPanel).getByText('********')).toBeInTheDocument();
    expect(within(providerPanel).getByText('email')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Provider name'), { target: { value: 'Slack Ops' } });
    fireEvent.change(screen.getByLabelText('Provider kind'), { target: { value: 'slack_webhook' } });
    fireEvent.change(screen.getByLabelText('Provider webhook URL'), { target: { value: 'https://hooks.slack.example/ops' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create alert provider' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        alertProvidersPath,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            kind: 'slack_webhook',
            name: 'Slack Ops',
            status: 'active',
            config: {
              webhook_url: 'https://hooks.slack.example/ops',
            },
          }),
        })
      )
    );
    expect(await within(providerPanel).findByText('Slack Ops')).toBeInTheDocument();
    expect(within(providerPanel).getByText('im')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Test provider Primary SMTP' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/admin/observability/alert-providers/alert_provider_smtp/test',
        expect.objectContaining({ method: 'POST' })
      )
    );
    expect(await screen.findByText('Primary SMTP: provider configuration validated')).toBeInTheDocument();
  });

  it('submits SMTP alert providers with sender and recipient envelope fields', async () => {
    mockFetchRoutes([
      { path: alertsPath, response: okResponse([]) },
      {
        path: alertProvidersPath,
        response: okResponse([]),
      },
      {
        path: alertProvidersPath,
        method: 'POST',
        response: okResponse({
          id: 'alert_provider_smtp',
          kind: 'smtp',
          channel: 'email',
          name: 'Primary SMTP',
          status: 'active',
          config: {
            smtp_host: 'smtp.example.com',
            smtp_port: '587',
            username: 'alerts@example.com',
            password: '********',
            from_email: 'alerts@example.com',
            recipients: 'ops@example.com,oncall@example.com',
          },
        }),
      },
    ]);

    render(<AdminAlertsPage />);

    await screen.findByLabelText('Alert notification providers');
    fireEvent.change(screen.getByLabelText('Provider name'), { target: { value: 'Primary SMTP' } });
    fireEvent.change(screen.getByLabelText('Provider kind'), { target: { value: 'smtp' } });
    fireEvent.change(screen.getByLabelText('SMTP host'), { target: { value: 'smtp.example.com' } });
    fireEvent.change(screen.getByLabelText('SMTP port'), { target: { value: '587' } });
    fireEvent.change(screen.getByLabelText('SMTP username'), { target: { value: 'alerts@example.com' } });
    fireEvent.change(screen.getByLabelText('SMTP password'), { target: { value: 'smtp-secret' } });
    fireEvent.change(screen.getByLabelText('From email'), { target: { value: 'alerts@example.com' } });
    fireEvent.change(screen.getByLabelText('Recipients'), { target: { value: 'ops@example.com,oncall@example.com' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create alert provider' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        alertProvidersPath,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            kind: 'smtp',
            name: 'Primary SMTP',
            status: 'active',
            config: {
              smtp_host: 'smtp.example.com',
              smtp_port: '587',
              username: 'alerts@example.com',
              password: 'smtp-secret',
              from_email: 'alerts@example.com',
              recipients: 'ops@example.com,oncall@example.com',
            },
          }),
        })
      )
    );
  });

  it('submits third-party alert providers with kind-specific credentials', async () => {
    mockFetchRoutes([
      { path: alertsPath, response: okResponse([]) },
      {
        path: alertProvidersPath,
        response: okResponse([]),
      },
      {
        path: alertProvidersPath,
        method: 'POST',
        response: okResponse({
          id: 'alert_provider_pagerduty',
          kind: 'pagerduty',
          channel: 'third_party',
          name: 'PagerDuty Ops',
          status: 'active',
          config: {
            routing_key: '********',
            api_url: 'https://events.pagerduty.com/v2/enqueue',
          },
        }),
      },
    ]);

    render(<AdminAlertsPage />);

    await screen.findByLabelText('Alert notification providers');
    fireEvent.change(screen.getByLabelText('Provider name'), { target: { value: 'PagerDuty Ops' } });
    fireEvent.change(screen.getByLabelText('Provider kind'), { target: { value: 'pagerduty' } });
    fireEvent.change(screen.getByLabelText('Routing key'), { target: { value: 'pd-routing-key' } });
    fireEvent.change(screen.getByLabelText('Provider API URL'), { target: { value: 'https://events.pagerduty.test/v2/enqueue' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create alert provider' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        alertProvidersPath,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            kind: 'pagerduty',
            name: 'PagerDuty Ops',
            status: 'active',
            config: {
              routing_key: 'pd-routing-key',
              api_url: 'https://events.pagerduty.test/v2/enqueue',
            },
          }),
        })
      )
    );
  });

  it('submits Opsgenie alert providers with API keys', async () => {
    mockFetchRoutes([
      { path: alertsPath, response: okResponse([]) },
      {
        path: alertProvidersPath,
        response: okResponse([]),
      },
      {
        path: alertProvidersPath,
        method: 'POST',
        response: okResponse({
          id: 'alert_provider_opsgenie',
          kind: 'opsgenie',
          channel: 'third_party',
          name: 'Opsgenie Ops',
          status: 'active',
          config: {
            api_key: '********',
            api_url: 'https://api.opsgenie.com/v2/alerts',
          },
        }),
      },
    ]);

    render(<AdminAlertsPage />);

    await screen.findByLabelText('Alert notification providers');
    fireEvent.change(screen.getByLabelText('Provider name'), { target: { value: 'Opsgenie Ops' } });
    fireEvent.change(screen.getByLabelText('Provider kind'), { target: { value: 'opsgenie' } });
    fireEvent.change(screen.getByLabelText('API key'), { target: { value: 'opsgenie-key' } });
    fireEvent.change(screen.getByLabelText('Provider API URL'), { target: { value: 'https://opsgenie.example/v2/alerts' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create alert provider' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        alertProvidersPath,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            kind: 'opsgenie',
            name: 'Opsgenie Ops',
            status: 'active',
            config: {
              api_key: 'opsgenie-key',
              api_url: 'https://opsgenie.example/v2/alerts',
            },
          }),
        })
      )
    );
  });

  it('submits cloud monitor alert providers with webhook URLs', async () => {
    mockFetchRoutes([
      { path: alertsPath, response: okResponse([]) },
      {
        path: alertProvidersPath,
        response: okResponse([]),
      },
      {
        path: alertProvidersPath,
        method: 'POST',
        response: okResponse({
          id: 'alert_provider_aliyun_monitor',
          kind: 'aliyun_monitor',
          channel: 'third_party',
          name: 'Aliyun Monitor',
          status: 'active',
          config: {
            webhook_url: '********',
          },
        }),
      },
    ]);

    render(<AdminAlertsPage />);

    await screen.findByLabelText('Alert notification providers');
    fireEvent.change(screen.getByLabelText('Provider name'), { target: { value: 'Aliyun Monitor' } });
    fireEvent.change(screen.getByLabelText('Provider kind'), { target: { value: 'aliyun_monitor' } });
    fireEvent.change(screen.getByLabelText('Provider webhook URL'), { target: { value: 'https://monitor.example/alert' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create alert provider' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        alertProvidersPath,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            kind: 'aliyun_monitor',
            name: 'Aliyun Monitor',
            status: 'active',
            config: {
              webhook_url: 'https://monitor.example/alert',
            },
          }),
        })
      )
    );
  });

  it('submits Twilio SMS alert providers with recipient limits configuration', async () => {
    mockFetchRoutes([
      { path: alertsPath, response: okResponse([]) },
      {
        path: alertProvidersPath,
        response: okResponse([]),
      },
      {
        path: alertProvidersPath,
        method: 'POST',
        response: okResponse({
          id: 'alert_provider_twilio_sms',
          kind: 'twilio_sms',
          channel: 'sms',
          name: 'Twilio SMS',
          status: 'active',
          config: {
            account_sid: 'AC123',
            auth_token: '********',
            from_number: '+15550000000',
            recipients: '+15550000001,+15550000002',
            api_url: 'https://api.twilio.test',
          },
        }),
      },
    ]);

    render(<AdminAlertsPage />);

    await screen.findByLabelText('Alert notification providers');
    fireEvent.change(screen.getByLabelText('Provider name'), { target: { value: 'Twilio SMS' } });
    fireEvent.change(screen.getByLabelText('Provider kind'), { target: { value: 'twilio_sms' } });
    fireEvent.change(screen.getByLabelText('Account SID'), { target: { value: 'AC123' } });
    fireEvent.change(screen.getByLabelText('Auth token'), { target: { value: 'twilio-secret' } });
    fireEvent.change(screen.getByLabelText('From number'), { target: { value: '+15550000000' } });
    fireEvent.change(screen.getByLabelText('SMS recipients'), { target: { value: '+15550000001,+15550000002' } });
    fireEvent.change(screen.getByLabelText('Provider API URL'), { target: { value: 'https://api.twilio.test' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create alert provider' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        alertProvidersPath,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            kind: 'twilio_sms',
            name: 'Twilio SMS',
            status: 'active',
            config: {
              account_sid: 'AC123',
              auth_token: 'twilio-secret',
              from_number: '+15550000000',
              recipients: '+15550000001,+15550000002',
              api_url: 'https://api.twilio.test',
            },
          }),
        })
      )
    );
  });

  it('submits Aliyun SMS alert providers with template configuration', async () => {
    mockFetchRoutes([
      { path: alertsPath, response: okResponse([]) },
      {
        path: alertProvidersPath,
        response: okResponse([]),
      },
      {
        path: alertProvidersPath,
        method: 'POST',
        response: okResponse({
          id: 'alert_provider_aliyun_sms',
          kind: 'aliyun_sms',
          channel: 'sms',
          name: 'Aliyun SMS',
          status: 'active',
          config: {
            access_key_id: 'aliyun-key',
            access_key_secret: '********',
            sign_name: 'Oblivious',
            template_code: 'SMS_123',
            recipients: '+8613800000000',
            api_url: 'https://dysmsapi.aliyun.test',
          },
        }),
      },
    ]);

    render(<AdminAlertsPage />);

    await screen.findByLabelText('Alert notification providers');
    fireEvent.change(screen.getByLabelText('Provider name'), { target: { value: 'Aliyun SMS' } });
    fireEvent.change(screen.getByLabelText('Provider kind'), { target: { value: 'aliyun_sms' } });
    fireEvent.change(screen.getByLabelText('Access key ID'), { target: { value: 'aliyun-key' } });
    fireEvent.change(screen.getByLabelText('Access key secret'), { target: { value: 'aliyun-secret' } });
    fireEvent.change(screen.getByLabelText('Sign name'), { target: { value: 'Oblivious' } });
    fireEvent.change(screen.getByLabelText('Template code'), { target: { value: 'SMS_123' } });
    fireEvent.change(screen.getByLabelText('SMS recipients'), { target: { value: '+8613800000000' } });
    fireEvent.change(screen.getByLabelText('Provider API URL'), { target: { value: 'https://dysmsapi.aliyun.test' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create alert provider' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        alertProvidersPath,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            kind: 'aliyun_sms',
            name: 'Aliyun SMS',
            status: 'active',
            config: {
              access_key_id: 'aliyun-key',
              access_key_secret: 'aliyun-secret',
              sign_name: 'Oblivious',
              template_code: 'SMS_123',
              recipients: '+8613800000000',
              api_url: 'https://dysmsapi.aliyun.test',
            },
          }),
        })
      )
    );
  });

  it('submits phone alert providers with Twilio call credentials', async () => {
    mockFetchRoutes([
      { path: alertsPath, response: okResponse([]) },
      {
        path: alertProvidersPath,
        response: okResponse([]),
      },
      {
        path: alertProvidersPath,
        method: 'POST',
        response: okResponse({
          id: 'alert_provider_phone',
          kind: 'phone',
          channel: 'phone',
          name: 'On-call phone',
          status: 'active',
          config: {
            provider: 'twilio',
            account_sid: 'AC123',
            auth_token: '********',
            from_number: '+15550000000',
            phone_numbers: '+15550000003',
            api_url: 'https://api.twilio.test',
          },
        }),
      },
    ]);

    render(<AdminAlertsPage />);

    await screen.findByLabelText('Alert notification providers');
    fireEvent.change(screen.getByLabelText('Provider name'), { target: { value: 'On-call phone' } });
    fireEvent.change(screen.getByLabelText('Provider kind'), { target: { value: 'phone' } });
    fireEvent.change(screen.getByLabelText('Phone provider'), { target: { value: 'twilio' } });
    fireEvent.change(screen.getByLabelText('Account SID'), { target: { value: 'AC123' } });
    fireEvent.change(screen.getByLabelText('Auth token'), { target: { value: 'twilio-secret' } });
    fireEvent.change(screen.getByLabelText('From number'), { target: { value: '+15550000000' } });
    fireEvent.change(screen.getByLabelText('Phone numbers'), { target: { value: '+15550000003' } });
    fireEvent.change(screen.getByLabelText('Provider API URL'), { target: { value: 'https://api.twilio.test' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create alert provider' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        alertProvidersPath,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            kind: 'phone',
            name: 'On-call phone',
            status: 'active',
            config: {
              provider: 'twilio',
              account_sid: 'AC123',
              auth_token: 'twilio-secret',
              from_number: '+15550000000',
              phone_numbers: '+15550000003',
              api_url: 'https://api.twilio.test',
            },
          }),
        })
      )
    );
  });

  it('requests admin observability alerts and renders severity and status', async () => {
    mockAlertsResponse([
      {
        id: 'alert_1',
        name: 'Relay latency spike',
        severity: 'critical',
        status: 'firing',
        summary: 'p95 latency exceeded threshold',
        source: 'relay',
        lastTriggeredAt: '2026-06-05T08:00:00Z',
      },
      {
        id: 'alert_2',
        name: 'Channel recovered',
        severity: 'warning',
        status: 'resolved',
        summary: 'provider health returned to normal',
        source: 'channels',
      },
    ]);

    render(<AdminAlertsPage />);

    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/observability/alerts', expect.objectContaining({ method: 'GET' })));
    expect(await screen.findByRole('heading', { name: 'Alerts' })).toBeInTheDocument();
    expect(screen.getByText('Relay latency spike')).toBeInTheDocument();
    expect(screen.getByText('p95 latency exceeded threshold')).toBeInTheDocument();
    expect(screen.getByText('critical')).toBeInTheDocument();
    expect(screen.getByText('firing')).toBeInTheDocument();
    expect(screen.getByText('Channel recovered')).toBeInTheDocument();
    expect(screen.getByText('warning')).toBeInTheDocument();
    expect(screen.getByText('resolved')).toBeInTheDocument();
  });

  it('renders an empty state when no alerts are active', async () => {
    mockAlertsResponse([]);

    render(<AdminAlertsPage />);

    expect(await screen.findByText('No alerts to review.')).toBeInTheDocument();
  });

  it('renders an error state when alerts fail to load', async () => {
    mockFetchRoutes([{ path: alertsPath, response: errorResponse(500, 'alerts unavailable') }]);

    render(<AdminAlertsPage />);

    expect(await screen.findByText('Unable to load alerts.')).toBeInTheDocument();
    expect(screen.getAllByText('alerts unavailable')[0]).toBeInTheDocument();
  });

  it('filters alerts by severity status and component through the admin query', async () => {
    mockFetchRoutes([
      {
        path: alertsPath,
        response: okResponse([
          {
            key: 'workflow-failure-rate',
            title: 'Workflow failure rate',
            severity: 'critical',
            status: 'open',
            component: 'workflow',
          },
          {
            key: 'relay-latency',
            title: 'Relay latency',
            severity: 'warning',
            status: 'acknowledged',
            component: 'relay',
          },
        ]),
      },
      {
        path: `${alertsPath}?severity=critical&status=acknowledged&component=relay`,
        response: okResponse([
          {
            key: 'relay-backlog',
            title: 'Relay backlog',
            severity: 'critical',
            status: 'acknowledged',
            component: 'relay',
          },
        ]),
      },
    ]);

    render(<AdminAlertsPage />);

    expect(await screen.findByText('Workflow failure rate')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Severity'), { target: { value: 'critical' } });
    fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'acknowledged' } });
    fireEvent.change(screen.getByLabelText('Component'), { target: { value: 'relay' } });
    fireEvent.click(screen.getByRole('button', { name: 'Apply alert filters' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenLastCalledWith(
        '/api/v1/admin/observability/alerts?severity=critical&status=acknowledged&component=relay',
        expect.objectContaining({ method: 'GET' })
      )
    );
    expect(await screen.findByText('Relay backlog')).toBeInTheDocument();
    expect(screen.queryByText('Workflow failure rate')).not.toBeInTheDocument();
    expect(screen.queryByText('Relay latency')).not.toBeInTheDocument();
  });

  it('acknowledges and resolves alerts through action endpoints and reloads the list', async () => {
    mockFetchRoutes([
      {
        path: alertsPath,
        response: okResponse([
          {
            id: 'state_1',
            key: 'workflow-failure-rate',
            name: 'Workflow failure rate',
            severity: 'critical',
            status: 'open',
            source: 'workflow',
          },
        ]),
      },
      {
        path: '/api/v1/admin/observability/alerts/workflow-failure-rate/acknowledge',
        method: 'POST',
        response: okResponse({ id: 'workflow-failure-rate', status: 'acknowledged' }),
      },
      {
        path: alertsPath,
        response: okResponse([
          {
            id: 'state_1',
            key: 'workflow-failure-rate',
            name: 'Workflow failure rate',
            severity: 'critical',
            status: 'acknowledged',
            source: 'workflow',
          },
        ]),
      },
      {
        path: '/api/v1/admin/observability/alerts/workflow-failure-rate/resolve',
        method: 'POST',
        response: okResponse({ id: 'workflow-failure-rate', status: 'resolved' }),
      },
      {
        path: alertsPath,
        response: okResponse([
          {
            id: 'state_1',
            key: 'workflow-failure-rate',
            name: 'Workflow failure rate',
            severity: 'critical',
            status: 'resolved',
            source: 'workflow',
          },
        ]),
      },
    ]);

    render(<AdminAlertsPage />);

    expect(await screen.findByText('Workflow failure rate')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Acknowledge Workflow failure rate' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/admin/observability/alerts/workflow-failure-rate/acknowledge',
        expect.objectContaining({ method: 'POST' })
      )
    );
    await waitFor(() => expect(screen.getByText('acknowledged')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: 'Resolve Workflow failure rate' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/admin/observability/alerts/workflow-failure-rate/resolve',
        expect.objectContaining({ method: 'POST' })
      )
    );
    await waitFor(() => expect(screen.getByText('resolved')).toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledWith(alertRoutingPath, expect.objectContaining({ method: 'GET' }));
  });

  it('keeps the alert visible and shows an error when an alert action fails', async () => {
    mockFetchRoutes([
      {
        path: alertsPath,
        response: okResponse([
          {
            id: 'relay-backlog',
            name: 'Relay backlog',
            severity: 'warning',
            status: 'open',
            source: 'relay',
          },
        ]),
      },
      {
        path: '/api/v1/admin/observability/alerts/relay-backlog/resolve',
        method: 'POST',
        response: errorResponse(404, 'alert state not found'),
      },
    ]);

    render(<AdminAlertsPage />);

    expect(await screen.findByText('Relay backlog')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Resolve Relay backlog' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/admin/observability/alerts/relay-backlog/resolve',
        expect.objectContaining({ method: 'POST' })
      )
    );
    expect(await screen.findByRole('alert')).toHaveTextContent('alert state not found');
    expect(screen.getByText('Relay backlog')).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(alertRoutingPath, expect.objectContaining({ method: 'GET' }));
  });

  it('loads delivery history for an alert', async () => {
    mockFetchRoutes([
      {
        path: alertsPath,
        response: okResponse([
          {
            key: 'relay-backlog',
            title: 'Relay backlog',
            severity: 'warning',
            status: 'open',
            component: 'relay',
            lastOccurredAt: '2026-06-05T08:30:00Z',
          },
        ]),
      },
      {
        path: '/api/v1/admin/observability/alerts/relay-backlog/deliveries',
        response: okResponse([
          {
            alertKey: 'relay-backlog',
            channel: 'email',
            delivered: true,
            attemptedAt: '2026-06-05T08:30:00Z',
          },
          {
            alertKey: 'relay-backlog',
            channel: 'im',
            providerId: 'alert_provider_slack_ops',
            providerKind: 'slack_webhook',
            delivered: false,
            error: 'im webhook failed',
            attemptedAt: '2026-06-05T08:30:00Z',
          },
        ]),
      },
    ]);

    render(<AdminAlertsPage />);

    expect(await screen.findByText('Relay backlog')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'View deliveries Relay backlog' }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/admin/observability/alerts/relay-backlog/deliveries',
        expect.objectContaining({ method: 'GET' })
      )
    );
    const history = await screen.findByLabelText('Notification delivery history');
    expect(within(history).getByText('email')).toBeInTheDocument();
    expect(within(history).getByText('delivered')).toBeInTheDocument();
    expect(within(history).getByText('im')).toBeInTheDocument();
    expect(within(history).getByText('alert_provider_slack_ops')).toBeInTheDocument();
    expect(within(history).getByText('slack_webhook')).toBeInTheDocument();
    expect(within(history).getByText('im webhook failed')).toBeInTheDocument();
  });

  it('loads recovery actions for operational incidents', async () => {
    mockFetchRoutes([
      {
        path: alertsPath,
        response: okResponse([]),
      },
      {
        path: recoveryActionsPath,
        response: okResponse([
          {
            id: 'restart-relay:relay-backlog:1',
            policyName: 'restart-relay',
            alertKey: 'relay-backlog',
            severity: 'critical',
            component: 'relay',
            type: 'restart',
            status: 'recorded',
            reason: 'Relay backlog',
            createdAt: '2026-06-05T09:00:00Z',
          },
        ]),
      },
    ]);

    render(<AdminAlertsPage />);

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/observability/recovery-actions', expect.objectContaining({ method: 'GET' }))
    );
    const recoveryPanel = await screen.findByLabelText('Recovery actions');
    expect(within(recoveryPanel).getByText('restart-relay')).toBeInTheDocument();
    expect(within(recoveryPanel).getByText('relay-backlog')).toBeInTheDocument();
    expect(within(recoveryPanel).getByText('restart')).toBeInTheDocument();
    expect(within(recoveryPanel).getByText('recorded')).toBeInTheDocument();
  });
});
