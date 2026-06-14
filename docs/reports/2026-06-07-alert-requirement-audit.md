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
| Routing rules: debug logs only, info email, warning email + IM, critical email + IM + SMS + third party. | Proven | `DefaultAlertRoutingRules`; `TestDefaultAlertDeliveryPolicyRoutesBySeverity`. Current critical routing also includes `phone`, matching section 9.2's critical phone requirement. `scripts/verify-commercial-db-evidence.sh observability-alert-recovery-persistence` now reruns SQL routing-rule persistence as no-skip PostgreSQL evidence. |

## 9.2 Four-Level Alert System

| Requirement | Status | Evidence |
| --- | --- | --- |
| Supports debug, info, warning, and critical severities. | Proven | `AlertSeverityDebug`, `AlertSeverityInfo`, `AlertSeverityWarning`, `AlertSeverityCritical`; `TestDefaultAlertDeliveryPolicyRoutesBySeverity`. |
| Debug records logs only and sends no notification. | Proven | `DefaultAlertRoutingRules` maps debug to no delivery channels; `AlertRouter.Route` logs before notification and skips notify for debug. |
| Info logs and sends email as a one-hour digest. | Proven | `AlertRouter.Route` logs all routed alerts; `AlertDeliveryDispatcher` queues info/email alerts and `FlushInfoEmailDigests` sends one digest after the configured window; `TestAlertDeliveryDispatcherBatchesInfoEmailDigestForOneHour`. |
| Warning records, attempts recovery, and notifies email + IM at most once every 15 minutes. | Proven | `normalizeAlertNotifyWindows` sets warning to 15 minutes; `TestAlertDeliveryDispatcherSuppressesRepeatedNotificationInsideWindow`; HTTP and publishing-channel paths call `RecoveryController.HandleAlert`. |
| Critical immediately attempts recovery and notifies all channels including phone. | Proven | Critical notify window is zero; default routing includes email, IM, SMS, third-party, and phone; `configureHTTPAlerting` records restart/failover recovery policies for critical HTTP alerts. |
| Same alert repeated 3 times within 5 minutes escalates one level. | Proven | `AlertEscalator` and alert state stores use a 5-minute/3-occurrence rule; `TestAlertEscalatorRaisesSeverityOnThirdRepeatWithinFiveMinutes` and `TestSQLAlertStateStorePersistsAlertLifecycleAndEscalation`. The commercial DB profile also reruns SQL lifecycle, filter, notification-throttle, recovery-cooldown, and repeated delivery-batch persistence without accepting skips. |
| Warning open for 30 minutes escalates to critical. | Proven | `shouldEscalateSustainedWarning` in memory and SQL state stores; `TestAlertStateStoreEscalatesWarningOpenForThirtyMinutes`. |

## 9.3 Fully Automated Recovery

| Requirement | Status | Evidence / gap |
| --- | --- | --- |
| Automatic restart when health check fails 3 times. | Proven | `deploy/kubernetes/app-deployment.yaml` defines `/healthz` liveness/readiness probes with `failureThreshold: 3`; `scripts/verify-k8s-recovery-policy.sh` validates the probe contract; HTTP 5xx/panic paths record restart recovery actions. |
| Restart on OOM and panic/crash. | Partial with repository panic proof | Kubernetes liveness/startup behavior can recover crashed containers. HTTP middleware now routes recovered panics into critical alert state plus `restart` recovery actions through `record-http-panic`, and `RecoveryPolicy.FieldMatches` has focused panic/OOM signal coverage in `TestRecoveryControllerMatchesPanicAndOOMRecoverySignals`; `TestWithRecoverRoutesPanicToCriticalAlertAndRecovery` proves the real middleware chain records panic recovery evidence, and `TestConfigureHTTPAlertingRoutesPanicAndOOMRecoverySignals` proves default HTTP recovery wiring records panic/OOM signals before generic critical HTTP recovery. Remaining boundary: true OOM/crash restart execution still depends on Kubernetes/runtime evidence. |
| Restart strategy max 5 restarts/10 minutes with 10s, 30s, 60s, 120s, 300s backoff, then mark failure for manual intervention. | Proven | `RecoveryController` counts restart attempts in a 10-minute window, assigns the default backoff sequence, persists `attempt` and `next_attempt_at`, and records `exhausted` on the sixth attempt; `TestRecoveryControllerSchedulesRestartBackoffAndExhaustsAfterFiveAttempts`. |
| Kubernetes `restartPolicy: Always` + readiness probe. | Proven | Deployment pod templates rely on Kubernetes Deployment default `restartPolicy: Always`; readiness/liveness probes are validated by `scripts/verify-k8s-recovery-policy.sh`. |
| Automatic scale out when CPU > 80% for 5 minutes, memory > 85% for 5 minutes, or queue backlog > 100. | Proven | `deploy/kubernetes/hpa.yaml` sets CPU 80%, memory 85%, queue backlog `workflow_queue_backlog` average value `100`, and 5-minute scale-up stabilization; `scripts/verify-k8s-recovery-policy.sh`. |
| Scale-out increases current replicas by 50%, minimum 1, with maximum configured and 5-minute cooldown. | Proven | HPA uses `Percent` 50 and `Pods` 1 scale-up policies, max replicas 10, and 300-second scale-up stabilization; `scripts/verify-k8s-recovery-policy.sh`. |
| Scale-down when CPU/memory < 30% for 15 minutes, reduce by 20%, minimum 3, cooldown 15 minutes. | Partial with explicit boundary | HPA uses min replicas 3, 20% scale-down, and 900-second stabilization. Kubernetes HPA v2 scale-down remains based on target utilization rather than a literal low-utilization threshold; `docs/release/recovery-platform-contract.md` requires custom metric/external autoscaler evidence before claiming exact `<30%` trigger behavior. |
| PostgreSQL Patroni failover, Redis Sentinel failover, Kafka leader election, load balancer health removal/rejoin. | Explicit platform boundary | `docs/release/recovery-platform-contract.md` requires Patroni or managed PostgreSQL failover, Redis Sentinel/Cluster or managed Redis failover, Kafka leader election/failover, and load-balancer target removal/rejoin evidence before production recovery readiness is claimed. The application repo does not fake these platform clusters. |

## Current Conclusion

Sections 9.1 and 9.2 are proven by focused tests after the 2026-06-07 alert notification work, and `scripts/verify-commercial-db-evidence.sh observability-alert-recovery-persistence` now supplies no-skip PostgreSQL evidence for alert routing/state/delivery/recovery-cooldown persistence. Section 9.3 remains partial with an explicit platform boundary: the repository records bounded restart recovery actions, now includes panic/OOM signal-specific restart policy and default wiring proof, and validates Kubernetes probes/HPA behavior for the expressible 9.3 autoscaling rules. Exact `<30%` scale-down trigger behavior, true OOM/crash restart execution, and infrastructure failover must be proven by deployment-platform evidence described in `docs/release/recovery-platform-contract.md`.

Next implementation order:

1. Continue the fusion matrix top-down with API gateway and Relay.
2. If production recovery readiness is requested later, attach deployment-platform evidence for the boundaries in `docs/release/recovery-platform-contract.md`.
