import type { Page, Route } from '@playwright/test';

const now = '2026-06-15T16:00:00Z';

const adminSession = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_admin_alerts',
    expiresAt: '2026-06-16T16:00:00Z',
  },
  user: {
    id: 'user_admin_alerts',
    email: 'alerts-admin@example.com',
    name: 'Alerts Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_admin_alerts',
  },
};

const workflowAlert = {
  key: 'workflow-failure-rate',
  name: 'Workflow failure rate',
  severity: 'critical',
  status: 'open',
  summary: 'DAG success rate dropped below 99 percent.',
  source: 'workflow',
  lastTriggeredAt: now,
  occurrenceCount: 3,
};

const relayAlert = {
  key: 'relay-backlog',
  title: 'Relay backlog',
  severity: 'warning',
  status: 'open',
  summary: 'Queue depth exceeded the recovery policy threshold.',
  component: 'relay',
  lastOccurredAt: now,
  occurrenceCount: 7,
};

const mobileAlert = {
  key: 'mobile-alert-provider-research-cluster',
  title: 'mobilealertproviderresearchclusterwithoutbreaks20260624',
  severity: 'critical',
  status: 'acknowledged',
  summary: 'alertdeliveryfailureevidencemobilewithoutbreaks20260624 requires operator inspection.',
  component: 'observability',
  lastOccurredAt: now,
  occurrenceCount: 2,
};

const mobileRecoveryAction = {
  id: 'policyrestartrelayproviderresearchclusterwithoutbreaks20260624:relay-backlog:1',
  policyName: 'policyrestartrelayproviderresearchclusterwithoutbreaks20260624',
  alertKey: relayAlert.key,
  severity: 'critical',
  component: 'relay',
  type: 'restart',
  status: 'recorded',
  reason: 'Relay backlog recovery policy',
  createdAt: now,
};

const initialRoutingRules = {
  debug: [],
  info: ['email'],
  warning: ['email', 'im'],
  critical: ['email', 'im', 'sms', 'third_party'],
};

const savedRoutingRules = {
  debug: [],
  info: ['email'],
  warning: ['email', 'im', 'sms'],
  critical: ['email', 'im', 'sms', 'third_party'],
};

const smtpProvider = {
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
};

const slackProvider = {
  id: 'alert_provider_slack',
  kind: 'slack_webhook',
  channel: 'im',
  name: 'Slack Ops',
  status: 'active',
  config: {
    webhook_url: '********',
  },
};

const mobileProvider = {
  id: 'providerdeliverytargetwithoutbreaks20260624',
  kind: 'slack_webhook',
  channel: 'im',
  name: 'mobilealertproviderresearchclusterwithoutbreaks20260624',
  status: 'active',
  config: {
    webhook_url: 'https://hooks.slack.example/mobile-alert-provider-research-cluster',
  },
};

function envelope(data: unknown) {
  return {
    ok: true,
    data,
    error: null,
  };
}

async function fulfillJSON(route: Route, data: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(envelope(data)),
  });
}

async function fulfillError(route: Route, message: string, status = 422) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'fixture_contract_mismatch', message },
    }),
  });
}

async function fulfillNotFound(route: Route) {
  await route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'not_found', message: 'admin alerts fixture route not found' },
    }),
  });
}

function routingPayloadMatches(payload: Record<string, unknown>) {
  return JSON.stringify(payload.rules) === JSON.stringify(savedRoutingRules);
}

function providerPayloadMatches(payload: Record<string, unknown>) {
  return (
    payload.kind === 'slack_webhook' &&
    payload.name === 'Slack Ops' &&
    payload.status === 'active' &&
    JSON.stringify(payload.config) === JSON.stringify({ webhook_url: 'https://hooks.slack.example/browser' })
  );
}

function alertQueryMatches(url: URL) {
  return (
    url.searchParams.get('severity') === 'critical' &&
    url.searchParams.get('status') === 'acknowledged' &&
    url.searchParams.get('component') === 'relay'
  );
}

export async function registerAdminAlertsRoutes(page: Page): Promise<void> {
  let workflowStatus = workflowAlert.status;
  let routingRules = initialRoutingRules;
  const providers = [smtpProvider, mobileProvider];

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, adminSession);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/observability/alerts') {
      if (url.search) {
        if (!alertQueryMatches(url)) {
          await fulfillError(route, 'alert filter query did not match severity/status/component evidence');
          return;
        }
        await fulfillJSON(route, {
          alerts: [{ ...relayAlert, status: 'acknowledged', severity: 'critical' }],
          total: 1,
        });
        return;
      }

      await fulfillJSON(route, {
        alerts: [{ ...workflowAlert, status: workflowStatus }, relayAlert, mobileAlert],
        total: 3,
      });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/observability/alerts/${workflowAlert.key}/acknowledge`) {
      workflowStatus = 'acknowledged';
      await fulfillJSON(route, { ...workflowAlert, status: workflowStatus });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/observability/alerts/${workflowAlert.key}/resolve`) {
      workflowStatus = 'resolved';
      await fulfillJSON(route, { ...workflowAlert, status: workflowStatus });
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/admin/observability/alerts/${relayAlert.key}/deliveries`) {
      await fulfillJSON(route, {
        attempts: [
          {
            id: 'delivery_email',
            alertKey: relayAlert.key,
            channel: 'email',
            providerId: smtpProvider.id,
            providerKind: smtpProvider.kind,
            delivered: true,
            error: '',
            attemptedAt: now,
          },
          {
            id: 'delivery_im_failed',
            alertKey: relayAlert.key,
            channel: 'im',
            providerId: slackProvider.id,
            providerKind: slackProvider.kind,
            delivered: false,
            error: 'im webhook failed',
            attemptedAt: now,
          },
          {
            id: 'delivery_im_mobile_without_breaks',
            alertKey: relayAlert.key,
            channel: 'im',
            providerId: mobileProvider.id,
            providerKind: mobileProvider.kind,
            delivered: false,
            error: 'deliveryerrorwithoutbreaks20260624',
            attemptedAt: now,
          },
        ],
        total: 3,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/observability/recovery-actions') {
      await fulfillJSON(route, {
        actions: [
          mobileRecoveryAction,
          {
            id: 'restart-relay:relay-backlog:1',
            policyName: 'restart-relay',
            alertKey: relayAlert.key,
            severity: 'critical',
            component: 'relay',
            type: 'restart',
            status: 'recorded',
            reason: 'Relay backlog recovery policy',
            createdAt: now,
          },
        ],
        total: 1,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/observability/alert-routing') {
      await fulfillJSON(route, routingRules);
      return;
    }

    if (method === 'PUT' && pathname === '/api/v1/admin/observability/alert-routing') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!routingPayloadMatches(payload)) {
        await fulfillError(route, 'alert-routing payload did not include warning SMS channel evidence');
        return;
      }
      routingRules = savedRoutingRules;
      await fulfillJSON(route, routingRules);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/observability/alert-providers') {
      await fulfillJSON(route, { providers, total: providers.length });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/observability/alert-providers') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!providerPayloadMatches(payload)) {
        await fulfillError(route, 'alert-provider payload did not include Slack webhook evidence');
        return;
      }
      providers.push(slackProvider);
      await fulfillJSON(route, slackProvider, 201);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/observability/alert-providers/${smtpProvider.id}/test`) {
      await fulfillJSON(route, {
        providerId: smtpProvider.id,
        kind: smtpProvider.kind,
        channel: smtpProvider.channel,
        ok: true,
        message: 'provider configuration validated',
        testedAt: now,
      });
      return;
    }

    await fulfillNotFound(route);
  });
}
