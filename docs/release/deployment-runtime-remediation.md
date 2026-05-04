# Deployment Runtime Remediation

This project cannot finish DEPLOY-01 until a real Docker or Kubernetes runtime is available.

## Current Blocker

Observed in this checkout:

```text
[deploy-validate] docker daemon is not reachable for the current user/session
permission denied while trying to connect to the docker API at unix:///var/run/docker.sock
dockerd needs to be started with root privileges.
kubectl: command not found
```

## Option A: Restore Docker Access

Use one of these host-level fixes:

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

## Option B: Validate With Kubernetes

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

DEPLOY-01 can be marked complete only after one real runtime path starts the actual stack and `scripts/deploy-smoke.sh` passes against that stack.
