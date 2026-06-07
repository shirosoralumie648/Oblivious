# Alert Requirement Audit - 2026-06-07

Scope: `docs/superpowers/specs/2026-06-04-functional-logic-details.md` sections 9.1-9.3.

Status values:

- `Proven`: current code and focused tests prove the requirement.
- `Partial`: current code exists, but the requirement is only partly implemented or partly verified.
- `Gap`: current evidence contradicts or misses the requirement.

## 9.1 Alert Notification Channels

| Requirement | Status | Evidence |
| --- | --- | --- |
| SMTP email provider with `smtp_host`, `smtp_port`, `username`, `password` config. | Proven | `src/server/internal/observability/alert_provider_config.go` validates SMTP config; `TestAlertProviderConfigRequiresSMTPEnvelopeFields` and `TestAlertProviderDeliveryResolverSendsSMTPEmailProvider` cover config and delivery. |
| Email templates include both plain text and HTML. | Proven | `smtpAlertMessage`, `smtpAlertPlainBody`, and `smtpAlertHTMLBody` build multipart alternatives; `TestAlertProviderDeliveryResolverSendsSMTPEmailProvider` asserts both MIME parts. |
| Email applies to info and warning. | Proven | `DefaultAlertRoutingRules` routes info to email and warning to email + IM; info email digest is covered by `TestAlertDeliveryDispatcherBatchesInfoEmailDigestForOneHour`; warning SMTP delivery is covered by `TestAlertProviderDeliveryResolverSendsSMTPEmailProvider`. |
| IM robot providers support Slack, Feishu, DingTalk, and WeCom. | Proven | `AlertProviderKindSlackWebhook`, `AlertProviderKindFeishuWebhook`, `AlertProviderKindDingTalk`, and `AlertProviderKindWeComWebhook`; `TestAlertProviderDeliveryResolverPostsActiveSlackWebhookProvider` and `TestAlertProviderDeliveryResolverPostsNativeIMWebhookProviders`. |
| IM robot payloads use Markdown rich messages and apply to all severities. | Proven | `imAlertWebhookPayload` emits Slack `mrkdwn` blocks, Feishu interactive markdown cards, DingTalk markdown, and WeCom markdown; default routing includes IM for warning and critical, and routing rules allow IM for any severity. |
| SMS supports Twilio and Aliyun SMS. | Proven | `SMSAlertDeliverySink` dispatches `AlertProviderKindTwilioSMS` and `AlertProviderKindAliyunSMS`; `TestAlertProviderDeliveryResolverPostsSMSAndPhoneProvidersWithRecipientLimits` covers both providers. |
| Phone delivery supports critical escalation without harassment. | Proven | `PhoneAlertDeliverySink` supports Twilio calls; per-recipient phone limit is 1/hour in `AlertRecipientDeliveryLimiter`; `TestAlertProviderDeliveryResolverPostsSMSAndPhoneProvidersWithRecipientLimits` verifies the limit. |
| SMS limit is 5/hour/person. | Proven | `smsHourlyRecipientLimit`; `TestAlertProviderDeliveryResolverPostsSMSAndPhoneProvidersWithRecipientLimits` verifies Twilio and Aliyun sends stop after 5/hour/person. |
| Third-party integrations support PagerDuty, Opsgenie, Aliyun Monitor, and Tencent Cloud through API/webhook. | Proven | `ThirdPartyAlertDeliverySink`; `TestAlertProviderDeliveryResolverPostsThirdPartyProviders` covers PagerDuty, Opsgenie, Aliyun Monitor, and Tencent Cloud style payloads. |
| Routing rules: debug logs only, info email, warning email + IM, critical email + IM + SMS + third party. | Proven | `DefaultAlertRoutingRules`; `TestDefaultAlertDeliveryPolicyRoutesBySeverity`. Current critical routing also includes `phone`, matching section 9.2's critical phone requirement. |

## 9.2 Four-Level Alert System

| Requirement | Status | Evidence |
| --- | --- | --- |
| Supports debug, info, warning, and critical severities. | Proven | `AlertSeverityDebug`, `AlertSeverityInfo`, `AlertSeverityWarning`, `AlertSeverityCritical`; `TestDefaultAlertDeliveryPolicyRoutesBySeverity`. |
| Debug records logs only and sends no notification. | Proven | `DefaultAlertRoutingRules` maps debug to no delivery channels; `AlertRouter.Route` logs before notification and skips notify for debug. |
| Info logs and sends email as a one-hour digest. | Proven | `AlertRouter.Route` logs all routed alerts; `AlertDeliveryDispatcher` queues info/email alerts and `FlushInfoEmailDigests` sends one digest after the configured window; `TestAlertDeliveryDispatcherBatchesInfoEmailDigestForOneHour`. |
| Warning records, attempts recovery, and notifies email + IM at most once every 15 minutes. | Proven | `normalizeAlertNotifyWindows` sets warning to 15 minutes; `TestAlertDeliveryDispatcherSuppressesRepeatedNotificationInsideWindow`; HTTP and publishing-channel paths call `RecoveryController.HandleAlert`. |
| Critical immediately attempts recovery and notifies all channels including phone. | Proven | Critical notify window is zero; default routing includes email, IM, SMS, third-party, and phone; `configureHTTPAlerting` records restart/failover recovery policies for critical HTTP alerts. |
| Same alert repeated 3 times within 5 minutes escalates one level. | Proven | `AlertEscalator` and alert state stores use a 5-minute/3-occurrence rule; `TestAlertEscalatorRaisesSeverityOnThirdRepeatWithinFiveMinutes` and `TestSQLAlertStateStorePersistsAlertLifecycleAndEscalation`. |
| Warning open for 30 minutes escalates to critical. | Proven | `shouldEscalateSustainedWarning` in memory and SQL state stores; `TestAlertStateStoreEscalatesWarningOpenForThirtyMinutes`. |

## 9.3 Fully Automated Recovery

| Requirement | Status | Evidence / gap |
| --- | --- | --- |
| Automatic restart when health check fails 3 times. | Partial | Kubernetes deployments include `/healthz` liveness/readiness probes with `failureThreshold: 3`, and HTTP 5xx/panic paths record restart recovery actions. Missing: a controller-backed restart executor or manifest validation proving failed health checks trigger the intended Kubernetes restart path. |
| Restart on OOM and panic/crash. | Partial | Kubernetes liveness/startup behavior can recover crashed containers, and HTTP middleware routes panic/5xx into recovery action records. Missing: explicit OOM/panic policy tests and restart action execution semantics. |
| Restart strategy max 5 restarts/10 minutes with 10s, 30s, 60s, 120s, 300s backoff, then mark failure for manual intervention. | Proven | `RecoveryController` counts restart attempts in a 10-minute window, assigns the default backoff sequence, persists `attempt` and `next_attempt_at`, and records `exhausted` on the sixth attempt; `TestRecoveryControllerSchedulesRestartBackoffAndExhaustsAfterFiveAttempts`. |
| Kubernetes `restartPolicy: Always` + readiness probe. | Partial | Deployment templates define readiness/liveness probes; Deployment pod templates rely on Kubernetes default `restartPolicy: Always`. Missing: manifest audit or validation that asserts the intended restart policy/probe contract. |
| Automatic scale out when CPU > 80% for 5 minutes, memory > 85% for 5 minutes, or queue backlog > 100. | Partial | `deploy/kubernetes/hpa.yaml` exists and has CPU/memory metrics. Missing: thresholds are currently CPU 70/memory 80, there is no queue backlog metric trigger, and no test validates the HPA spec against Functional Logic 9.3. |
| Scale-out increases current replicas by 50%, minimum 1, with maximum configured and 5-minute cooldown. | Partial | HPA has max replicas and scale behavior, but current policy is fixed pod increments rather than 50% scaling and 5-minute cooldown proof. |
| Scale-down when CPU/memory < 30% for 15 minutes, reduce by 20%, minimum 3, cooldown 15 minutes. | Gap | Current HPA min replicas is 2 and scale-down behavior is 1 pod per 120 seconds with 300-second stabilization. It does not match the specified minimum or 15-minute cooldown. |
| PostgreSQL Patroni failover, Redis Sentinel failover, Kafka leader election, load balancer health removal/rejoin. | Gap | Current repository has runbooks and basic Kubernetes manifests for PostgreSQL/Redis, but no Patroni, Redis Sentinel, Kafka, or load-balancer failover manifests/tests proving these behaviors. |

## Current Conclusion

Sections 9.1 and 9.2 are proven by focused tests after the 2026-06-07 alert notification work. Section 9.3 remains partial: the repository records bounded restart recovery actions and has baseline Kubernetes probes/HPA, but it does not yet fully verify the Kubernetes restart path, autoscaling behavior, and infrastructure failover semantics required by Functional Logic 9.3.

Next implementation order:

1. Add a Kubernetes manifest validation test or script for restart probes and HPA thresholds.
2. Update HPA manifests to match CPU/memory thresholds, min replicas, and cooldowns where Kubernetes can express them.
3. Decide whether Patroni, Redis Sentinel, Kafka, and load-balancer failover are in-repo deliverables or documented external platform prerequisites; then add manifests/tests or an explicit release boundary.
