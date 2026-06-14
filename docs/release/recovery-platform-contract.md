# Recovery Platform Contract

This contract maps Functional Logic 9.3 recovery requirements to repository-owned artifacts and deployment-platform responsibilities.

## Repository-Owned Recovery

| Capability | Repository evidence |
| --- | --- |
| HTTP health probes restart unhealthy server pods after 3 failed checks | `deploy/kubernetes/app-deployment.yaml`, `scripts/verify-k8s-recovery-policy.sh` |
| Recovered HTTP panics create critical alert state and restart recovery action records | `src/server/internal/http/middleware.go`, `TestWithRecoverRoutesPanicToCriticalAlertAndRecovery` |
| Panic/OOM recovery signals can be matched by signal-specific restart policies | `src/server/internal/observability/recovery.go`, `TestRecoveryControllerMatchesPanicAndOOMRecoverySignals` |
| Default HTTP recovery wiring records panic/OOM signals before generic critical HTTP recovery | `src/server/internal/http/server.go`, `TestConfigureHTTPAlertingRoutesPanicAndOOMRecoverySignals` |
| Restart recovery records max 5 attempts per 10 minutes, backoff `10s, 30s, 60s, 120s, 300s`, and exhausted/manual-intervention state | `src/server/internal/observability/recovery.go`, `TestRecoveryControllerSchedulesRestartBackoffAndExhaustsAfterFiveAttempts` |
| HPA scale-up on CPU 80%, memory 85%, and workflow queue backlog 100 | `deploy/kubernetes/hpa.yaml`, `scripts/verify-k8s-recovery-policy.sh` |
| HPA scale-up by 50% with at least 1 pod and max replicas configured | `deploy/kubernetes/hpa.yaml`, `scripts/verify-k8s-recovery-policy.sh` |
| HPA scale-down by 20% with minimum 3 replicas and 15-minute stabilization | `deploy/kubernetes/hpa.yaml`, `scripts/verify-k8s-recovery-policy.sh` |

## Platform Responsibilities

The repository ships reference manifests for PgBouncer, MinIO, and Kafka (`deploy/kubernetes/pgbouncer.yaml`, `deploy/kubernetes/minio.yaml`, `deploy/kubernetes/kafka.yaml`, plus the optional `infra-extras` docker-compose profile; statically validated by `scripts/verify-infra-manifests.sh`). These reference shapes follow fusion spec part3 §9.1/§9.3 (PgBouncer 500 max client connections, MinIO 4-node distributed mode, Kafka 3 brokers with replication factor 3), but the repository still does not ship a full production PostgreSQL/Redis/Kafka cluster. A production deployment that claims Functional Logic 9.3 failover must provide and validate these platform components outside this application repo:

- PostgreSQL high availability through Patroni or an equivalent managed PostgreSQL failover service.
- Redis high availability through Redis Sentinel, Redis Cluster, or an equivalent managed Redis failover service.
- Kafka with at least three brokers and automatic leader election, or a managed Kafka-compatible service with equivalent failover semantics.
- load balancer health checks that remove unhealthy application targets and re-add recovered targets.

The exact `<30%` CPU/memory scale-down trigger is also a platform contract. Kubernetes HPA v2 expresses scale-down behavior through target utilization, policies, and stabilization windows. This repo therefore validates minimum replicas, 20% downscale policy, and 15-minute stabilization; deployments that require a literal `<30% for 15 minutes` trigger must add a custom metric adapter or external autoscaler rule and record that evidence with the release.

## Release Evidence Required

Before claiming production recovery readiness, attach current evidence for:

1. `bash scripts/verify-k8s-recovery-policy.sh`
2. Kubernetes rollout/smoke through `scripts/k8s-validate.sh` or an equivalent cluster validation record.
3. Patroni or managed PostgreSQL failover test.
4. Redis Sentinel/Cluster or managed Redis failover test.
5. Kafka broker leader election/failover test if Kafka-backed deployment is enabled.
6. Load balancer target removal/rejoin test.
7. Custom autoscaler evidence if claiming exact `<30%` scale-down trigger behavior.
