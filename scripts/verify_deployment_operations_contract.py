#!/usr/bin/env python3
import json
import subprocess
import sys
from pathlib import Path

import yaml


def read_yaml(path):
    with open(path, "r", encoding="utf-8") as handle:
        return yaml.safe_load(handle) or {}


def read_yaml_stream(path):
    with open(path, "r", encoding="utf-8") as handle:
        return [doc for doc in yaml.safe_load_all(handle) if doc]


def deployment_doc(path):
    docs = read_yaml_stream(path)
    return next((doc for doc in docs if doc.get("kind") == "Deployment"), docs[0] if docs else {})


def containers(doc):
    return (((doc.get("spec") or {}).get("template") or {}).get("spec") or {}).get("containers") or []


def container(doc, name):
    return next((entry for entry in containers(doc) if entry.get("name") == name), {})


def first_container(doc):
    return containers(doc)[0] if containers(doc) else {}


def dig(value, *path):
    node = value
    for key in path:
        if isinstance(node, dict):
            node = node.get(key)
        elif isinstance(node, list) and isinstance(key, int) and 0 <= key < len(node):
            node = node[key]
        else:
            return None
    return node


def has_resources(ctr):
    requests = dig(ctr, "resources", "requests") or {}
    limits = dig(ctr, "resources", "limits") or {}
    return all(str((source or {}).get(key) or "") != "" for source, key in [(requests, "cpu"), (requests, "memory"), (limits, "cpu"), (limits, "memory")])


def has_probe(ctr, probe):
    path = dig(ctr, probe, "httpGet", "path")
    port = dig(ctr, probe, "httpGet", "port")
    exec_command = dig(ctr, probe, "exec", "command")
    return (str(path or "") != "" and str(port or "") != "") or (isinstance(exec_command, list) and len(exec_command) > 0)


def has_env_ref(ctr, ref_kind, name):
    return any(dig(entry, ref_kind, "name") == name for entry in ctr.get("envFrom") or [])


def env_value(ctr, name):
    entry = next((item for item in ctr.get("env") or [] if item.get("name") == name), None)
    if entry is None:
        return ""
    if "valueFrom" in entry:
        return "__valueFrom__"
    return str(entry.get("value") or "")


def has_named_container_port(ctr, name, port):
    return any(entry.get("name") == name and int(entry.get("containerPort") or 0) == port for entry in ctr.get("ports") or [])


def service_with_port(docs, service_name, port_name, port, target_port):
    service = next((doc for doc in docs if doc.get("kind") == "Service" and dig(doc, "metadata", "name") == service_name), None)
    if service is None:
        return False
    return any(
        entry.get("name") == port_name
        and int(entry.get("port") or 0) == port
        and str(entry.get("targetPort")) == str(target_port)
        for entry in dig(service, "spec", "ports") or []
    )


def depends_on_service(service, dependency):
    depends_on = service.get("depends_on")
    if isinstance(depends_on, dict):
        return dependency in depends_on
    if isinstance(depends_on, list):
        return dependency in depends_on
    return False


def each_node(value, path=None):
    if path is None:
        path = []
    yield value, path
    if isinstance(value, dict):
        for key, child in value.items():
            yield from each_node(child, path + [key])
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from each_node(child, path + [index])


def path_label(path):
    return ".".join(str(part) for part in path)


def first_party_deployment_paths():
    return [
        "deploy/kubernetes/admin-deployment.yaml",
        "deploy/kubernetes/agent-deployment.yaml",
        "deploy/kubernetes/app-deployment.yaml",
        "deploy/kubernetes/billing-deployment.yaml",
        "deploy/kubernetes/channel-deployment.yaml",
        "deploy/kubernetes/chat-deployment.yaml",
        "deploy/kubernetes/gateway-deployment.yaml",
        "deploy/kubernetes/marketplace-deployment.yaml",
        "deploy/kubernetes/observability-deployment.yaml",
        "deploy/kubernetes/rag-deployment.yaml",
        "deploy/kubernetes/relay-deployment.yaml",
        "deploy/kubernetes/task-deployment.yaml",
        "deploy/kubernetes/workflow-deployment.yaml",
        "deploy/kubernetes/web.yaml",
    ]


def require_first_party_security_context(repo, relative_path, missing):
    deployment = deployment_doc(repo / relative_path)
    pod_security = dig(deployment, "spec", "template", "spec", "securityContext") or {}
    if pod_security.get("runAsNonRoot") is not True:
        missing.append(f"{relative_path} pod securityContext must run as non-root")
    ctrs = containers(deployment)
    if not ctrs:
        missing.append(f"{relative_path} must define at least one container")
    for ctr in ctrs:
        name = str(ctr.get("name") or "")
        security = ctr.get("securityContext") or {}
        if security.get("allowPrivilegeEscalation") is not False:
            missing.append(f"{relative_path} {name} container must disable privilege escalation")
        if security.get("readOnlyRootFilesystem") is not True:
            missing.append(f"{relative_path} {name} container must use a read-only root filesystem")
        dropped = [str(item) for item in dig(security, "capabilities", "drop") or []]
        if "ALL" not in dropped:
            missing.append(f"{relative_path} {name} container must drop all Linux capabilities")


def require_network_policy_contract(repo, missing):
    policy_path = repo / "deploy/kubernetes/network-policy.yaml"
    if not policy_path.exists():
        missing.append("kubernetes network policy manifest must exist")
        return
    policies = [doc for doc in read_yaml_stream(policy_path) if doc.get("kind") == "NetworkPolicy"]
    default_deny = next((doc for doc in policies if dig(doc, "metadata", "name") == "oblivious-default-deny"), None)
    allow_ingress = next((doc for doc in policies if dig(doc, "metadata", "name") == "oblivious-allow-platform-ingress"), None)
    allow_egress = next((doc for doc in policies if dig(doc, "metadata", "name") == "oblivious-allow-platform-egress"), None)
    if default_deny is None:
        missing.append("network policy must include oblivious-default-deny")
    else:
        policy_types = dig(default_deny, "spec", "policyTypes") or []
        if "Ingress" not in policy_types or "Egress" not in policy_types:
            missing.append("oblivious-default-deny must cover ingress and egress")
        if dig(default_deny, "spec", "podSelector") != {}:
            missing.append("oblivious-default-deny must select all pods")
        if dig(default_deny, "spec", "ingress") or []:
            missing.append("oblivious-default-deny must not allow ingress")
        if dig(default_deny, "spec", "egress") or []:
            missing.append("oblivious-default-deny must not allow egress")
    if allow_ingress is None:
        missing.append("network policy must include oblivious-allow-platform-ingress")
    else:
        ingress_rules = dig(allow_ingress, "spec", "ingress") or []
        if not any(any(dig(peer, "namespaceSelector", "matchLabels", "kubernetes.io/metadata.name") == "oblivious" for peer in rule.get("from") or []) for rule in ingress_rules):
            missing.append("oblivious-allow-platform-ingress must allow same-namespace traffic")
        if not any(any(dig(peer, "namespaceSelector", "matchLabels", "kubernetes.io/metadata.name") == "ingress-nginx" for peer in rule.get("from") or []) for rule in ingress_rules):
            missing.append("oblivious-allow-platform-ingress must allow ingress-nginx traffic")
    if allow_egress is None:
        missing.append("network policy must include oblivious-allow-platform-egress")
    else:
        egress_rules = dig(allow_egress, "spec", "egress") or []
        all_ports = [int(port.get("port") or 0) for rule in egress_rules for port in rule.get("ports") or []]
        if 53 not in all_ports:
            missing.append("oblivious-allow-platform-egress must allow DNS egress")
        if 443 not in all_ports:
            missing.append("oblivious-allow-platform-egress must allow HTTPS provider egress")
        if not any(any(dig(peer, "namespaceSelector", "matchLabels", "kubernetes.io/metadata.name") == "oblivious" for peer in rule.get("to") or []) for rule in egress_rules):
            missing.append("oblivious-allow-platform-egress must allow same-namespace service traffic")


def git_ls_k8s_files(repo):
    result = subprocess.run(["git", "-C", str(repo), "ls-files", "deploy/kubernetes/*.yaml"], check=True, text=True, capture_output=True)
    return [line.strip() for line in result.stdout.splitlines() if line.strip() and line.strip() != "deploy/kubernetes/server.yaml"]


def main():
    repo = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(__file__).resolve().parents[1]
    missing = []

    require_network_policy_contract(repo, missing)
    for relative_path in first_party_deployment_paths():
        require_first_party_security_context(repo, relative_path, missing)

    app_deployment = read_yaml(repo / "deploy/kubernetes/app-deployment.yaml")
    server = container(app_deployment, "server")
    if str(dig(app_deployment, "spec", "strategy", "rollingUpdate", "maxUnavailable")) != "0":
        missing.append("app deployment must use RollingUpdate maxUnavailable=0")
    if not has_resources(server):
        missing.append("app deployment server must define CPU/memory requests and limits")
    if not has_probe(server, "livenessProbe"):
        missing.append("app deployment server must define livenessProbe")
    if not has_probe(server, "readinessProbe"):
        missing.append("app deployment server must define readinessProbe")
    if not has_probe(server, "startupProbe"):
        missing.append("app deployment server must define startupProbe")
    if not has_env_ref(server, "configMapRef", "oblivious-config"):
        missing.append("app deployment server must load oblivious-config ConfigMap")
    if not has_env_ref(server, "secretRef", "oblivious-secrets"):
        missing.append("app deployment server must load oblivious-secrets Secret")

    config_map = read_yaml(repo / "deploy/kubernetes/configmap.yaml")
    if dig(config_map, "data", "APP_ENV") != "production":
        missing.append("configmap must set production APP_ENV")
    if dig(config_map, "data", "RELAY_RATE_LIMIT_BACKEND") != "redis":
        missing.append("configmap must enable Redis relay rate limiting")
    if str(dig(config_map, "data", "RELAY_DEFAULT_MODEL") or "") == "":
        missing.append("configmap must document relay default model")
    if str(dig(config_map, "data", "RELAY_SEMANTIC_CACHE_BACKEND") or "") == "":
        missing.append("configmap must document relay semantic cache backend")
    if dig(config_map, "data", "SESSION_COOKIE_SECURE") != "true":
        missing.append("configmap must enable secure session cookies")
    if dig(config_map, "data", "OBSERVABILITY_REQUEST_LOG_BACKEND") != "clickhouse":
        missing.append("configmap must enable ClickHouse request logs")
    if "oblivious-clickhouse" not in str(dig(config_map, "data", "CLICKHOUSE_DSN") or ""):
        missing.append("configmap must document ClickHouse DSN")
    if str(dig(config_map, "data", "ALIPAY_CHECKOUT_BASE_URL") or "") == "":
        missing.append("configmap must document Alipay checkout base URL")
    if str(dig(config_map, "data", "WECHATPAY_CHECKOUT_BASE_URL") or "") == "":
        missing.append("configmap must document WeChat Pay checkout base URL")
    if str(dig(config_map, "data", "RAG_INDEX_WORKER_ENABLED") or "") == "":
        missing.append("configmap must document RAG index worker control")

    secret_example = read_yaml(repo / "deploy/kubernetes/secret.example.yaml")
    if dig(secret_example, "metadata", "name") != "oblivious-secrets":
        missing.append("secret example must define oblivious-secrets")
    secret_data = secret_example.get("stringData") or {}
    required_secret_keys = [
        "DATABASE_URL", "SESSION_SECRET", "OBLIVIOUS_SECRET_ENCRYPTION_KEY", "POSTGRES_PASSWORD", "LLM_API_KEY", "OPENAI_API_KEY",
        "DB_URL_RELAY", "DB_URL_CHAT", "DB_URL_WORKFLOW", "DB_URL_RAG", "DB_URL_AGENT", "DB_URL_BILLING",
        "DB_URL_MARKETPLACE", "DB_URL_ADMIN", "DB_URL_CHANNEL", "DB_URL_TASK", "DB_URL_OBSERVABILITY",
        "STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "ALIPAY_WEBHOOK_SECRET", "WECHATPAY_WEBHOOK_SECRET",
        "MARKETPLACE_PAYOUT_WEBHOOK_SECRET",
    ]
    for key in required_secret_keys:
        if key not in secret_data:
            missing.append(f"secret example must document {key}")
    leaked_secret_keys = [
        key for key, value in secret_data.items()
        if str(value) != "" and "REPLACE_ME" not in str(value) and "REPLACE_WITH" not in str(value) and "CHANGE_ME" not in str(value)
    ]
    if leaked_secret_keys:
        missing.append(f"secret example must keep non-empty values as placeholders: {', '.join(leaked_secret_keys)}")

    tracked_k8s_files = git_ls_k8s_files(repo)
    config_maps = {}
    secrets = {"oblivious-secrets": list(secret_data.keys())}
    for relative_path in tracked_k8s_files:
        for doc in read_yaml_stream(repo / relative_path):
            name = str(dig(doc, "metadata", "name") or "")
            if doc.get("kind") == "ConfigMap":
                config_maps[name] = list((doc.get("data") or {}).keys())
            elif doc.get("kind") == "Secret":
                secrets[name] = list(dict.fromkeys(list((doc.get("stringData") or {}).keys()) + list((doc.get("data") or {}).keys())))

    for relative_path in tracked_k8s_files:
        for doc in read_yaml_stream(repo / relative_path):
            for node, path in each_node(doc):
                if not isinstance(node, dict):
                    continue
                secret_ref = node.get("secretKeyRef")
                if isinstance(secret_ref, dict):
                    ref_name = str(secret_ref.get("name") or "")
                    ref_key = str(secret_ref.get("key") or "")
                    if ref_name not in secrets:
                        missing.append(f"{relative_path} {path_label(path)} secretKeyRef must name a tracked Secret")
                    elif ref_key not in secrets[ref_name]:
                        missing.append(f"{relative_path} {path_label(path)} secretKeyRef {ref_name}/{ref_key} must be documented in secret.example.yaml")
                config_ref = node.get("configMapKeyRef")
                if isinstance(config_ref, dict):
                    ref_name = str(config_ref.get("name") or "")
                    ref_key = str(config_ref.get("key") or "")
                    if ref_name not in config_maps:
                        missing.append(f"{relative_path} {path_label(path)} configMapKeyRef must name a tracked ConfigMap")
                    elif ref_key not in config_maps[ref_name]:
                        missing.append(f"{relative_path} {path_label(path)} configMapKeyRef {ref_name}/{ref_key} must be documented in configmap.yaml")
                secret_ref = node.get("secretRef")
                if isinstance(secret_ref, dict):
                    ref_name = str(secret_ref.get("name") or "")
                    if ref_name not in secrets:
                        missing.append(f"{relative_path} {path_label(path)} secretRef must name a tracked Secret")
                config_ref = node.get("configMapRef")
                if isinstance(config_ref, dict):
                    ref_name = str(config_ref.get("name") or "")
                    if ref_name not in config_maps:
                        missing.append(f"{relative_path} {path_label(path)} configMapRef must name a tracked ConfigMap")
            if doc.get("kind") == "Ingress":
                annotations = dig(doc, "metadata", "annotations") or {}
                cert_manager_managed = "cert-manager.io/cluster-issuer" in annotations
                for tls in dig(doc, "spec", "tls") or []:
                    tls_secret = str(tls.get("secretName") or "")
                    if tls_secret and not cert_manager_managed and tls_secret not in secrets:
                        missing.append(f"{relative_path} Ingress TLS secret {tls_secret} must be tracked or cert-manager managed")

    hpa = read_yaml(repo / "deploy/kubernetes/hpa.yaml")
    metrics = dig(hpa, "spec", "metrics") or []
    if dig(hpa, "spec", "scaleTargetRef", "name") != "oblivious-server":
        missing.append("HPA must target oblivious-server deployment")
    if int(dig(hpa, "spec", "minReplicas") or 0) < 3:
        missing.append("HPA must keep at least three replicas")
    if not any(dig(metric, "resource", "name") == "cpu" for metric in metrics):
        missing.append("HPA must include CPU utilization metric")
    if not any(dig(metric, "resource", "name") == "memory" for metric in metrics):
        missing.append("HPA must include memory utilization metric")
    if not any(dig(metric, "external", "metric", "name") == "workflow_queue_backlog" for metric in metrics):
        missing.append("HPA must include workflow queue backlog metric")

    for component in ["postgres", "qdrant", "redis", "clickhouse"]:
        doc = deployment_doc(repo / f"deploy/kubernetes/{component}.yaml")
        ctr = container(doc, component)
        if str(dig(ctr, "resources", "requests", "cpu") or "") == "" or str(dig(ctr, "resources", "requests", "memory") or "") == "":
            missing.append(f"{component} deployment must define CPU/memory requests")
        if str(dig(ctr, "resources", "limits", "cpu") or "") == "" or str(dig(ctr, "resources", "limits", "memory") or "") == "":
            missing.append(f"{component} deployment must define CPU/memory limits")
        if not has_probe(ctr, "readinessProbe"):
            missing.append(f"{component} deployment must define readinessProbe")

    web_deployment = deployment_doc(repo / "deploy/kubernetes/web.yaml")
    web = container(web_deployment, "web")
    if not has_resources(web):
        missing.append("web deployment must define CPU/memory requests and limits")
    if not has_probe(web, "livenessProbe"):
        missing.append("web deployment must define livenessProbe")
    if not has_probe(web, "readinessProbe"):
        missing.append("web deployment must define readinessProbe")
    if not has_named_container_port(web, "http", 8080):
        missing.append("web deployment must listen on non-root HTTP port 8080")
    web_dockerfile = (repo / "Dockerfile.web").read_text(encoding="utf-8")
    if "listen 8080" not in web_dockerfile:
        missing.append("Dockerfile.web must listen on non-root port 8080")
    if "EXPOSE 8080" not in web_dockerfile:
        missing.append("Dockerfile.web must expose non-root port 8080")

    agent_docs = read_yaml_stream(repo / "deploy/kubernetes/agent-deployment.yaml")
    agent_deployment = next((doc for doc in agent_docs if doc.get("kind") == "Deployment"), {})
    agent = container(agent_deployment, "agent")
    if not has_named_container_port(agent, "http", 8083):
        missing.append("agent deployment must expose named http port 8083")
    if not has_named_container_port(agent, "grpc", 50063):
        missing.append("agent deployment must expose named grpc port 50063")
    if env_value(agent, "AGENT_GRPC_PORT") != "50063":
        missing.append("agent deployment must set AGENT_GRPC_PORT=50063")
    if env_value(agent, "GRPC_PORT") != "50063":
        missing.append("agent deployment must set GRPC_PORT=50063 for generated clients")
    if env_value(agent, "AGENT_RELAY_BASE_URL") == "":
        missing.append("agent deployment must configure AGENT_RELAY_BASE_URL")
    if env_value(agent, "DATABASE_URL") != "__valueFrom__":
        missing.append("agent deployment must read DATABASE_URL from Secret")
    if env_value(agent, "DB_URL_AGENT") != "__valueFrom__":
        missing.append("agent deployment must read DB_URL_AGENT from Secret")
    if not has_probe(agent, "livenessProbe"):
        missing.append("agent deployment must define livenessProbe")
    if not has_probe(agent, "readinessProbe"):
        missing.append("agent deployment must define readinessProbe")
    if not has_probe(agent, "startupProbe"):
        missing.append("agent deployment must define startupProbe")
    if not service_with_port(agent_docs, "agent", "grpc", 50063, "grpc"):
        missing.append("agent service must expose grpc port 50063")
    agent_dockerfile = (repo / "deploy/docker/Dockerfile.agent").read_text(encoding="utf-8")
    if "EXPOSE 8083 50063" not in agent_dockerfile:
        missing.append("Dockerfile.agent must expose HTTP and gRPC ports")
    agent_cmd = (repo / "src/server/cmd/agent/main.go").read_text(encoding="utf-8")
    for needle in ['cfg.Env == "production"', "chat.NewLocalGateway(nil)", "chat.NewCompositeGateway(relayGateway, localGenerator)"]:
        if needle not in agent_cmd:
            missing.append(f"cmd/agent must keep production Relay-disabled gateway fail-closed via {needle}")

    relay_docs = read_yaml_stream(repo / "deploy/kubernetes/relay-deployment.yaml")
    relay_deployment = next((doc for doc in relay_docs if doc.get("kind") == "Deployment"), {})
    relay_ctr = container(relay_deployment, "relay")
    if env_value(relay_ctr, "APP_ENV") != "production":
        missing.append("relay deployment must set APP_ENV=production")
    if env_value(relay_ctr, "OBLIVIOUS_DB_MODE") != "microservices":
        missing.append("relay deployment must set OBLIVIOUS_DB_MODE=microservices")
    if env_value(relay_ctr, "DATABASE_URL") != "__valueFrom__":
        missing.append("relay deployment must read DATABASE_URL from Secret")
    if env_value(relay_ctr, "DB_URL_RELAY") != "__valueFrom__":
        missing.append("relay deployment must read DB_URL_RELAY from Secret")
    if not has_probe(relay_ctr, "livenessProbe"):
        missing.append("relay deployment must define livenessProbe")
    if not has_probe(relay_ctr, "readinessProbe"):
        missing.append("relay deployment must define readinessProbe")
    if not has_probe(relay_ctr, "startupProbe"):
        missing.append("relay deployment must define startupProbe")
    relay_cmd = (repo / "src/server/cmd/relay/main.go").read_text(encoding="utf-8")
    for needle in ["db.Open", "migrations.Apply", "DBURLRelay", "SetQuotaManager", "SetAPITokenQuotaManager", "SetUsageLogger", "SetRateLimitResolver"]:
        if needle not in relay_cmd:
            missing.append(f"cmd/relay must include production wiring for {needle}")
    if "standalone relay command is not commercially wired for production" in relay_cmd:
        missing.append("cmd/relay must not self-disable in production")
    if 'log.Fatalf("load relay channels from database: %v", err)' not in relay_cmd:
        missing.append("cmd/relay must fail startup when relay channels cannot be loaded")
    if 'warning: failed to load relay channels from database' in relay_cmd:
        missing.append("cmd/relay must not continue after relay channel loading failure")
    pricing_migration = (repo / "src/server/migrations/0085_relay_pricing_entries.sql").read_text(encoding="utf-8")
    for needle in ["relay_pricing_entries", "'chat'", "'total_tokens'", "'files', '', 'storage_bytes'"]:
        if needle not in pricing_migration:
            missing.append(f"relay pricing migration must seed {needle}")
    pricing_code = (repo / "src/server/internal/relay/pricing.go").read_text(encoding="utf-8")
    for needle in ["ErrRelayPriceNotConfigured", "LoadPricingStoreFromSQL", "allowsModelAgnosticPricing"]:
        if needle not in pricing_code:
            missing.append(f"relay pricing code must include {needle}")
    router_code = (repo / "src/server/internal/relay/router.go").read_text(encoding="utf-8")
    for needle in ["relay_pricing_not_configured", "billingPreAuthorizationRouterError"]:
        if needle not in router_code:
            missing.append(f"relay router must fail closed for missing prices via {needle}")
    for file_name in ["src/server/cmd/relay/main.go", "src/server/internal/http/server.go"]:
        content = (repo / file_name).read_text(encoding="utf-8")
        for needle in ["loadRelayPricingStore", "LoadPricingStoreFromSQL"]:
            if needle not in content:
                missing.append(f"{file_name} must load relay pricing from SQL")
    relay_config_code = (repo / "src/server/internal/relay/relay.go").read_text(encoding="utf-8")
    if "production relay requires configured pricing store" not in relay_config_code or "production relay requires active pricing entries" not in relay_config_code:
        missing.append("relay production construction must require a configured, non-empty pricing store")

    workflow_docs = read_yaml_stream(repo / "deploy/kubernetes/workflow-deployment.yaml")
    workflow_deployment = next((doc for doc in workflow_docs if doc.get("kind") == "Deployment"), {})
    workflow = container(workflow_deployment, "workflow")
    if not has_named_container_port(workflow, "http", 8082):
        missing.append("workflow deployment must expose named http port 8082")
    if not has_named_container_port(workflow, "grpc", 50064):
        missing.append("workflow deployment must expose named grpc port 50064")
    if env_value(workflow, "WORKFLOW_GRPC_PORT") != "50064":
        missing.append("workflow deployment must set WORKFLOW_GRPC_PORT=50064")
    if env_value(workflow, "GRPC_PORT") != "50064":
        missing.append("workflow deployment must set GRPC_PORT=50064 for generated clients")
    if env_value(workflow, "KAFKA_BROKERS") != "oblivious-kafka.oblivious.svc.cluster.local:9092":
        missing.append("workflow deployment must set KAFKA_BROKERS to the Kubernetes Kafka service")
    if env_value(workflow, "OBLIVIOUS_DB_MODE") != "microservices":
        missing.append("workflow deployment must set OBLIVIOUS_DB_MODE=microservices")
    if env_value(workflow, "DB_URL_WORKFLOW") != "__valueFrom__":
        missing.append("workflow deployment must read DB_URL_WORKFLOW from Secret")
    if not has_probe(workflow, "livenessProbe"):
        missing.append("workflow deployment must define livenessProbe")
    if not has_probe(workflow, "readinessProbe"):
        missing.append("workflow deployment must define readinessProbe")
    if not has_probe(workflow, "startupProbe"):
        missing.append("workflow deployment must define startupProbe")
    if not service_with_port(workflow_docs, "workflow", "grpc", 50064, "grpc"):
        missing.append("workflow service must expose grpc port 50064")

    observability_docs = read_yaml_stream(repo / "deploy/kubernetes/observability-deployment.yaml")
    observability_deployment = next((doc for doc in observability_docs if doc.get("kind") == "Deployment"), {})
    observability = container(observability_deployment, "observability")
    if env_value(observability, "OBLIVIOUS_DB_MODE") != "microservices":
        missing.append("observability deployment must set OBLIVIOUS_DB_MODE=microservices")
    if env_value(observability, "DB_URL_OBSERVABILITY") != "__valueFrom__":
        missing.append("observability deployment must read DB_URL_OBSERVABILITY from Secret")
    observability_config_file = repo / "src/server/pkg/config/observability.go"
    if not observability_config_file.exists():
        missing.append("pkg/config must include observability service config")
    else:
        observability_config = observability_config_file.read_text(encoding="utf-8")
        for needle in ["LoadObservabilityConfig", "DB_URL_OBSERVABILITY", "withServiceDatabaseURL"]:
            if needle not in observability_config:
                missing.append(f"observability config must include {needle}")
    observability_cmd = (repo / "src/server/cmd/observability/main.go").read_text(encoding="utf-8")
    for needle in ["LoadObservabilityConfig", "cfg.Port", "cfg.DBMode"]:
        if needle not in observability_cmd:
            missing.append(f"cmd/observability must consume observability service config for {needle}")

    task_docs = read_yaml_stream(repo / "deploy/kubernetes/task-deployment.yaml")
    task_deployment = next((doc for doc in task_docs if doc.get("kind") == "Deployment"), {})
    task = container(task_deployment, "task")
    if not has_named_container_port(task, "http", 8084):
        missing.append("task deployment must expose named http port 8084")
    if not has_named_container_port(task, "grpc", 50065):
        missing.append("task deployment must expose named grpc port 50065")
    if env_value(task, "TASK_GRPC_PORT") != "50065":
        missing.append("task deployment must set TASK_GRPC_PORT=50065")
    if env_value(task, "GRPC_PORT") != "50065":
        missing.append("task deployment must set GRPC_PORT=50065 for generated clients")
    if env_value(task, "KAFKA_BROKERS") != "oblivious-kafka.oblivious.svc.cluster.local:9092":
        missing.append("task deployment must set KAFKA_BROKERS to the Kubernetes Kafka service")
    if not has_probe(task, "livenessProbe"):
        missing.append("task deployment must define livenessProbe")
    if not has_probe(task, "readinessProbe"):
        missing.append("task deployment must define readinessProbe")
    if not has_probe(task, "startupProbe"):
        missing.append("task deployment must define startupProbe")
    if not service_with_port(task_docs, "task", "grpc", 50065, "grpc"):
        missing.append("task service must expose grpc port 50065")

    workflow_dockerfile = (repo / "deploy/docker/Dockerfile.workflow").read_text(encoding="utf-8")
    task_dockerfile = (repo / "deploy/docker/Dockerfile.task").read_text(encoding="utf-8")
    if "EXPOSE 8082 50064" not in workflow_dockerfile:
        missing.append("Dockerfile.workflow must expose HTTP and gRPC ports")
    if "EXPOSE 8084 50065" not in task_dockerfile:
        missing.append("Dockerfile.task must expose HTTP and gRPC ports")

    compose = read_yaml(repo / "docker-compose.yml")
    compose_agent = dig(compose, "services", "agent") or {}
    compose_env = [str(item) for item in compose_agent.get("environment") or []]
    if "microservices" not in (compose_agent.get("profiles") or []):
        missing.append("docker compose microservices profile must include agent service")
    if dig(compose_agent, "build", "dockerfile") != "deploy/docker/Dockerfile.agent":
        missing.append("docker compose agent must build Dockerfile.agent")
    if not any(":50063" in str(value) for value in compose_agent.get("ports") or []):
        missing.append("docker compose agent must publish gRPC port 50063")
    if "AGENT_GRPC_PORT=50063" not in compose_env:
        missing.append("docker compose agent must set AGENT_GRPC_PORT=50063")
    if not any(value.startswith("AGENT_RELAY_BASE_URL=") for value in compose_env):
        missing.append("docker compose agent must set AGENT_RELAY_BASE_URL")
    compose_workflow = dig(compose, "services", "workflow") or {}
    workflow_env = [str(item) for item in compose_workflow.get("environment") or []]
    if dig(compose_workflow, "build", "dockerfile") != "deploy/docker/Dockerfile.workflow":
        missing.append("docker compose workflow must build Dockerfile.workflow")
    if not depends_on_service(compose_workflow, "kafka"):
        missing.append("docker compose workflow must depend on kafka")
    if not any(":50064" in str(value) for value in compose_workflow.get("ports") or []):
        missing.append("docker compose workflow must publish gRPC port 50064")
    if "WORKFLOW_GRPC_PORT=50064" not in workflow_env:
        missing.append("docker compose workflow must set WORKFLOW_GRPC_PORT=50064")
    if "KAFKA_BROKERS=kafka:9092" not in workflow_env:
        missing.append("docker compose workflow must set KAFKA_BROKERS=kafka:9092")
    compose_task = dig(compose, "services", "task") or {}
    task_env = [str(item) for item in compose_task.get("environment") or []]
    if "microservices" not in (compose_task.get("profiles") or []):
        missing.append("docker compose microservices profile must include task service")
    if dig(compose_task, "build", "dockerfile") != "deploy/docker/Dockerfile.task":
        missing.append("docker compose task must build Dockerfile.task")
    if not depends_on_service(compose_task, "kafka"):
        missing.append("docker compose task must depend on kafka")
    if not any(":50065" in str(value) for value in compose_task.get("ports") or []):
        missing.append("docker compose task must publish gRPC port 50065")
    if "TASK_GRPC_PORT=50065" not in task_env:
        missing.append("docker compose task must set TASK_GRPC_PORT=50065")
    if "KAFKA_BROKERS=kafka:9092" not in task_env:
        missing.append("docker compose task must set KAFKA_BROKERS=kafka:9092")
    compose_kafka = dig(compose, "services", "kafka") or {}
    kafka_profiles = compose_kafka.get("profiles") or []
    if kafka_profiles and "microservices" not in kafka_profiles:
        missing.append("docker compose kafka must be available in the microservices profile")
    compose_web = dig(compose, "services", "oblivious-web") or {}
    if not any(":8080" in str(value) for value in compose_web.get("ports") or []):
        missing.append("docker compose web must publish container port 8080")

    release_workloads = [path for path in tracked_k8s_files if path.endswith("-deployment.yaml")]
    if release_workloads.count("deploy/kubernetes/app-deployment.yaml") != 1:
        missing.append("app-deployment.yaml must be the sole canonical application workload")
    if "deploy/kubernetes/server.yaml" in release_workloads:
        missing.append("server.yaml must not enter release validation inventory")

    alerts = read_yaml(repo / "deploy/observability/prometheus-alerts.yaml")
    alert_names = [rule.get("alert") for group in alerts.get("groups") or [] for rule in group.get("rules") or []]
    for alert in ["RelayOutage", "QuotaSettlementFailure", "StripeWebhookFailure", "MigrationFailure", "HighProviderErrorRate", "TenantIsolationIncident", "WorkflowExecutionFailureRate", "RAGRetrievalSlowness", "AgentRunFailureRate", "AgentToolCallFailureRate"]:
        if alert not in alert_names:
            missing.append(f"prometheus alerts must include {alert}")

    dashboard = json.loads((repo / "deploy/observability/grafana-dashboard.json").read_text(encoding="utf-8"))
    panel_text = "\n".join(str(panel.get("title") or "") for panel in dashboard.get("panels") or [])
    for needle in ["Relay", "Workflow", "RAG", "Agent"]:
        if needle.lower() not in panel_text.lower():
            missing.append(f"grafana dashboard must include {needle} panels")

    docs = {
        "backup-restore-runbook.md": ["pg_dump", "pg_restore", "schema_migrations", "retention", "encryption"],
        "observability-slos.md": ["RelayOutage", "QuotaSettlementFailure", "TenantIsolationIncident", "AgentRunFailureRate"],
        "release-rollback-runbook.md": ["deploy-validate", "deploy-smoke", "backup-postgres", "restore-postgres", "rollback"],
        "incident-response-runbook.md": ["rollback", "disaster", "recovery", "tenant"],
        "disaster-recovery-runbook.md": ["backup-restore-smoke", "restore-postgres", "no-final-readiness"],
    }
    for file_name, needles in docs.items():
        content = (repo / "docs/release" / file_name).read_text(encoding="utf-8")
        for needle in needles:
            if needle not in content:
                missing.append(f"{file_name} must mention {needle}")

    scripts = {
        "scripts/deploy-validate.sh": ["docker compose", "deploy-smoke"],
        "scripts/k8s-validate.sh": ["OBLIVIOUS_K8S_SECRET_FILE", "kubectl", "secret.example.yaml"],
        "scripts/backup-postgres.sh": ["pg_dump", "--format=custom", "sha256sum"],
        "scripts/restore-postgres.sh": ["pg_restore", "schema_migrations", "sha256sum"],
        "scripts/backup-restore-smoke.sh": ["backup-postgres.sh", "restore-postgres.sh", "schema_migrations"],
    }
    for file_name, needles in scripts.items():
        content = (repo / file_name).read_text(encoding="utf-8")
        for needle in needles:
            if needle not in content:
                missing.append(f"{file_name} must mention {needle}")

    if missing:
        print("[deployment-operations-contract] incomplete deployment/operations contract:", file=sys.stderr)
        for entry in missing:
            print(f"  - {entry}", file=sys.stderr)
        return 1
    print("[deployment-operations-contract] deployment manifests, recovery scripts, and operations docs verified.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
