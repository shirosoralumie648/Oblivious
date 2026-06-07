#!/usr/bin/env node

import fs from 'node:fs';

const dashboardPath = new URL('../deploy/observability/grafana-dashboard.json', import.meta.url);
const dashboard = JSON.parse(fs.readFileSync(dashboardPath, 'utf8'));
const panels = Array.isArray(dashboard.panels) ? dashboard.panels : [];

const panelText = panels
  .map((panel) => {
    const targetText = Array.isArray(panel.targets)
      ? panel.targets.map((target) => `${target.expr ?? ''} ${target.query ?? ''} ${target.rawSql ?? ''}`).join(' ')
      : '';
    return `${panel.title ?? ''} ${panel.description ?? ''} ${targetText}`;
  })
  .join('\n')
  .toLowerCase();

const required = [
  'model usage',
  'feature usage',
  'top user cost',
  'usage time trend',
  'model x time',
  'user x feature',
  'feature x time',
  'request_logs',
  'cost_usd',
  'request_tokens + response_tokens',
  'workflow active executions',
  'workflow active execution age',
  'workflow execution total',
  'workflow execution duration',
  'workflow node error rate',
  'workflow_execution_active',
  'workflow_execution_active_age_seconds',
  'workflow_execution_total',
  'workflow_execution_duration_seconds',
  'workflow_node_error_rate',
  'rag retrieval latency',
  'rag document processing duration',
  'rag chunk count',
  'rag_retrieval_latency_seconds',
  'rag_document_processing_duration_seconds',
  'rag_chunk_count',
  'agent run total',
  'agent tool call total',
  'agent iteration count',
  'agent_run_total',
  'agent_tool_call_total',
  'agent_iteration_count',
];

const missing = required.filter((needle) => !panelText.includes(needle.toLowerCase()));
if (missing.length > 0) {
  console.error(`Grafana dashboard is missing usage analytics or workflow observability coverage: ${missing.join(', ')}`);
  process.exit(1);
}

console.log('Grafana dashboard usage analytics and workflow observability coverage verified.');
