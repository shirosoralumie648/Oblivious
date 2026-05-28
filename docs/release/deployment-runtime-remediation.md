# Deployment Runtime Remediation

DEPLOY-01 is complete after a real Docker compose build/start/smoke passed on 2026-05-12. Phase 21 extends this release-candidate proof into v07 Production Operations by requiring migration-aware deployment validation, shared app/Relay smoke probes, and executable Kubernetes validation where cluster tooling exists. Keep this remediation note for hosts where the default Docker Hub or Go module paths are still blocked.

## Current Runtime Evidence

Current 2026-05-12 observation in this checkout:

```text
docker info: passes
docker compose config: passes
bash scripts/deploy-validate.sh with restricted-network overrides: builds images, starts stack, and passes /healthz smoke
Docker Hub via user proxy: curl --proxy http://127.0.0.1:7897 https://registry-1.docker.io/v2/ reaches the registry
Docker daemon pulls without overrides: still time out or refuse direct registry connections
Docker client credsStore: stale "desktop" entry removed from ~/.docker/config.json; backup at ~/.docker/config.json.bak-20260512-0426
Docker daemon proxy setup: requires sudo/root; current non-interactive sudo requires a password
kubectl: command not found
```

Verified restricted-network command:

```bash
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_POSTGRES_IMAGE=docker.m.daocloud.io/pgvector/pgvector:pg16 \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

Observed result: `oblivious-oblivious-server` and `oblivious-oblivious-web` built, compose started PostgreSQL, Redis, server, and web, the historical `scripts/deploy-smoke.sh` passed against `http://127.0.0.1:8080/healthz`, and the validation script removed the compose stack.

## Phase 21 Runtime Validation Contract

Phase 21 validation is stricter than the historical DEPLOY-01 check:

- `scripts/deploy-validate.sh` must start PostgreSQL and Redis, run `/usr/local/bin/oblivious-migrate` from the server image, then start server/web and run shared smoke.
- PostgreSQL runtime images must provide the `vector` extension required by `src/server/migrations/0016_pgvector.sql`; compose defaults to `pgvector/pgvector:pg16`, and restricted-network runs should set `OBLIVIOUS_POSTGRES_IMAGE=docker.m.daocloud.io/pgvector/pgvector:pg16`.
- `docker compose up` calls are bounded by `DEPLOY_VALIDATE_DOCKER_UP_TIMEOUT_SECONDS` (default `600`) so image-pull stalls fail with registry/pre-pull remediation instead of hanging without diagnostics.
- Host ports default to `8080` for the server and `4173` for the web app. If a local process already owns those ports, set `OBLIVIOUS_SERVER_HOST_PORT` and `OBLIVIOUS_WEB_HOST_PORT`; `scripts/deploy-validate.sh` derives `BASE_URL` from `OBLIVIOUS_SERVER_HOST_PORT` unless `BASE_URL` is explicitly set.
- `scripts/deploy-smoke.sh` must prove `/healthz`, `/metrics`, `/api/v1/auth/me`, and `/v1/chat/completions`.
- The app and Relay smoke probes do not require live provider credentials; they prove routes are mounted and handled locally by auth or policy rather than returning `404` or provider-network errors.
- `scripts/k8s-validate.sh` is the Kubernetes entrypoint for a real or local cluster. It requires `kubectl`, a reachable context, and `OBLIVIOUS_K8S_SECRET_FILE` pointing at a filled secret manifest outside git.
- Missing `kubectl`, missing cluster access, or a placeholder secret is recorded as non-success evidence. It must not be counted as Kubernetes proof.

Normal-network compose validation:

```bash
bash scripts/deploy-validate.sh
```

Restricted-network compose validation:

```bash
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_POSTGRES_IMAGE=docker.m.daocloud.io/pgvector/pgvector:pg16 \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

Kubernetes validation when cluster tooling is available:

```bash
cp deploy/kubernetes/secret.example.yaml /tmp/oblivious-secret.yaml
# edit /tmp/oblivious-secret.yaml outside git and replace every placeholder
OBLIVIOUS_K8S_SECRET_FILE=/tmp/oblivious-secret.yaml bash scripts/k8s-validate.sh
```

Historical 2026-05-04 observation:

```text
[deploy-validate] docker daemon is not reachable for the current user/session
permission denied while trying to connect to the docker API at unix:///var/run/docker.sock
dockerd needs to be started with root privileges.
```

## Option A: Restore Docker Daemon Access

Use one of these host-level fixes if `docker info` fails:

```bash
sudo systemctl start docker
sudo usermod -aG docker "$USER"
newgrp docker
```

Then verify:

```bash
docker info
bash scripts/deploy-validate.sh
```

If Docker Desktop is the intended runtime, start Docker Desktop and confirm the active context points at a working engine:

```bash
docker context ls
docker info
bash scripts/deploy-validate.sh
```

## Option B: Configure Docker Registry Access

If `docker info` passes but builds fail while resolving `registry-1.docker.io`, configure Docker daemon proxy or registry access.

First confirm the proxy itself reaches Docker Hub:

```bash
curl -I --proxy http://127.0.0.1:7897 https://registry-1.docker.io/v2/
```

If that works, configure the system Docker daemon to use the same proxy:

```bash
sudo mkdir -p /etc/systemd/system/docker.service.d
sudo tee /etc/systemd/system/docker.service.d/http-proxy.conf >/dev/null <<'EOF'
[Service]
Environment="HTTP_PROXY=http://127.0.0.1:7897"
Environment="HTTPS_PROXY=http://127.0.0.1:7897"
Environment="NO_PROXY=localhost,127.0.0.1,::1"
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker
docker info | grep -i proxy
bash scripts/deploy-validate.sh
```

If plain `docker pull` fails with `docker-credential-desktop` missing while using the Linux engine, remove or replace the stale Desktop credential helper in `~/.docker/config.json`:

```bash
cp ~/.docker/config.json ~/.docker/config.json.bak
# Remove the line containing: "credsStore": "desktop"
docker pull hello-world:latest
```

Do not remove the Desktop credential helper if Docker Desktop is the intended active runtime and `docker-credential-desktop` is installed.

In this checkout, the active runtime is the Linux engine at `/var/run/docker.sock`, `docker-credential-desktop` is not installed, and the stale `credsStore: desktop` entry has already been removed.

If host-level daemon proxy changes are not available, use the validated image-prefix override:

```bash
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_POSTGRES_IMAGE=docker.m.daocloud.io/pgvector/pgvector:pg16 \
  bash scripts/deploy-validate.sh
```

If Go module downloads fail from inside `Dockerfile.server`, include the validated Go proxy overrides:

```bash
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_POSTGRES_IMAGE=docker.m.daocloud.io/pgvector/pgvector:pg16 \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

If `docker compose up` times out while starting PostgreSQL, Redis, server, or web, first pre-pull the images named by `docker compose config --images`, or increase the bounded wait only for a slow but progressing mirror:

```bash
DEPLOY_VALIDATE_DOCKER_UP_TIMEOUT_SECONDS=1200 \
  OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_POSTGRES_IMAGE=docker.m.daocloud.io/pgvector/pgvector:pg16 \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

If the pgvector runtime image itself is blocked but GitHub and `apt.postgresql.org` are reachable, build the local fallback image from `Dockerfile.postgres-pgvector` and use it for compose proof:

```bash
docker build -f Dockerfile.postgres-pgvector -t oblivious-postgres-pgvector:pg16 .
OBLIVIOUS_SERVER_HOST_PORT=18080 \
  OBLIVIOUS_WEB_HOST_PORT=14173 \
  OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

The 2026-05-28 Phase 21 fallback proof used this local pgvector image path and alternate host ports because port `8080` was already owned by a local Python process.

## Option C: Validate With Kubernetes

Install or provide `kubectl` plus a target cluster, then fill secrets outside git. Prefer the scripted validator because it applies manifests in order, waits for rollouts, port-forwards the server service, and runs the same shared smoke contract as compose:

```bash
cp deploy/kubernetes/secret.example.yaml /tmp/oblivious-secret.yaml
# edit /tmp/oblivious-secret.yaml with real values outside git
OBLIVIOUS_K8S_SECRET_FILE=/tmp/oblivious-secret.yaml bash scripts/k8s-validate.sh
```

For manual debugging, the equivalent high-level sequence is:

```bash
kubectl apply -f deploy/kubernetes/namespace.yaml
kubectl apply -f /tmp/oblivious-secret.yaml
kubectl apply -f deploy/kubernetes/configmap.yaml
kubectl apply -f deploy/kubernetes/postgres.yaml
kubectl apply -f deploy/kubernetes/redis.yaml
kubectl apply -f deploy/kubernetes/server.yaml
kubectl apply -f deploy/kubernetes/web.yaml
kubectl -n oblivious rollout status deployment/oblivious-server
kubectl -n oblivious rollout status deployment/oblivious-web
```

Then expose the server service as appropriate for the cluster and run:

```bash
BASE_URL=http://127.0.0.1:8080 bash scripts/deploy-smoke.sh
```

## Completion Rule

DEPLOY-01 can be marked complete only after one real runtime path starts the actual stack and `scripts/deploy-smoke.sh` passes against that stack. This checkout met that rule through Docker compose on 2026-05-12.

For v07 `OPS-01`, the evidence bar is higher: the runtime path must also run migrations through `oblivious-migrate`, prove `/metrics`, prove one app API path, and prove one Relay path. Kubernetes proof requires `scripts/k8s-validate.sh` to pass against a real or local cluster; missing cluster tooling is useful environment evidence but not a successful Kubernetes validation.
