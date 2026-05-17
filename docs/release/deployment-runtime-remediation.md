# Deployment Runtime Remediation

DEPLOY-01 is complete after a real Docker compose build/start/smoke passed on 2026-05-12. Keep this remediation note for hosts where the default Docker Hub or Go module paths are still blocked.

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
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

Observed result: `oblivious-oblivious-server` and `oblivious-oblivious-web` built, compose started PostgreSQL, Redis, server, and web, `scripts/deploy-smoke.sh` passed against `http://127.0.0.1:8080/healthz`, and the validation script removed the compose stack.

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
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ bash scripts/deploy-validate.sh
```

If Go module downloads fail from inside `Dockerfile.server`, include the validated Go proxy overrides:

```bash
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

## Option C: Validate With Kubernetes

Install or provide `kubectl` plus a target cluster, then fill secrets outside git:

```bash
cp deploy/kubernetes/secret.example.yaml /tmp/oblivious-secret.yaml
# edit /tmp/oblivious-secret.yaml with real values outside git
kubectl apply -f deploy/kubernetes/namespace.yaml
kubectl apply -f /tmp/oblivious-secret.yaml
kubectl apply -f deploy/kubernetes/
```

Confirm rollout and smoke:

```bash
kubectl -n oblivious rollout status deployment/oblivious-server
kubectl -n oblivious rollout status deployment/oblivious-web
```

Expose the server service as appropriate for the cluster, then run:

```bash
BASE_URL=http://127.0.0.1:8080 bash scripts/deploy-smoke.sh
```

## Completion Rule

DEPLOY-01 can be marked complete only after one real runtime path starts the actual stack and `scripts/deploy-smoke.sh` passes against that stack. This checkout met that rule through Docker compose on 2026-05-12.
