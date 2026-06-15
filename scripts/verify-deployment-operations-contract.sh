#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

ruby -ryaml -rjson -e '
  repo = ARGV.fetch(0)
  missing = []

  def read_yaml(path)
    YAML.load_file(path)
  end

  def read_yaml_stream(path)
    YAML.load_stream(File.read(path)).compact
  end

  def deployment_doc(path)
    docs = read_yaml_stream(path)
    docs.find { |doc| doc["kind"] == "Deployment" } || docs.first || {}
  end

  def container(doc, name)
    doc.dig("spec", "template", "spec", "containers").to_a.find { |entry| entry["name"] == name } || {}
  end

  def first_container(doc)
    doc.dig("spec", "template", "spec", "containers").to_a.first || {}
  end

  def has_resources?(ctr)
    requests = ctr.dig("resources", "requests") || {}
    limits = ctr.dig("resources", "limits") || {}
    requests["cpu"].to_s != "" &&
      requests["memory"].to_s != "" &&
      limits["cpu"].to_s != "" &&
      limits["memory"].to_s != ""
  end

  def has_probe?(ctr, probe)
    path = ctr.dig(probe, "httpGet", "path")
    port = ctr.dig(probe, "httpGet", "port")
    exec_command = ctr.dig(probe, "exec", "command")
    (path.to_s != "" && port.to_s != "") || (exec_command.is_a?(Array) && !exec_command.empty?)
  end

  def has_env_ref?(ctr, ref_kind, name)
    ctr.fetch("envFrom", []).any? { |entry| entry.dig(ref_kind, "name") == name }
  end

  def env_value(ctr, name)
    entry = ctr.fetch("env", []).find { |item| item["name"] == name }
    return "" if entry.nil?
    return "__valueFrom__" if entry.key?("valueFrom")
    entry["value"].to_s
  end

  def has_named_container_port?(ctr, name, port)
    ctr.fetch("ports", []).any? { |entry| entry["name"] == name && entry["containerPort"].to_i == port }
  end

  def service_with_port?(docs, service_name, port_name, port, target_port)
    service = docs.find { |doc| doc["kind"] == "Service" && doc.dig("metadata", "name") == service_name }
    return false if service.nil?
    service.dig("spec", "ports").to_a.any? do |entry|
      entry["name"] == port_name &&
        entry["port"].to_i == port &&
        entry["targetPort"].to_s == target_port.to_s
    end
  end

  def depends_on_service?(service, dependency)
    depends_on = service["depends_on"]
    case depends_on
    when Hash
      depends_on.key?(dependency)
    when Array
      depends_on.include?(dependency)
    else
      false
    end
  end

  def each_node(value, path = [], &block)
    yield value, path
    case value
    when Hash
      value.each { |key, child| each_node(child, path + [key], &block) }
    when Array
      value.each_with_index { |child, index| each_node(child, path + [index], &block) }
    end
  end

  def path_label(path)
    path.map(&:to_s).join(".")
  end

  app_deployment = read_yaml(File.join(repo, "deploy/kubernetes/app-deployment.yaml"))
  server = container(app_deployment, "server")
  missing << "app deployment must use RollingUpdate maxUnavailable=0" unless app_deployment.dig("spec", "strategy", "rollingUpdate", "maxUnavailable").to_s == "0"
  missing << "app deployment server must define CPU/memory requests and limits" unless has_resources?(server)
  missing << "app deployment server must define livenessProbe" unless has_probe?(server, "livenessProbe")
  missing << "app deployment server must define readinessProbe" unless has_probe?(server, "readinessProbe")
  missing << "app deployment server must define startupProbe" unless has_probe?(server, "startupProbe")
  missing << "app deployment server must load oblivious-config ConfigMap" unless has_env_ref?(server, "configMapRef", "oblivious-config")
  missing << "app deployment server must load oblivious-secrets Secret" unless has_env_ref?(server, "secretRef", "oblivious-secrets")

  config_map = read_yaml(File.join(repo, "deploy/kubernetes/configmap.yaml"))
  missing << "configmap must set production APP_ENV" unless config_map.dig("data", "APP_ENV") == "production"
  missing << "configmap must enable Redis relay rate limiting" unless config_map.dig("data", "RELAY_RATE_LIMIT_BACKEND") == "redis"
  missing << "configmap must document relay default model" unless config_map.dig("data", "RELAY_DEFAULT_MODEL").to_s != ""
  missing << "configmap must document relay semantic cache backend" unless config_map.dig("data", "RELAY_SEMANTIC_CACHE_BACKEND").to_s != ""
  missing << "configmap must enable secure session cookies" unless config_map.dig("data", "SESSION_COOKIE_SECURE") == "true"

  secret_example = read_yaml(File.join(repo, "deploy/kubernetes/secret.example.yaml"))
  missing << "secret example must define oblivious-secrets" unless secret_example.dig("metadata", "name") == "oblivious-secrets"
  secret_data = secret_example["stringData"] || {}
  %w[DATABASE_URL SESSION_SECRET POSTGRES_PASSWORD LLM_API_KEY OPENAI_API_KEY DB_URL_CHAT DB_URL_MARKETPLACE DB_URL_OBSERVABILITY].each do |key|
    missing << "secret example must document #{key}" unless secret_data.key?(key)
  end
  leaked_secret_keys = secret_data.select { |_key, value| value.to_s != "" && !value.to_s.include?("REPLACE_ME") }.keys
  missing << "secret example must keep non-empty values as REPLACE_ME placeholders: #{leaked_secret_keys.join(", ")}" unless leaked_secret_keys.empty?

  tracked_k8s_files = IO.popen(["git", "-C", repo, "ls-files", "deploy/kubernetes/*.yaml"], &:read).lines.map(&:strip).reject(&:empty?)
  config_maps = {}
  secrets = { "oblivious-secrets" => secret_data.keys }
  tracked_k8s_files.each do |relative_path|
    read_yaml_stream(File.join(repo, relative_path)).each do |doc|
      name = doc.dig("metadata", "name").to_s
      case doc["kind"]
      when "ConfigMap"
        config_maps[name] = (doc["data"] || {}).keys
      when "Secret"
        secrets[name] = ((doc["stringData"] || {}).keys + (doc["data"] || {}).keys).uniq
      end
    end
  end

  tracked_k8s_files.each do |relative_path|
    read_yaml_stream(File.join(repo, relative_path)).each do |doc|
      each_node(doc) do |node, path|
        next unless node.is_a?(Hash)

        if (secret_ref = node["secretKeyRef"]).is_a?(Hash)
          ref_name = secret_ref["name"].to_s
          ref_key = secret_ref["key"].to_s
          missing << "#{relative_path} #{path_label(path)} secretKeyRef must name a tracked Secret" unless secrets.key?(ref_name)
          if secrets.key?(ref_name) && !secrets.fetch(ref_name).include?(ref_key)
            missing << "#{relative_path} #{path_label(path)} secretKeyRef #{ref_name}/#{ref_key} must be documented in secret.example.yaml"
          end
        end

        if (config_ref = node["configMapKeyRef"]).is_a?(Hash)
          ref_name = config_ref["name"].to_s
          ref_key = config_ref["key"].to_s
          missing << "#{relative_path} #{path_label(path)} configMapKeyRef must name a tracked ConfigMap" unless config_maps.key?(ref_name)
          if config_maps.key?(ref_name) && !config_maps.fetch(ref_name).include?(ref_key)
            missing << "#{relative_path} #{path_label(path)} configMapKeyRef #{ref_name}/#{ref_key} must be documented in configmap.yaml"
          end
        end

        if (secret_ref = node["secretRef"]).is_a?(Hash)
          ref_name = secret_ref["name"].to_s
          missing << "#{relative_path} #{path_label(path)} secretRef must name a tracked Secret" unless secrets.key?(ref_name)
        end

        if (config_ref = node["configMapRef"]).is_a?(Hash)
          ref_name = config_ref["name"].to_s
          missing << "#{relative_path} #{path_label(path)} configMapRef must name a tracked ConfigMap" unless config_maps.key?(ref_name)
        end
      end

      if doc["kind"] == "Ingress"
        annotations = doc.dig("metadata", "annotations") || {}
        cert_manager_managed = annotations.key?("cert-manager.io/cluster-issuer")
        doc.dig("spec", "tls").to_a.each do |tls|
          tls_secret = tls["secretName"].to_s
          next if tls_secret == "" || cert_manager_managed || secrets.key?(tls_secret)
          missing << "#{relative_path} Ingress TLS secret #{tls_secret} must be tracked or cert-manager managed"
        end
      end
    end
  end

  hpa = read_yaml(File.join(repo, "deploy/kubernetes/hpa.yaml"))
  metrics = hpa.dig("spec", "metrics").to_a
  missing << "HPA must target oblivious-server deployment" unless hpa.dig("spec", "scaleTargetRef", "name") == "oblivious-server"
  missing << "HPA must keep at least three replicas" unless hpa.dig("spec", "minReplicas").to_i >= 3
  missing << "HPA must include CPU utilization metric" unless metrics.any? { |metric| metric.dig("resource", "name") == "cpu" }
  missing << "HPA must include memory utilization metric" unless metrics.any? { |metric| metric.dig("resource", "name") == "memory" }
  missing << "HPA must include workflow queue backlog metric" unless metrics.any? { |metric| metric.dig("external", "metric", "name") == "workflow_queue_backlog" }

  %w[postgres qdrant redis].each do |component|
    doc = deployment_doc(File.join(repo, "deploy/kubernetes/#{component}.yaml"))
    ctr = container(doc, component)
    missing << "#{component} deployment must define CPU/memory requests" unless ctr.dig("resources", "requests", "cpu").to_s != "" && ctr.dig("resources", "requests", "memory").to_s != ""
    missing << "#{component} deployment must define CPU/memory limits" unless ctr.dig("resources", "limits", "cpu").to_s != "" && ctr.dig("resources", "limits", "memory").to_s != ""
    missing << "#{component} deployment must define readinessProbe" unless has_probe?(ctr, "readinessProbe")
  end

  web_deployment = deployment_doc(File.join(repo, "deploy/kubernetes/web.yaml"))
  web = container(web_deployment, "web")
  missing << "web deployment must define CPU/memory requests and limits" unless has_resources?(web)
  missing << "web deployment must define livenessProbe" unless has_probe?(web, "livenessProbe")
  missing << "web deployment must define readinessProbe" unless has_probe?(web, "readinessProbe")

  agent_docs = read_yaml_stream(File.join(repo, "deploy/kubernetes/agent-deployment.yaml"))
  agent_deployment = agent_docs.find { |doc| doc["kind"] == "Deployment" } || {}
  agent = container(agent_deployment, "agent")
  missing << "agent deployment must expose named http port 8083" unless has_named_container_port?(agent, "http", 8083)
  missing << "agent deployment must expose named grpc port 50063" unless has_named_container_port?(agent, "grpc", 50063)
  missing << "agent deployment must set AGENT_GRPC_PORT=50063" unless env_value(agent, "AGENT_GRPC_PORT") == "50063"
  missing << "agent deployment must set GRPC_PORT=50063 for generated clients" unless env_value(agent, "GRPC_PORT") == "50063"
  missing << "agent deployment must configure AGENT_RELAY_BASE_URL" unless env_value(agent, "AGENT_RELAY_BASE_URL") != ""
  missing << "agent deployment must read DATABASE_URL from Secret" unless env_value(agent, "DATABASE_URL") == "__valueFrom__"
  missing << "agent deployment must define livenessProbe" unless has_probe?(agent, "livenessProbe")
  missing << "agent deployment must define readinessProbe" unless has_probe?(agent, "readinessProbe")
  missing << "agent deployment must define startupProbe" unless has_probe?(agent, "startupProbe")
  missing << "agent service must expose grpc port 50063" unless service_with_port?(agent_docs, "agent", "grpc", 50063, "grpc")

  agent_dockerfile = File.read(File.join(repo, "deploy/docker/Dockerfile.agent"))
  missing << "Dockerfile.agent must expose HTTP and gRPC ports" unless agent_dockerfile.include?("EXPOSE 8083 50063")

  workflow_docs = read_yaml_stream(File.join(repo, "deploy/kubernetes/workflow-deployment.yaml"))
  workflow_deployment = workflow_docs.find { |doc| doc["kind"] == "Deployment" } || {}
  workflow = container(workflow_deployment, "workflow")
  missing << "workflow deployment must expose named http port 8082" unless has_named_container_port?(workflow, "http", 8082)
  missing << "workflow deployment must expose named grpc port 50064" unless has_named_container_port?(workflow, "grpc", 50064)
  missing << "workflow deployment must set WORKFLOW_GRPC_PORT=50064" unless env_value(workflow, "WORKFLOW_GRPC_PORT") == "50064"
  missing << "workflow deployment must set GRPC_PORT=50064 for generated clients" unless env_value(workflow, "GRPC_PORT") == "50064"
  missing << "workflow deployment must set KAFKA_BROKERS to the Kubernetes Kafka service" unless env_value(workflow, "KAFKA_BROKERS") == "oblivious-kafka.oblivious.svc.cluster.local:9092"
  missing << "workflow deployment must define livenessProbe" unless has_probe?(workflow, "livenessProbe")
  missing << "workflow deployment must define readinessProbe" unless has_probe?(workflow, "readinessProbe")
  missing << "workflow deployment must define startupProbe" unless has_probe?(workflow, "startupProbe")
  missing << "workflow service must expose grpc port 50064" unless service_with_port?(workflow_docs, "workflow", "grpc", 50064, "grpc")

  task_docs = read_yaml_stream(File.join(repo, "deploy/kubernetes/task-deployment.yaml"))
  task_deployment = task_docs.find { |doc| doc["kind"] == "Deployment" } || {}
  task = container(task_deployment, "task")
  missing << "task deployment must expose named http port 8084" unless has_named_container_port?(task, "http", 8084)
  missing << "task deployment must expose named grpc port 50065" unless has_named_container_port?(task, "grpc", 50065)
  missing << "task deployment must set TASK_GRPC_PORT=50065" unless env_value(task, "TASK_GRPC_PORT") == "50065"
  missing << "task deployment must set GRPC_PORT=50065 for generated clients" unless env_value(task, "GRPC_PORT") == "50065"
  missing << "task deployment must set KAFKA_BROKERS to the Kubernetes Kafka service" unless env_value(task, "KAFKA_BROKERS") == "oblivious-kafka.oblivious.svc.cluster.local:9092"
  missing << "task deployment must define livenessProbe" unless has_probe?(task, "livenessProbe")
  missing << "task deployment must define readinessProbe" unless has_probe?(task, "readinessProbe")
  missing << "task deployment must define startupProbe" unless has_probe?(task, "startupProbe")
  missing << "task service must expose grpc port 50065" unless service_with_port?(task_docs, "task", "grpc", 50065, "grpc")

  workflow_dockerfile = File.read(File.join(repo, "deploy/docker/Dockerfile.workflow"))
  task_dockerfile = File.read(File.join(repo, "deploy/docker/Dockerfile.task"))
  missing << "Dockerfile.workflow must expose HTTP and gRPC ports" unless workflow_dockerfile.include?("EXPOSE 8082 50064")
  missing << "Dockerfile.task must expose HTTP and gRPC ports" unless task_dockerfile.include?("EXPOSE 8084 50065")

  compose = read_yaml(File.join(repo, "docker-compose.yml"))
  compose_agent = compose.dig("services", "agent") || {}
  compose_env = compose_agent.fetch("environment", []).map(&:to_s)
  missing << "docker compose microservices profile must include agent service" unless compose_agent.fetch("profiles", []).include?("microservices")
  missing << "docker compose agent must build Dockerfile.agent" unless compose_agent.dig("build", "dockerfile") == "deploy/docker/Dockerfile.agent"
  missing << "docker compose agent must publish gRPC port 50063" unless compose_agent.fetch("ports", []).map(&:to_s).any? { |value| value.include?(":50063") }
  missing << "docker compose agent must set AGENT_GRPC_PORT=50063" unless compose_env.include?("AGENT_GRPC_PORT=50063")
  missing << "docker compose agent must set AGENT_RELAY_BASE_URL" unless compose_env.any? { |value| value.start_with?("AGENT_RELAY_BASE_URL=") }
  compose_workflow = compose.dig("services", "workflow") || {}
  workflow_env = compose_workflow.fetch("environment", []).map(&:to_s)
  missing << "docker compose workflow must build Dockerfile.workflow" unless compose_workflow.dig("build", "dockerfile") == "deploy/docker/Dockerfile.workflow"
  missing << "docker compose workflow must depend on kafka" unless depends_on_service?(compose_workflow, "kafka")
  missing << "docker compose workflow must publish gRPC port 50064" unless compose_workflow.fetch("ports", []).map(&:to_s).any? { |value| value.include?(":50064") }
  missing << "docker compose workflow must set WORKFLOW_GRPC_PORT=50064" unless workflow_env.include?("WORKFLOW_GRPC_PORT=50064")
  missing << "docker compose workflow must set KAFKA_BROKERS=kafka:9092" unless workflow_env.include?("KAFKA_BROKERS=kafka:9092")
  compose_task = compose.dig("services", "task") || {}
  task_env = compose_task.fetch("environment", []).map(&:to_s)
  missing << "docker compose microservices profile must include task service" unless compose_task.fetch("profiles", []).include?("microservices")
  missing << "docker compose task must build Dockerfile.task" unless compose_task.dig("build", "dockerfile") == "deploy/docker/Dockerfile.task"
  missing << "docker compose task must depend on kafka" unless depends_on_service?(compose_task, "kafka")
  missing << "docker compose task must publish gRPC port 50065" unless compose_task.fetch("ports", []).map(&:to_s).any? { |value| value.include?(":50065") }
  missing << "docker compose task must set TASK_GRPC_PORT=50065" unless task_env.include?("TASK_GRPC_PORT=50065")
  missing << "docker compose task must set KAFKA_BROKERS=kafka:9092" unless task_env.include?("KAFKA_BROKERS=kafka:9092")
  compose_kafka = compose.dig("services", "kafka") || {}
  kafka_profiles = compose_kafka.fetch("profiles", [])
  missing << "docker compose kafka must be available in the microservices profile" unless kafka_profiles.empty? || kafka_profiles.include?("microservices")

  local_server_deployment = deployment_doc(File.join(repo, "deploy/kubernetes/server.yaml"))
  local_server = first_container(local_server_deployment)
  missing << "local server deployment must define CPU/memory requests and limits" unless has_resources?(local_server)

  alerts = read_yaml(File.join(repo, "deploy/observability/prometheus-alerts.yaml"))
  alert_names = alerts.fetch("groups", []).flat_map { |group| group.fetch("rules", []).map { |rule| rule["alert"] } }
  %w[RelayOutage QuotaSettlementFailure StripeWebhookFailure MigrationFailure HighProviderErrorRate TenantIsolationIncident WorkflowExecutionFailureRate RAGRetrievalSlowness AgentRunFailureRate AgentToolCallFailureRate].each do |alert|
    missing << "prometheus alerts must include #{alert}" unless alert_names.include?(alert)
  end

  dashboard = JSON.parse(File.read(File.join(repo, "deploy/observability/grafana-dashboard.json")))
  panel_text = dashboard.fetch("panels", []).map { |panel| panel["title"].to_s }.join("\n")
  %w[Relay Workflow RAG Agent].each do |needle|
    missing << "grafana dashboard must include #{needle} panels" unless panel_text.downcase.include?(needle.downcase)
  end

  docs = {
    "backup-restore-runbook.md" => %w[pg_dump pg_restore schema_migrations retention encryption],
    "observability-slos.md" => %w[RelayOutage QuotaSettlementFailure TenantIsolationIncident AgentRunFailureRate],
    "release-rollback-runbook.md" => %w[deploy-validate deploy-smoke backup-postgres restore-postgres rollback],
    "incident-response-runbook.md" => %w[rollback disaster recovery tenant],
    "disaster-recovery-runbook.md" => %w[backup-restore-smoke restore-postgres no-final-readiness],
  }
  docs.each do |file, needles|
    content = File.read(File.join(repo, "docs/release", file))
    needles.each do |needle|
      missing << "#{file} must mention #{needle}" unless content.include?(needle)
    end
  end

  scripts = {
    "scripts/deploy-validate.sh" => %w[docker compose deploy-smoke],
    "scripts/k8s-validate.sh" => %w[OBLIVIOUS_K8S_SECRET_FILE kubectl secret.example.yaml],
    "scripts/backup-postgres.sh" => %w[pg_dump --format=custom sha256sum],
    "scripts/restore-postgres.sh" => %w[pg_restore schema_migrations sha256sum],
    "scripts/backup-restore-smoke.sh" => %w[backup-postgres.sh restore-postgres.sh schema_migrations],
  }
  scripts.each do |file, needles|
    content = File.read(File.join(repo, file))
    needles.each do |needle|
      missing << "#{file} must mention #{needle}" unless content.include?(needle)
    end
  end

  unless missing.empty?
    warn "[deployment-operations-contract] incomplete deployment/operations contract:"
    missing.each { |entry| warn "  - #{entry}" }
    exit 1
  end
  puts "[deployment-operations-contract] deployment manifests, recovery scripts, and operations docs verified."
' "$repo_root"
