import { useEffect, useMemo, useState, type DragEvent } from 'react';
import { RiCodeBoxLine, RiKey2Line, RiShieldKeyholeLine, RiWebhookLine } from '@remixicon/react';
import {
  Background,
  Controls,
  MarkerType,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  type Connection,
  type Edge as ReactFlowEdge,
  type Node as ReactFlowNode,
  type NodeProps,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import { createScheduledTasksApi, type ScheduledTask } from '../../features/scheduledTasks/scheduledTasksApi';
import type { ScheduledTaskRun } from '../../types/api';
import {
  createWorkflowsApi,
  type CheckWorkflowResourceLimitsRequest,
  type CreateWorkflowBranchRequest,
  type ResumeWorkflowExecutionRequest,
  type ResolvePausedFailureRequest,
  type WorkflowDefinition,
  type WorkflowExecutionDebugSnapshot,
  type WorkflowExecution,
  type WorkflowExecutionStatus,
  type WorkflowNodeExecution,
  type WorkflowNodeTestResult,
  type ConversationTriggerMatch,
  type SemanticTriggerMatch,
} from '../../features/workflows/workflowsApi';
import { createHttpClient } from '../../services/http/client';

const manualDraftDefinition = {
  nodes: [{ id: 'manual-start', type: 'manual' }],
};

type NodeDebugDraft = {
  nodeId: string;
  inputText: string;
};

type NodeFailurePolicyDraft = {
  failureBranchNodeId: string;
  maxRetries: string;
  retryDelaysText: string;
};

type ResourceCheckDraft = {
  nodeExecutionCount: string;
  totalTokens: string;
};

type WorkflowResourcePolicyDraft = {
  concurrencyOverflow: 'queue' | 'reject';
  maxConcurrentExecutions: string;
  maxExecutionDurationSeconds: string;
  maxNodeExecutions: string;
  maxTokensBudget: string;
};

type PausedFailureDecisionDraft = {
  inputText: string;
  nextNodeId: string;
};

type PausedInputDraft = {
  inputText: string;
};

type ConversationTriggerDraft = {
  conversationId: string;
  id: string;
};

type ScheduleTriggerDraft = {
  cron: string;
  enabled: boolean;
  id: string;
};

type SemanticTriggerDraft = {
  id: string;
  keywordsText: string;
  threshold: string;
};

type WebhookTriggerDraft = {
  id: string;
  path: string;
  secret: string;
};

type BranchDraft = {
  description: string;
  experimentKey: string;
  name: string;
  trafficPercent: string;
};

type WorkflowDefinitionDraft = {
  edgeBranch: string;
  edgeSource: string;
  edgeTarget: string;
  nodeId: string;
  nodeInputText: string;
  nodeType: string;
};

type WorkflowCanvasPosition = {
  x: number;
  y: number;
};

type VisualWorkflowNode = {
  failureBranchNodeId: string;
  failurePolicy: Record<string, unknown>;
  failureStrategy: WorkflowNodeFailureStrategy;
  id: string;
  input?: unknown;
  maxRetries?: number;
  position?: WorkflowCanvasPosition;
  retryDelaysText: string;
  status: string;
  type: string;
};

type VisualWorkflowEdge = {
  branch?: string;
  id: string;
  index: number;
  source: string;
  status: 'active' | 'invalid';
  target: string;
};

type WorkflowReactFlowNodeData = Record<string, unknown> & {
  hasNodeExecution: boolean;
  index: number;
  node: VisualWorkflowNode;
  nodeStatus: string;
  onSelect: (workflowId: string, node: VisualWorkflowNode) => void;
  position: WorkflowCanvasPosition;
  statusBadgeClass: string;
  workflowId: string;
};

type WorkflowReactFlowNode = ReactFlowNode<WorkflowReactFlowNodeData, 'workflowNode'>;

type WorkflowNodeFailureStrategy = 'auto_retry' | 'failure_branch' | 'pause_on_failure' | 'skip_on_failure';
type StructuredTriggerDraftKind = 'conversation' | 'schedule' | 'semantic' | 'webhook';
type WorkflowTriggerKind = 'conversation' | 'schedule' | 'semantic' | 'webhook';

const workflowNodeFailureStrategies: Array<{ label: string; value: WorkflowNodeFailureStrategy }> = [
  { label: 'Pause on failure', value: 'pause_on_failure' },
  { label: 'Auto retry', value: 'auto_retry' },
  { label: 'Skip on failure', value: 'skip_on_failure' },
  { label: 'Failure branch', value: 'failure_branch' },
];

const workflowTriggerKinds: Array<{ label: string; value: WorkflowTriggerKind }> = [
  { label: 'Conversation', value: 'conversation' },
  { label: 'Schedule', value: 'schedule' },
  { label: 'Semantic', value: 'semantic' },
  { label: 'Webhook', value: 'webhook' },
];

const workflowCanvasGridSize = 20;
const workflowCanvasNodeWidth = 176;
const workflowCanvasNodeHeight = 92;
const workflowCanvasLayoutOrigin: WorkflowCanvasPosition = { x: 80, y: 80 };
const workflowCanvasColumnGap = 240;
const workflowCanvasRowGap = 140;
const workflowReactFlowSnapGrid: [number, number] = [workflowCanvasGridSize, workflowCanvasGridSize];

function WorkflowReactFlowNodeComponent({ data, selected }: NodeProps<WorkflowReactFlowNode>) {
  const nodeCardClass = visualNodeCardClass(data.nodeStatus, data.hasNodeExecution);
  return (
    <button
      aria-label={`Canvas node ${data.index + 1} ${data.node.id} ${data.node.type} at ${data.position.x} ${data.position.y}`}
      aria-pressed={selected}
      className={`flex flex-col items-start rounded-lg border p-3 text-left shadow-sm transition ${
        selected ? `${nodeCardClass} ring-2 ring-[#1a614f] ring-offset-1` : nodeCardClass
      }`}
      onClick={(event) => {
        event.stopPropagation();
        data.onSelect(data.workflowId, data.node);
      }}
      style={{
        height: workflowCanvasNodeHeight,
        width: workflowCanvasNodeWidth,
      }}
      type="button"
    >
      <span className="text-[11px] font-semibold uppercase tracking-wide text-[#6d6658]">
        Node {data.index + 1}
      </span>
      <span className="mt-1 w-full truncate font-mono text-sm font-semibold text-[#181611]">
        {data.node.id}
      </span>
      <span className="mt-2 flex w-full flex-wrap gap-1 text-[11px] font-semibold">
        <span className="rounded-md border border-[#d7d2c4] bg-white px-1.5 py-0.5 text-[#625b4f]">
          {data.node.type}
        </span>
        <span className={`rounded-md border px-1.5 py-0.5 ${data.statusBadgeClass}`}>
          {data.nodeStatus}
        </span>
      </span>
    </button>
  );
}

const workflowReactFlowNodeTypes = {
  workflowNode: WorkflowReactFlowNodeComponent,
};

const workflowNodePalette = [
  { label: 'Start', type: 'start' },
  { label: 'End', type: 'end' },
  { label: 'Manual', type: 'manual' },
  { label: 'Trigger', type: 'trigger' },
  { label: 'Condition', type: 'condition' },
  { label: 'Loop', type: 'loop' },
  { label: 'Join', type: 'join' },
  { label: 'Code', type: 'code' },
  { label: 'HTTP', type: 'http' },
  { label: 'LLM', type: 'llm' },
  { label: 'Knowledge', type: 'knowledge' },
  { label: 'User input', type: 'user_input' },
  { label: 'Approval', type: 'approval' },
  { label: 'Agent', type: 'agent' },
  { label: 'Tool', type: 'tool' },
  { label: 'Database', type: 'database' },
  { label: 'RPA', type: 'rpa' },
  { label: 'Transform', type: 'transform' },
  { label: 'Router', type: 'router' },
  { label: 'Notification', type: 'notification' },
  { label: 'Delay', type: 'delay' },
  { label: 'Webhook', type: 'webhook' },
];

const workflowNodeDragDataType = 'application/x-workflow-node-type';
const workflowDefinitionNodeTypes = workflowNodePalette.map((nodeType) => nodeType.type);

const emptyDebugDraft: NodeDebugDraft = {
  inputText: '{}',
  nodeId: '',
};

const emptyNodeFailurePolicyDraft: NodeFailurePolicyDraft = {
  failureBranchNodeId: '',
  maxRetries: '',
  retryDelaysText: '',
};

const emptyResourceCheckDraft: ResourceCheckDraft = {
  nodeExecutionCount: '',
  totalTokens: '',
};

const emptyWorkflowResourcePolicyDraft: WorkflowResourcePolicyDraft = {
  concurrencyOverflow: 'queue',
  maxConcurrentExecutions: '',
  maxExecutionDurationSeconds: '',
  maxNodeExecutions: '',
  maxTokensBudget: '',
};

const emptyPausedFailureDecisionDraft: PausedFailureDecisionDraft = {
  inputText: '{}',
  nextNodeId: '',
};

const emptyPausedInputDraft: PausedInputDraft = {
  inputText: '{}',
};

const emptyConversationTriggerDraft: ConversationTriggerDraft = {
  conversationId: '',
  id: '',
};

const emptyScheduleTriggerDraft: ScheduleTriggerDraft = {
  cron: '',
  enabled: true,
  id: '',
};

const emptySemanticTriggerDraft: SemanticTriggerDraft = {
  id: '',
  keywordsText: '',
  threshold: '',
};

const emptyWebhookTriggerDraft: WebhookTriggerDraft = {
  id: '',
  path: '',
  secret: '',
};

const emptyConversationMatchId = '';
const emptySemanticMatchMessage = '';

const emptyBranchDraft: BranchDraft = {
  description: '',
  experimentKey: '',
  name: '',
  trafficPercent: '',
};

const emptyWorkflowDefinitionDraft: WorkflowDefinitionDraft = {
  edgeBranch: '',
  edgeSource: '',
  edgeTarget: '',
  nodeId: '',
  nodeInputText: '{}',
  nodeType: 'http',
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isWorkflowNodeFailureStrategy(value: unknown): value is WorkflowNodeFailureStrategy {
  return workflowNodeFailureStrategies.some((strategy) => strategy.value === value);
}

function readNodeId(nodeRecord: Record<string, unknown>, index: number) {
  return typeof nodeRecord.id === 'string' && nodeRecord.id.trim() !== '' ? nodeRecord.id : `node-${index + 1}`;
}

function readNodeFailureStrategy(nodeRecord: Record<string, unknown>): WorkflowNodeFailureStrategy {
  const failurePolicy = nodeRecord.failurePolicy;
  if (isRecord(failurePolicy) && isWorkflowNodeFailureStrategy(failurePolicy.strategy)) {
    return failurePolicy.strategy;
  }

  if (isWorkflowNodeFailureStrategy(nodeRecord.failureStrategy)) {
    return nodeRecord.failureStrategy;
  }

  return 'pause_on_failure';
}

function readNodeFailurePolicy(nodeRecord: Record<string, unknown>) {
  return isRecord(nodeRecord.failurePolicy) ? nodeRecord.failurePolicy : {};
}

function readNodeFailureBranchNodeId(failurePolicy: Record<string, unknown>) {
  return typeof failurePolicy.failureBranchNodeId === 'string' ? failurePolicy.failureBranchNodeId : '';
}

function readNodeMaxRetries(failurePolicy: Record<string, unknown>) {
  return typeof failurePolicy.maxRetries === 'number' && Number.isSafeInteger(failurePolicy.maxRetries)
    ? failurePolicy.maxRetries
    : undefined;
}

function readNodePosition(nodeRecord: Record<string, unknown>): WorkflowCanvasPosition | undefined {
  const position = nodeRecord.position;
  if (!isRecord(position)) {
    return undefined;
  }

  const x = position.x;
  const y = position.y;
  if (typeof x !== 'number' || typeof y !== 'number' || !Number.isFinite(x) || !Number.isFinite(y)) {
    return undefined;
  }

  return {
    x,
    y,
  };
}

function snapCanvasCoordinate(value: number) {
  return Math.max(0, Math.round(value / workflowCanvasGridSize) * workflowCanvasGridSize);
}

function snapCanvasPosition(position: WorkflowCanvasPosition): WorkflowCanvasPosition {
  return {
    x: snapCanvasCoordinate(position.x),
    y: snapCanvasCoordinate(position.y),
  };
}

function finiteCanvasCoordinate(value: unknown, fallback: number) {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function fallbackCanvasPosition(index: number): WorkflowCanvasPosition {
  return {
    x: workflowCanvasLayoutOrigin.x + (index % 3) * workflowCanvasColumnGap,
    y: workflowCanvasLayoutOrigin.y + Math.floor(index / 3) * workflowCanvasRowGap,
  };
}

function visualCanvasNodePosition(node: VisualWorkflowNode, index: number, snapEnabled = true) {
  const position = node.position ?? fallbackCanvasPosition(index);
  return snapEnabled ? snapCanvasPosition(position) : position;
}

function formatRetryDelays(value: unknown) {
  if (!Array.isArray(value)) {
    return '';
  }

  return value
    .map((delay) => (typeof delay === 'string' || typeof delay === 'number' ? String(delay).trim() : ''))
    .filter((delay) => delay !== '')
    .join(', ');
}

function getWorkflowNodes(workflow: WorkflowDefinition): VisualWorkflowNode[] {
  const nodes = workflow.definition.nodes;
  if (!Array.isArray(nodes)) {
    return [];
  }

  return nodes.map((node, index) => {
    if (typeof node !== 'object' || node === null) {
      return {
        failureBranchNodeId: '',
        failurePolicy: {},
        failureStrategy: 'pause_on_failure',
        id: `node-${index + 1}`,
        retryDelaysText: '',
        status: 'unknown',
        type: 'unknown',
      };
    }

    const nodeRecord = node as Record<string, unknown>;
    const failurePolicy = readNodeFailurePolicy(nodeRecord);
    return {
      failureBranchNodeId: readNodeFailureBranchNodeId(failurePolicy),
      failurePolicy,
      failureStrategy: readNodeFailureStrategy(nodeRecord),
      id: readNodeId(nodeRecord, index),
      input: nodeRecord.input,
      maxRetries: readNodeMaxRetries(failurePolicy),
      position: readNodePosition(nodeRecord),
      retryDelaysText: formatRetryDelays(failurePolicy.retryDelays),
      status: typeof nodeRecord.status === 'string' && nodeRecord.status.trim() !== '' ? nodeRecord.status : 'unknown',
      type: typeof nodeRecord.type === 'string' && nodeRecord.type.trim() !== '' ? nodeRecord.type : 'unknown',
    };
  });
}

function autoArrangeWorkflowDefinition(definition: Record<string, unknown>) {
  const nodes = workflowDefinitionNodes(definition);
  return {
    ...definition,
    nodes: nodes.map((node, index) => {
      if (!isRecord(node)) {
        return node;
      }

      return {
        ...node,
        position: snapCanvasPosition(fallbackCanvasPosition(index)),
      };
    }),
  };
}

function updateDefinitionNodePosition(
  definition: Record<string, unknown>,
  nodeId: string,
  position: WorkflowCanvasPosition
) {
  let didUpdate = false;
  const nodes = workflowDefinitionNodes(definition).map((node, index) => {
    if (!isRecord(node) || readNodeId(node, index) !== nodeId) {
      return node;
    }

    didUpdate = true;
    return {
      ...node,
      position,
    };
  });

  return didUpdate ? { ...definition, nodes } : undefined;
}

function nodeFailureStrategyDraftKey(workflowId: string, nodeId: string) {
  return `${workflowId}:${nodeId}`;
}

function nodeFailurePolicyDraftKey(workflowId: string, nodeId: string) {
  return `${workflowId}:${nodeId}`;
}

function nodeFailurePolicyDraftFromNode(node: VisualWorkflowNode): NodeFailurePolicyDraft {
  return {
    failureBranchNodeId: node.failureBranchNodeId,
    maxRetries: node.maxRetries === undefined ? '' : String(node.maxRetries),
    retryDelaysText: node.retryDelaysText,
  };
}

function preserveUnmanagedFailurePolicyFields(failurePolicy: Record<string, unknown>) {
  const next = { ...failurePolicy };
  delete next.maxRetries;
  delete next.max_retries;
  delete next.retryDelays;
  delete next.retry_delays;
  delete next.failureBranchNodeId;
  delete next.failure_branch_node_id;
  return next;
}

function updateDefinitionNodeConfig(
  definition: Record<string, unknown>,
  nodeId: string,
  strategy: WorkflowNodeFailureStrategy,
  failurePolicyPatch: Record<string, unknown>,
  input: Record<string, unknown>
) {
  const nodes = definition.nodes;
  if (!Array.isArray(nodes)) {
    return undefined;
  }

  let didUpdate = false;
  const nextNodes = nodes.map((node, index) => {
    if (!isRecord(node)) {
      return node;
    }

    if (readNodeId(node, index) !== nodeId) {
      return node;
    }

    didUpdate = true;
    const currentFailurePolicy = isRecord(node.failurePolicy) ? node.failurePolicy : {};
    return {
      ...node,
      failurePolicy: {
        ...preserveUnmanagedFailurePolicyFields(currentFailurePolicy),
        ...failurePolicyPatch,
        strategy,
      },
      input,
    };
  });

  if (!didUpdate) {
    return undefined;
  }

  return {
    ...definition,
    nodes: nextNodes,
  };
}

function workflowDefinitionNodes(definition: Record<string, unknown>) {
  return Array.isArray(definition.nodes) ? definition.nodes : [];
}

function workflowDefinitionEdges(definition: Record<string, unknown>) {
  return Array.isArray(definition.edges) ? definition.edges : [];
}

function addDefinitionNode(
  definition: Record<string, unknown>,
  nodeId: string,
  nodeType: string,
  input: Record<string, unknown>
) {
  return {
    ...definition,
    nodes: [
      ...workflowDefinitionNodes(definition),
      {
        id: nodeId,
        input,
        type: nodeType,
      },
    ],
  };
}

function nextWorkflowPaletteNodeId(nodes: VisualWorkflowNode[], nodeType: string) {
  const normalizedType = nodeType.trim() || 'node';
  const existingIds = new Set(nodes.map((node) => node.id));
  let index = nodes.length + 1;
  let nodeId = `${normalizedType}-${index}`;
  while (existingIds.has(nodeId)) {
    index += 1;
    nodeId = `${normalizedType}-${index}`;
  }

  return nodeId;
}

function addDefinitionNodeFromPalette(
  definition: Record<string, unknown>,
  nodes: VisualWorkflowNode[],
  nodeType: string,
  position: WorkflowCanvasPosition
) {
  const nodeId = nextWorkflowPaletteNodeId(nodes, nodeType);
  return {
    ...definition,
    nodes: [
      ...workflowDefinitionNodes(definition),
      {
        id: nodeId,
        input: {},
        position,
        type: nodeType,
      },
    ],
  };
}

function addDefinitionEdge(definition: Record<string, unknown>, source: string, target: string, branch?: string) {
  const edge: Record<string, unknown> = {
    source,
    target,
  };
  if (branch && branch.trim() !== '') {
    edge.branch = branch.trim();
  }

  return {
    ...definition,
    edges: [
      ...workflowDefinitionEdges(definition),
      edge,
    ],
  };
}

function removeDefinitionEdge(definition: Record<string, unknown>, edgeIndex: number) {
  return {
    ...definition,
    edges: workflowDefinitionEdges(definition).filter((_, index) => index !== edgeIndex),
  };
}

function removeDefinitionNode(definition: Record<string, unknown>, nodeId: string) {
  return {
    ...definition,
    edges: workflowDefinitionEdges(definition).filter((edge) => {
      if (!isRecord(edge)) {
        return true;
      }

      const source = readEdgeEndpoint(edge, ['source', 'sourceNodeId', 'from', 'fromNodeId']);
      const target = readEdgeEndpoint(edge, ['target', 'targetNodeId', 'to', 'toNodeId']);
      return source !== nodeId && target !== nodeId;
    }),
    nodes: workflowDefinitionNodes(definition).filter((node, index) => {
      if (!isRecord(node)) {
        return true;
      }

      return readNodeId(node, index) !== nodeId;
    }),
  };
}

function readEdgeEndpoint(edge: Record<string, unknown>, keys: string[]) {
  for (const key of keys) {
    const value = edge[key];
    if (typeof value === 'string' && value.trim() !== '') {
      return value;
    }
  }

  return 'unknown';
}

function readEdgeBranch(edge: Record<string, unknown>) {
  for (const key of ['branch', 'condition', 'when']) {
    const value = edge[key];
    if (typeof value === 'string' && value.trim() !== '') {
      return value.trim();
    }
  }

  const data = edge.data;
  if (isRecord(data)) {
    return readEdgeBranch(data);
  }

  return undefined;
}

function getWorkflowEdges(workflow: WorkflowDefinition, nodes: VisualWorkflowNode[]): VisualWorkflowEdge[] {
  const edges = workflow.definition.edges;
  if (!Array.isArray(edges)) {
    return [];
  }

  const nodeIds = new Set(nodes.map((node) => node.id));

  return edges.map((edge, index) => {
    const edgeRecord = typeof edge === 'object' && edge !== null ? (edge as Record<string, unknown>) : {};
    const source = readEdgeEndpoint(edgeRecord, ['source', 'sourceNodeId', 'from', 'fromNodeId']);
    const target = readEdgeEndpoint(edgeRecord, ['target', 'targetNodeId', 'to', 'toNodeId']);
    const idValue = edgeRecord.id;

    return {
      branch: readEdgeBranch(edgeRecord),
      id: typeof idValue === 'string' && idValue.trim() !== '' ? idValue : `edge-${index + 1}`,
      index,
      source,
      status: nodeIds.has(source) && nodeIds.has(target) ? 'active' : 'invalid',
      target,
    };
  });
}

function getWorkflowTriggers(definition: Record<string, unknown>) {
  return isRecord(definition.triggers) ? definition.triggers : {};
}

function firstTriggerRecord(value: unknown) {
  if (isRecord(value)) {
    return value;
  }
  if (Array.isArray(value)) {
    const firstRecord = value.find(isRecord);
    return firstRecord ?? {};
  }

  return {};
}

function stringField(record: Record<string, unknown>, keys: string[]) {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === 'string' && value.trim() !== '') {
      return value.trim();
    }
  }

  return undefined;
}

function numberField(record: Record<string, unknown>, keys: string[]) {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }
    if (typeof value === 'string' && value.trim() !== '') {
      const parsed = Number(value);
      if (Number.isFinite(parsed)) {
        return parsed;
      }
    }
  }

  return undefined;
}

function stringListField(record: Record<string, unknown>, key: string) {
  const value = record[key];
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter((item): item is string => typeof item === 'string' && item.trim() !== '').map((item) => item.trim());
}

function hasConfiguredSecret(record: Record<string, unknown>) {
  return ['secret', 'webhook_secret', 'webhookSecret'].some((key) => {
    const value = record[key];
    return typeof value === 'string' && value.trim() !== '';
  });
}

function boolField(record: Record<string, unknown>, key: string) {
  const value = record[key];
  return typeof value === 'boolean' ? value : undefined;
}

function triggerValueItems(value: unknown) {
  if (Array.isArray(value)) {
    return [...value];
  }
  if (value === undefined) {
    return [];
  }

  return [value];
}

function applyFirstTriggerRecord(
  triggers: Record<string, unknown>,
  key: WorkflowTriggerKind,
  nextTrigger: Record<string, unknown> | undefined
) {
  const nextTriggers = { ...triggers };
  const items = triggerValueItems(triggers[key]);
  const firstRecordIndex = items.findIndex(isRecord);

  if (nextTrigger === undefined) {
    if (firstRecordIndex >= 0) {
      items.splice(firstRecordIndex, 1);
    }
  } else if (firstRecordIndex >= 0) {
    items[firstRecordIndex] = nextTrigger;
  } else {
    items.unshift(nextTrigger);
  }

  if (items.length === 0) {
    delete nextTriggers[key];
  } else {
    nextTriggers[key] = items;
  }

  return nextTriggers;
}

function conversationTriggerDraftFromTriggers(triggers: Record<string, unknown>): ConversationTriggerDraft {
  const conversationTrigger = firstTriggerRecord(triggers.conversation);
  return {
    conversationId: stringField(conversationTrigger, ['conversationId', 'conversation_id', 'conversation']) ?? '',
    id: stringField(conversationTrigger, ['id', 'name']) ?? '',
  };
}

function applyConversationTriggerDraft(triggers: Record<string, unknown>, draft: ConversationTriggerDraft) {
  const conversationId = draft.conversationId.trim();
  const id = draft.id.trim();
  if (conversationId === '' && id === '') {
    return applyFirstTriggerRecord(triggers, 'conversation', undefined);
  }

  const conversation: Record<string, unknown> = {};
  if (conversationId !== '') {
    conversation.conversationId = conversationId;
  }
  if (id !== '') {
    conversation.id = id;
  }

  return applyFirstTriggerRecord(triggers, 'conversation', conversation);
}

function scheduleTriggerDraftFromTriggers(triggers: Record<string, unknown>): ScheduleTriggerDraft {
  const scheduleTrigger = firstTriggerRecord(triggers.schedule);
  return {
    cron: stringField(scheduleTrigger, ['cron', 'cronExpression', 'cron_expression', 'expression']) ?? '',
    enabled: boolField(scheduleTrigger, 'enabled') ?? true,
    id: stringField(scheduleTrigger, ['id', 'name']) ?? '',
  };
}

function applyScheduleTriggerDraft(triggers: Record<string, unknown>, draft: ScheduleTriggerDraft) {
  const id = draft.id.trim();
  const cron = draft.cron.trim();
  if (id === '' && cron === '') {
    return applyFirstTriggerRecord(triggers, 'schedule', undefined);
  }

  const schedule: Record<string, unknown> = {};
  if (cron !== '') {
    schedule.cronExpression = cron;
  }
  schedule.enabled = draft.enabled;
  if (id !== '') {
    schedule.id = id;
  }

  return applyFirstTriggerRecord(triggers, 'schedule', schedule);
}

function semanticTriggerDraftFromTriggers(triggers: Record<string, unknown>): SemanticTriggerDraft {
  const semanticTrigger = firstTriggerRecord(triggers.semantic);
  const threshold = numberField(semanticTrigger, ['semanticThreshold', 'semantic_threshold', 'threshold']);
  return {
    id: stringField(semanticTrigger, ['id', 'name']) ?? '',
    keywordsText: stringListField(semanticTrigger, 'keywords').join(', '),
    threshold: threshold === undefined ? '' : String(threshold),
  };
}

function applySemanticTriggerDraft(triggers: Record<string, unknown>, draft: SemanticTriggerDraft) {
  const id = draft.id.trim();
  const keywords = draft.keywordsText
    .split(',')
    .map((keyword) => keyword.trim())
    .filter((keyword) => keyword !== '');
  const thresholdText = draft.threshold.trim();
  let threshold: number | undefined;
  if (thresholdText !== '') {
    const parsedThreshold = Number(thresholdText);
    if (!Number.isFinite(parsedThreshold)) {
      throw new Error('Semantic threshold must be a number.');
    }
    threshold = parsedThreshold;
  }

  if (id === '' && keywords.length === 0 && threshold === undefined) {
    return applyFirstTriggerRecord(triggers, 'semantic', undefined);
  }

  const semantic: Record<string, unknown> = {};
  if (id !== '') {
    semantic.id = id;
  }
  if (keywords.length > 0) {
    semantic.keywords = keywords;
  }
  if (threshold !== undefined) {
    semantic.semanticThreshold = threshold;
  }

  return applyFirstTriggerRecord(triggers, 'semantic', semantic);
}

function webhookTriggerDraftFromTriggers(triggers: Record<string, unknown>): WebhookTriggerDraft {
  const webhookTrigger = firstTriggerRecord(triggers.webhook);
  return {
    id: stringField(webhookTrigger, ['id', 'name']) ?? '',
    path: stringField(webhookTrigger, ['path', 'url', 'webhookUrl', 'webhook_url']) ?? '',
    secret: stringField(webhookTrigger, ['secret', 'webhook_secret', 'webhookSecret']) ?? '',
  };
}

function applyWebhookTriggerDraft(triggers: Record<string, unknown>, draft: WebhookTriggerDraft) {
  const id = draft.id.trim();
  const path = draft.path.trim();
  const secret = draft.secret.trim();
  if (id === '' && path === '' && secret === '') {
    return applyFirstTriggerRecord(triggers, 'webhook', undefined);
  }

  const webhook: Record<string, unknown> = {};
  if (id !== '') {
    webhook.id = id;
  }
  if (path !== '') {
    webhook.path = path;
  }
  if (secret !== '') {
    webhook.secret = secret;
  }

  return applyFirstTriggerRecord(triggers, 'webhook', webhook);
}

function workflowResourcePolicyDraftFromDefinition(
  definition: Record<string, unknown>
): WorkflowResourcePolicyDraft {
  const overflow = stringField(definition, ['concurrency_overflow', 'concurrencyOverflow']);
  return {
    concurrencyOverflow: overflow === 'reject' ? 'reject' : 'queue',
    maxConcurrentExecutions: numberField(definition, ['max_concurrent_executions', 'maxConcurrentExecutions'])?.toString() ?? '',
    maxExecutionDurationSeconds:
      numberField(definition, ['max_execution_duration_seconds', 'maxExecutionDurationSeconds'])?.toString() ?? '',
    maxNodeExecutions: numberField(definition, ['max_node_executions', 'maxNodeExecutions'])?.toString() ?? '',
    maxTokensBudget: numberField(definition, ['max_tokens_budget', 'maxTokensBudget'])?.toString() ?? '',
  };
}

function setOptionalPolicyInteger(
  definition: Record<string, unknown>,
  key: string,
  value: string,
  label: string
) {
  const parsedValue = parseOptionalInteger(value, label);
  if (parsedValue !== undefined) {
    if (parsedValue === 0) {
      throw new Error(`${label} must be greater than zero.`);
    }
    definition[key] = parsedValue;
  }
}

function applyWorkflowResourcePolicyDraft(
  definition: Record<string, unknown>,
  draft: WorkflowResourcePolicyDraft
) {
  const nextDefinition = { ...definition };
  [
    'max_concurrent_executions',
    'maxConcurrentExecutions',
    'concurrency_overflow',
    'concurrencyOverflow',
    'max_execution_duration_seconds',
    'maxExecutionDurationSeconds',
    'max_tokens_budget',
    'maxTokensBudget',
    'max_node_executions',
    'maxNodeExecutions',
    'totalTokens',
    'nodeExecutionCount',
    'workflow',
    'limits',
  ].forEach((key) => {
    delete nextDefinition[key];
  });

  setOptionalPolicyInteger(
    nextDefinition,
    'max_concurrent_executions',
    draft.maxConcurrentExecutions,
    'Max concurrent executions'
  );
  setOptionalPolicyInteger(
    nextDefinition,
    'max_execution_duration_seconds',
    draft.maxExecutionDurationSeconds,
    'Max execution duration seconds'
  );
  setOptionalPolicyInteger(nextDefinition, 'max_tokens_budget', draft.maxTokensBudget, 'Max tokens budget');
  setOptionalPolicyInteger(nextDefinition, 'max_node_executions', draft.maxNodeExecutions, 'Max node executions');
  nextDefinition.concurrency_overflow = draft.concurrencyOverflow;

  return nextDefinition;
}

function formatConversationTrigger(trigger: Record<string, unknown>) {
  return stringField(trigger, ['conversationId', 'conversation_id', 'id', 'conversation']) ?? 'configured';
}

function formatScheduleTrigger(trigger: Record<string, unknown>) {
  const cron = stringField(trigger, ['cron', 'cronExpression', 'cron_expression', 'expression']) ?? 'configured';
  const timezone = stringField(trigger, ['timezone', 'timeZone', 'tz']);
  return timezone ? `${cron} (${timezone})` : cron;
}

function formatSemanticTrigger(trigger: Record<string, unknown>) {
  const id = stringField(trigger, ['id', 'name']);
  const keywords = stringListField(trigger, 'keywords');
  const threshold = numberField(trigger, ['semanticThreshold', 'semantic_threshold', 'threshold']);
  return [
    id,
    keywords.length > 0 ? keywords.join(', ') : undefined,
    threshold === undefined ? undefined : `threshold ${threshold}`,
  ]
    .filter(Boolean)
    .join(' ') || 'configured';
}

function formatWebhookTrigger(trigger: Record<string, unknown>) {
  return [
    stringField(trigger, ['id', 'name']),
    stringField(trigger, ['path', 'url', 'webhookUrl', 'webhook_url']),
    hasConfiguredSecret(trigger) ? 'secret configured' : undefined,
  ]
    .filter(Boolean)
    .join(' ') || 'configured';
}

function formatTriggerValue(kind: WorkflowTriggerKind, value: unknown): string {
  if (Array.isArray(value)) {
    const summaries: string[] = value
      .map((item) => (isRecord(item) ? formatTriggerValue(kind, item) : undefined))
      .filter((summary): summary is string => typeof summary === 'string' && summary !== '');
    return summaries.length > 0 ? summaries.join('; ') : 'not configured';
  }

  if (!isRecord(value)) {
    return value === undefined ? 'not configured' : String(value);
  }

  if (kind === 'conversation') {
    return formatConversationTrigger(value);
  }
  if (kind === 'schedule') {
    return formatScheduleTrigger(value);
  }
  if (kind === 'semantic') {
    return formatSemanticTrigger(value);
  }

  return formatWebhookTrigger(value);
}

function formatSemanticTriggerMatch(match: SemanticTriggerMatch) {
  return [
    match.triggerId?.trim() || 'trigger',
    match.keyword?.trim() || 'matched',
    typeof match.score === 'number' && Number.isFinite(match.score) ? `score ${match.score}` : undefined,
    typeof match.semanticThreshold === 'number' && Number.isFinite(match.semanticThreshold)
      ? `threshold ${match.semanticThreshold}`
      : undefined,
    match.matchMethod?.trim() || undefined,
  ]
    .filter(Boolean)
    .join(' | ');
}

function formatConversationTriggerMatch(match: ConversationTriggerMatch) {
  return [match.triggerId?.trim() || 'trigger', match.conversationId?.trim() || 'conversation']
    .filter(Boolean)
    .join(' | ');
}

function describeNodeCount(workflow: WorkflowDefinition) {
  return `Nodes: ${getWorkflowNodes(workflow).length}`;
}

function getWorkflowNodeIds(workflow: WorkflowDefinition) {
  return getWorkflowNodes(workflow).map((node) => node.id);
}

function parseJsonObject(value: string) {
  const trimmedValue = value.trim();
  if (trimmedValue === '') {
    return {};
  }

  const parsedValue: unknown = JSON.parse(trimmedValue);
  if (typeof parsedValue !== 'object' || parsedValue === null || Array.isArray(parsedValue)) {
    throw new Error('Expected a JSON object');
  }

  return parsedValue as Record<string, unknown>;
}

function parseOptionalInteger(value: string, label: string) {
  const trimmedValue = value.trim();
  if (trimmedValue === '') {
    return undefined;
  }

  const parsedValue = Number(trimmedValue);
  if (!Number.isSafeInteger(parsedValue) || parsedValue < 0) {
    throw new Error(`${label} must be a non-negative whole number.`);
  }

  return parsedValue;
}

function parseRetryDelays(value: string) {
  return value
    .split(',')
    .map((delay) => delay.trim())
    .filter((delay) => delay !== '');
}

function workflowBranchDraftKey(workflowId: string, version: number) {
  return `${workflowId}:${version}`;
}

function defaultBranchName(workflow: WorkflowDefinition, version: WorkflowDefinition) {
  return `${workflow.name} v${version.version} branch`;
}

function workflowBranchSourceId(version: WorkflowDefinition) {
  const branch = version.definition.branch;
  if (typeof branch !== 'object' || branch === null || Array.isArray(branch)) {
    return null;
  }

  const sourceWorkflowId = (branch as Record<string, unknown>).sourceWorkflowId;
  return typeof sourceWorkflowId === 'string' && sourceWorkflowId.trim() !== '' ? sourceWorkflowId : null;
}

function isWorkflowBranchVersion(workflow: WorkflowDefinition, version: WorkflowDefinition) {
  return version.id !== workflow.id && workflowBranchSourceId(version) === workflow.id;
}

function formatJson(value: unknown) {
  if (value === undefined) {
    return '{}';
  }

  return JSON.stringify(value, null, 2);
}

function workflowRuntimeNodeDebugTemplate(nodeType: string) {
  if (nodeType === 'code') {
    return {
      language: 'javascript',
      code: 'return { result: inputs.value };',
      inputs: {
        value: 'sample',
      },
      timeoutMs: 30000,
    };
  }

  if (nodeType === 'tool') {
    return {
      toolName: 'web_search',
      toolType: 'builtin',
      arguments: {
        query: 'Oblivious workflow runtime',
      },
    };
  }

  if (nodeType === 'database') {
    return {
      connectionId: 'platform',
      query: 'SELECT id, name FROM workflows WHERE organization_id = $1',
      parameters: ['{{org.id}}'],
      limit: 20,
      readOnly: true,
    };
  }

  if (nodeType === 'rpa') {
    return {
      targetUrl: 'https://example.com',
      browserMode: 'headless',
      screenshot: true,
      timeoutMs: 60000,
      steps: [
        {
          action: 'goto',
          value: 'https://example.com',
        },
        {
          action: 'extract',
          selector: 'body',
        },
      ],
    };
  }

  return undefined;
}

function workflowNodeDebugInput(node: VisualWorkflowNode) {
  return node.input ?? workflowRuntimeNodeDebugTemplate(node.type);
}

function formatDuration(durationMs?: number) {
  return typeof durationMs === 'number' && Number.isFinite(durationMs) ? `${durationMs}ms` : 'unknown';
}

function formatTokens(tokens?: number) {
  return typeof tokens === 'number' && Number.isFinite(tokens) ? `${tokens} tokens` : 'tokens unknown';
}

function formatCost(costUsd?: number) {
  return typeof costUsd === 'number' && Number.isFinite(costUsd) ? `$${costUsd.toFixed(3)}` : 'cost unknown';
}

function nodeTestResultPayload(result: WorkflowNodeTestResult) {
  return {
    durationMs: result.durationMs,
    error: result.error,
    input: result.input,
    output: result.output,
    trace: result.trace,
  };
}

function getNodeExecutionId(nodeExecution: WorkflowNodeExecution, index: number) {
  const nodeId = nodeExecution.nodeId?.trim();
  return nodeId && nodeId !== '' ? nodeId : `node-${index + 1}`;
}

function isFailedNodeExecution(nodeExecution: WorkflowNodeExecution) {
  const nodeId = nodeExecution.nodeId?.trim();
  return nodeExecution.status === 'failed' && Boolean(nodeId);
}

function latestFailedNodeExecution(workflowExecution: WorkflowExecution) {
  if (workflowExecution.status !== 'paused') {
    return undefined;
  }

  const nodeExecutions = workflowExecution.nodeExecutions ?? [];
  for (let index = nodeExecutions.length - 1; index >= 0; index -= 1) {
    const nodeExecution = nodeExecutions[index];
    if (isFailedNodeExecution(nodeExecution)) {
      return nodeExecution;
    }
  }

  return undefined;
}

function pausedFailureDecisionDraftKey(executionId: string, nodeId: string) {
  return `${executionId}:${nodeId}`;
}

function isPendingResumeInputNode(nodeExecution: WorkflowNodeExecution) {
  const waitReason = nodeExecution.context?.waitReason;
  return (
    nodeExecution.status === 'pending' &&
    Boolean(nodeExecution.nodeId?.trim()) &&
    (waitReason === 'user_input_required' ||
      waitReason === 'approval_required' ||
      waitReason === 'agent_approval_required' ||
      nodeExecution.nodeType === 'user_input' ||
      nodeExecution.nodeType === 'approval' ||
      nodeExecution.nodeType === 'agent')
  );
}

function latestPendingResumeInputNode(workflowExecution: WorkflowExecution) {
  if (workflowExecution.status !== 'paused') {
    return undefined;
  }

  const nodeExecutions = workflowExecution.nodeExecutions ?? [];
  for (let index = nodeExecutions.length - 1; index >= 0; index -= 1) {
    const nodeExecution = nodeExecutions[index];
    if (isPendingResumeInputNode(nodeExecution)) {
      return nodeExecution;
    }
  }

  return undefined;
}

function pausedInputDraftKey(executionId: string, nodeId: string) {
  return `${executionId}:${nodeId}`;
}

function formatWaitReason(nodeExecution: WorkflowNodeExecution) {
  const waitReason = nodeExecution.context?.waitReason;
  return typeof waitReason === 'string' && waitReason.trim() !== '' ? waitReason : 'user_input_required';
}

function firstPendingAgentToolRunId(output: Record<string, unknown>) {
  const directToolRunId = stringField(output, ['toolRunId', 'toolRunID', 'tool_run_id']);
  if (directToolRunId) {
    return directToolRunId;
  }

  if (!Array.isArray(output.toolRuns)) {
    return undefined;
  }

  const toolRuns = output.toolRuns.filter((toolRun): toolRun is Record<string, unknown> => isRecord(toolRun));
  const pendingToolRun =
    toolRuns.find((toolRun) => {
      const approvalStatus = stringField(toolRun, ['approvalStatus', 'approval_status']);
      const status = stringField(toolRun, ['status']);
      return approvalStatus === 'pending' || status === 'pending_approval' || status === 'pending';
    }) ?? toolRuns[0];

  return pendingToolRun ? stringField(pendingToolRun, ['id', 'toolRunId', 'toolRunID', 'tool_run_id']) : undefined;
}

function defaultPausedInputDraft(nodeExecution?: WorkflowNodeExecution): PausedInputDraft {
  if (!nodeExecution || formatWaitReason(nodeExecution) !== 'agent_approval_required' || !isRecord(nodeExecution.output)) {
    return emptyPausedInputDraft;
  }

  const runId = stringField(nodeExecution.output, ['runId', 'runID', 'run_id']);
  const toolRunId = firstPendingAgentToolRunId(nodeExecution.output);
  const input: Record<string, unknown> = {};
  if (runId) {
    input.runId = runId;
  }
  if (toolRunId) {
    input.toolRunId = toolRunId;
  }
  input.approvalReason = 'approved';

  return Object.keys(input).length > 1 ? { inputText: formatJson(input) } : emptyPausedInputDraft;
}

function buildNodeExecutionMap(workflowExecution?: WorkflowExecution) {
  const nodeExecutionMap = new Map<string, WorkflowNodeExecution>();

  workflowExecution?.nodeExecutions?.forEach((nodeExecution, index) => {
    nodeExecutionMap.set(getNodeExecutionId(nodeExecution, index), nodeExecution);
  });

  return nodeExecutionMap;
}

function nodeExecutionStatus(nodeExecution?: WorkflowNodeExecution) {
  const status = nodeExecution?.status?.trim();
  return status && status !== '' ? status : undefined;
}

function visualNodeStatus(node: VisualWorkflowNode, nodeExecution?: WorkflowNodeExecution) {
  return nodeExecutionStatus(nodeExecution) ?? node.status;
}

function visualNodeAriaLabel(node: VisualWorkflowNode, index: number, nodeExecution?: WorkflowNodeExecution) {
  const durationText = nodeExecution ? ` ${formatDuration(nodeExecution.durationMs)}` : '';
  return `Node ${index + 1} ${node.id} ${node.type} ${visualNodeStatus(node, nodeExecution)}${durationText}`;
}

function visualNodeCardClass(status: string, hasExecution: boolean) {
  if (!hasExecution) {
    return 'border-[#e4dfd2] bg-[#fbfaf7] hover:border-[#1a614f] hover:bg-white';
  }

  if (status === 'failed' || status === 'cancelled' || status === 'timeout') {
    return 'border-red-200 bg-red-50 hover:border-red-300';
  }

  if (status === 'running' || status === 'paused' || status === 'retrying') {
    return 'border-amber-200 bg-amber-50 hover:border-amber-300';
  }

  if (status === 'succeeded') {
    return 'border-emerald-200 bg-emerald-50 hover:border-emerald-300';
  }

  return 'border-slate-200 bg-slate-50 hover:border-slate-300';
}

function visualNodeBadgeClass(status: string, hasExecution: boolean) {
  if (!hasExecution) {
    return 'border-[#d7d2c4] bg-white text-[#625b4f]';
  }

  if (status === 'failed' || status === 'cancelled' || status === 'timeout') {
    return 'border-red-200 bg-white text-red-800';
  }

  if (status === 'running' || status === 'paused' || status === 'retrying') {
    return 'border-amber-200 bg-white text-amber-800';
  }

  if (status === 'succeeded') {
    return 'border-emerald-200 bg-white text-emerald-800';
  }

  return 'border-slate-200 bg-white text-slate-700';
}

function buildWorkflowReactFlowNodes(
  workflowId: string,
  workflowNodes: VisualWorkflowNode[],
  nodeExecutionMap: Map<string, WorkflowNodeExecution>,
  selectedNodeId: string | undefined,
  snapEnabled: boolean,
  onSelect: (workflowId: string, node: VisualWorkflowNode) => void
): WorkflowReactFlowNode[] {
  return workflowNodes.map((node, index) => {
    const nodeExecution = nodeExecutionMap.get(node.id);
    const hasNodeExecution = nodeExecution !== undefined;
    const nodeStatus = visualNodeStatus(node, nodeExecution);
    const position = visualCanvasNodePosition(node, index, snapEnabled);

    return {
      data: {
        hasNodeExecution,
        index,
        node,
        nodeStatus,
        onSelect,
        position,
        statusBadgeClass: visualNodeBadgeClass(nodeStatus, hasNodeExecution),
        workflowId,
      },
      id: node.id,
      position,
      selected: selectedNodeId === node.id,
      style: {
        height: workflowCanvasNodeHeight,
        width: workflowCanvasNodeWidth,
      },
      type: 'workflowNode',
    };
  });
}

function buildWorkflowReactFlowEdges(activeWorkflowEdges: VisualWorkflowEdge[]): ReactFlowEdge[] {
  return activeWorkflowEdges.map((edge) => ({
    data: {},
    id: edge.id,
    label: edge.branch,
    markerEnd: {
      color: '#8c846f',
      type: MarkerType.ArrowClosed,
    },
    source: edge.source,
    style: {
      stroke: '#8c846f',
      strokeWidth: 2,
    },
    target: edge.target,
    type: 'smoothstep',
  }));
}

function buildExecutionDebugSummary(nodeExecutions: WorkflowNodeExecution[]) {
  const totalDurationMs = nodeExecutions.reduce(
    (totalDuration, nodeExecution) =>
      totalDuration +
      (typeof nodeExecution.durationMs === 'number' && Number.isFinite(nodeExecution.durationMs)
        ? nodeExecution.durationMs
        : 0),
    0
  );
  const failedNodeCount = nodeExecutions.filter((nodeExecution) => nodeExecution.status === 'failed').length;
  const retryingNodeCount = nodeExecutions.filter((nodeExecution) => nodeExecution.status === 'retrying').length;
  const longestNodeExecution = nodeExecutions.reduce<WorkflowNodeExecution | undefined>((longest, nodeExecution) => {
    if (!longest) {
      return nodeExecution;
    }

    return (nodeExecution.durationMs ?? -1) > (longest.durationMs ?? -1) ? nodeExecution : longest;
  }, undefined);

  return {
    failedNodeCount,
    longestNodeExecution,
    retryingNodeCount,
    totalDurationMs,
  };
}

function debugSnapshotTraceNodeId(traceEntry: WorkflowExecutionDebugSnapshot['trace'][number], index: number) {
  const nodeId = traceEntry.nodeId?.trim();
  return nodeId && nodeId !== '' ? nodeId : `node-${index + 1}`;
}

function debugSnapshotCallChain(snapshot: WorkflowExecutionDebugSnapshot) {
  return snapshot.trace.length > 0
    ? snapshot.trace.map((traceEntry, index) => debugSnapshotTraceNodeId(traceEntry, index)).join(' -> ')
    : 'No node executions recorded';
}

function buildDebugSnapshotErrors(snapshot: WorkflowExecutionDebugSnapshot) {
  const nodeErrors = snapshot.trace
    .filter((traceEntry) => traceEntry.error !== undefined)
    .map((traceEntry, index) => ({
      error: traceEntry.error,
      nodeId: debugSnapshotTraceNodeId(traceEntry, index),
    }));

  return { nodes: nodeErrors };
}

function debugSnapshotBottleneckText(snapshot: WorkflowExecutionDebugSnapshot) {
  const bottleneckNodeId = snapshot.performance.bottleneckNodeId?.trim();
  if (!bottleneckNodeId) {
    return 'Bottleneck: none';
  }

  return `Bottleneck: ${bottleneckNodeId} (${formatDuration(snapshot.performance.nodeDurationsMs[bottleneckNodeId])})`;
}

const executionStatusLabels: Record<WorkflowExecutionStatus, string> = {
  cancelled: 'Cancelled',
  completed: 'Completed',
  failed: 'Failed',
  max_iterations: 'Max iterations',
  partial_success: 'Partial success',
  paused: 'Paused',
  queued: 'Queued',
  running: 'Running',
  succeeded: 'Succeeded',
  timeout: 'Timed out',
};

function executionStatusClass(status: WorkflowExecutionStatus) {
  if (status === 'queued') {
    return 'border-[#cfc8b7] bg-white text-[#625b4f]';
  }
  if (status === 'running' || status === 'paused') {
    return 'border-blue-200 bg-blue-50 text-blue-800';
  }
  if (status === 'succeeded' || status === 'completed') {
    return 'border-emerald-200 bg-emerald-50 text-emerald-800';
  }
  if (status === 'timeout' || status === 'max_iterations' || status === 'partial_success') {
    return 'border-amber-200 bg-amber-50 text-amber-800';
  }

  return 'border-red-200 bg-red-50 text-red-800';
}

function formatExecutionStatus(status: WorkflowExecutionStatus) {
  return executionStatusLabels[status];
}

function workflowScheduledTasks(tasks: ScheduledTask[], workflowId: string) {
  return tasks.filter((task) => task.targetType === 'workflow' && task.targetId === workflowId);
}

function formatScheduledTaskRunStatus(status: string) {
  return status.trim() === '' ? 'unknown' : status;
}

function buildSignedWebhookPath(workflow: WorkflowDefinition, webhookTriggerDraft: WebhookTriggerDraft) {
  const configuredPath = webhookTriggerDraft.path.trim();
  if (configuredPath !== '') {
    return configuredPath;
  }

  const organizationId = workflow.organizationId?.trim();
  return `/api/v1/workflows/webhooks/${organizationId && organizationId !== '' ? organizationId : '{organization_id}'}/${workflow.id}`;
}

function signedWebhookCurlTarget(path: string) {
  if (/^https?:\/\//i.test(path)) {
    return path;
  }

  return `$APP_ORIGIN${path.startsWith('/') ? path : `/${path}`}`;
}

function hasSignedWebhookSecret(workflow: WorkflowDefinition, webhookTriggerDraft: WebhookTriggerDraft) {
  if (webhookTriggerDraft.secret.trim() !== '') {
    return true;
  }

  return ['webhook_secret', 'webhookSecret'].some((key) => {
    const value = workflow.definition[key];
    return typeof value === 'string' && value.trim() !== '';
  });
}

function signedWebhookCurlCommand(path: string) {
  const curlTarget = signedWebhookCurlTarget(path);
  return [
    `BODY='{"source":"github","action":"opened"}'`,
    'TIMESTAMP=$(date +%s)',
    'SIGNATURE=$(printf \'%s.%s\' "$TIMESTAMP" "$BODY" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" -hex | sed \'s/^.* //\')',
    `curl -X POST "${curlTarget}" \\`,
    '  -H "Content-Type: application/json" \\',
    '  -H "X-Oblivious-Timestamp: $TIMESTAMP" \\',
    '  -H "X-Oblivious-Signature: sha256=$SIGNATURE" \\',
    '  --data "$BODY"',
  ].join('\n');
}

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }

  return fallback;
}

export function WorkflowsPage() {
  const workflowsApi = useMemo(() => createWorkflowsApi(createHttpClient()), []);
  const scheduledTasksApi = useMemo(() => createScheduledTasksApi(createHttpClient()), []);
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [branchDrafts, setBranchDrafts] = useState<Record<string, BranchDraft>>({});
  const [branchForms, setBranchForms] = useState<Record<string, boolean>>({});
  const [debugDrafts, setDebugDrafts] = useState<Record<string, NodeDebugDraft>>({});
  const [error, setError] = useState<string | null>(null);
  const [execution, setExecution] = useState<WorkflowExecution | null>(null);
  const [executionDebugSnapshotsById, setExecutionDebugSnapshotsById] = useState<
    Record<string, WorkflowExecutionDebugSnapshot | undefined>
  >({});
  const [executionsByWorkflow, setExecutionsByWorkflow] = useState<Record<string, WorkflowExecution[]>>({});
  const [conversationMatchId, setConversationMatchId] = useState(emptyConversationMatchId);
  const [conversationMatchResults, setConversationMatchResults] = useState<ConversationTriggerMatch[] | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [nodeResults, setNodeResults] = useState<Record<string, WorkflowNodeTestResult | undefined>>({});
  const [pausedFailureDecisionDrafts, setPausedFailureDecisionDrafts] = useState<
    Record<string, PausedFailureDecisionDraft>
  >({});
  const [pausedInputDrafts, setPausedInputDrafts] = useState<Record<string, PausedInputDraft>>({});
  const [conversationTriggerDrafts, setConversationTriggerDrafts] = useState<Record<string, ConversationTriggerDraft>>(
    {}
  );
  const [dirtyStructuredTriggerDrafts, setDirtyStructuredTriggerDrafts] = useState<
    Record<string, Partial<Record<StructuredTriggerDraftKind, boolean>>>
  >({});
  const [resourceCheckDrafts, setResourceCheckDrafts] = useState<Record<string, ResourceCheckDraft>>({});
  const [runningWorkflowId, setRunningWorkflowId] = useState<string | null>(null);
  const [scheduledTaskRun, setScheduledTaskRun] = useState<ScheduledTaskRun | null>(null);
  const [scheduledTaskRunsByTask, setScheduledTaskRunsByTask] = useState<Record<string, ScheduledTaskRun[]>>({});
  const [scheduledTasks, setScheduledTasks] = useState<ScheduledTask[]>([]);
  const [scheduleTriggerDrafts, setScheduleTriggerDrafts] = useState<Record<string, ScheduleTriggerDraft>>({});
  const [semanticTriggerDrafts, setSemanticTriggerDrafts] = useState<Record<string, SemanticTriggerDraft>>({});
  const [semanticMatchMessage, setSemanticMatchMessage] = useState(emptySemanticMatchMessage);
  const [semanticMatchResults, setSemanticMatchResults] = useState<SemanticTriggerMatch[] | null>(null);
  const [nodeFailurePolicyDrafts, setNodeFailurePolicyDrafts] = useState<Record<string, NodeFailurePolicyDraft>>({});
  const [nodeFailureStrategyDrafts, setNodeFailureStrategyDrafts] = useState<Record<string, WorkflowNodeFailureStrategy>>(
    {}
  );
  const [selectedNodeIds, setSelectedNodeIds] = useState<Record<string, string | undefined>>({});
  const [snapToGridByWorkflow, setSnapToGridByWorkflow] = useState<Record<string, boolean>>({});
  const [versionsByWorkflow, setVersionsByWorkflow] = useState<Record<string, WorkflowDefinition[]>>({});
  const [webhookTriggerDrafts, setWebhookTriggerDrafts] = useState<Record<string, WebhookTriggerDraft>>({});
  const [workflowCreateVariablesText, setWorkflowCreateVariablesText] = useState('{}');
  const [workflowName, setWorkflowName] = useState('');
  const [workflowDefinitionDrafts, setWorkflowDefinitionDrafts] = useState<Record<string, WorkflowDefinitionDraft>>({});
  const [workflowResourcePolicyDrafts, setWorkflowResourcePolicyDrafts] = useState<
    Record<string, WorkflowResourcePolicyDraft>
  >({});
  const [workflowRunInputDrafts, setWorkflowRunInputDrafts] = useState<Record<string, string>>({});
  const [workflowWebhookPayloadDrafts, setWorkflowWebhookPayloadDrafts] = useState<Record<string, string>>({});
  const [workflowTriggerDrafts, setWorkflowTriggerDrafts] = useState<Record<string, string>>({});
  const [workflowVariablesDrafts, setWorkflowVariablesDrafts] = useState<Record<string, string>>({});
  const [workflows, setWorkflows] = useState<WorkflowDefinition[]>([]);

  const refreshScheduledTasks = async () => {
    const loadedTasks = await scheduledTasksApi.listScheduledTasks();
    const nextTasks = Array.isArray(loadedTasks) ? loadedTasks : [];
    setScheduledTasks(nextTasks);
    return nextTasks;
  };

  useEffect(() => {
    let cancelled = false;

    const loadWorkflows = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const nextWorkflows = await workflowsApi.listWorkflows();
        let nextScheduledTasks: ScheduledTask[] = [];
        try {
          const loadedScheduledTasks = await scheduledTasksApi.listScheduledTasks();
          nextScheduledTasks = Array.isArray(loadedScheduledTasks) ? loadedScheduledTasks : [];
        } catch (caughtError) {
          if (!cancelled) {
            setError(errorMessage(caughtError, 'Unable to load scheduled task metadata.'));
          }
        }
        if (!cancelled) {
          setWorkflows(nextWorkflows);
          setScheduledTasks(nextScheduledTasks);
          setWorkflowRunInputDrafts(
            Object.fromEntries(nextWorkflows.map((workflow) => [workflow.id, '{}']))
          );
          setWorkflowWebhookPayloadDrafts(
            Object.fromEntries(nextWorkflows.map((workflow) => [workflow.id, '{}']))
          );
          setWorkflowVariablesDrafts(
            Object.fromEntries(
              nextWorkflows.map((workflow) => [workflow.id, formatJson(workflow.variables)])
            )
          );
          setWorkflowTriggerDrafts(
            Object.fromEntries(
              nextWorkflows.map((workflow) => [workflow.id, formatJson(getWorkflowTriggers(workflow.definition))])
            )
          );
          setConversationTriggerDrafts(
            Object.fromEntries(
              nextWorkflows.map((workflow) => [
                workflow.id,
                conversationTriggerDraftFromTriggers(getWorkflowTriggers(workflow.definition)),
              ])
            )
          );
          setScheduleTriggerDrafts(
            Object.fromEntries(
              nextWorkflows.map((workflow) => [
                workflow.id,
                scheduleTriggerDraftFromTriggers(getWorkflowTriggers(workflow.definition)),
              ])
            )
          );
          setSemanticTriggerDrafts(
            Object.fromEntries(
              nextWorkflows.map((workflow) => [
                workflow.id,
                semanticTriggerDraftFromTriggers(getWorkflowTriggers(workflow.definition)),
              ])
            )
          );
          setWebhookTriggerDrafts(
            Object.fromEntries(
              nextWorkflows.map((workflow) => [
                workflow.id,
                webhookTriggerDraftFromTriggers(getWorkflowTriggers(workflow.definition)),
              ])
            )
          );
          setWorkflowResourcePolicyDrafts(
            Object.fromEntries(
              nextWorkflows.map((workflow) => [
                workflow.id,
                workflowResourcePolicyDraftFromDefinition(workflow.definition),
              ])
            )
          );
        }
      } catch (caughtError) {
        if (!cancelled) {
          setError(errorMessage(caughtError, 'Unable to load workflows. Retry the request or check the backend session.'));
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadWorkflows();

    return () => {
      cancelled = true;
    };
  }, [scheduledTasksApi, workflowsApi]);

  const handleCreateDraft = async () => {
    const trimmedName = workflowName.trim();
    if (trimmedName === '') {
      return;
    }

    setIsCreating(true);
    setError(null);

    try {
      const variables = parseJsonObject(workflowCreateVariablesText);
      const createdWorkflow = await workflowsApi.createWorkflow({
        definition: manualDraftDefinition,
        description: 'Draft workflow created from the workspace.',
        name: trimmedName,
        status: 'draft',
        variables,
      });
      setWorkflows((current) => [createdWorkflow, ...current]);
      syncWorkflowTriggerDraftState(createdWorkflow.id, createdWorkflow.definition);
      setWorkflowRunInputDrafts((current) => ({ ...current, [createdWorkflow.id]: '{}' }));
      setWorkflowWebhookPayloadDrafts((current) => ({ ...current, [createdWorkflow.id]: '{}' }));
      setWorkflowResourcePolicyDrafts((current) => ({
        ...current,
        [createdWorkflow.id]: workflowResourcePolicyDraftFromDefinition(createdWorkflow.definition),
      }));
      setWorkflowVariablesDrafts((current) => ({ ...current, [createdWorkflow.id]: formatJson(createdWorkflow.variables) }));
      setWorkflowName('');
      setWorkflowCreateVariablesText('{}');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to create workflow. Retry the request or check the backend session.'));
    } finally {
      setIsCreating(false);
    }
  };

  const handleMatchSemanticTriggers = async () => {
    const message = semanticMatchMessage.trim();
    if (message === '') {
      setError('Semantic match message is required.');
      return;
    }

    setBusyAction('semantic-matches');
    setError(null);

    try {
      const matches = await workflowsApi.matchSemanticTriggers({ message });
      setSemanticMatchResults(matches);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to check semantic trigger matches.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleMatchConversationTriggers = async () => {
    const conversationId = conversationMatchId.trim();
    if (conversationId === '') {
      setError('Conversation match ID is required.');
      return;
    }

    setBusyAction('conversation-matches');
    setError(null);

    try {
      const matches = await workflowsApi.matchConversationTriggers({ conversationId });
      setConversationMatchResults(matches);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to check conversation trigger matches.'));
    } finally {
      setBusyAction(null);
    }
  };

  const updateWorkflowVariablesDraft = (workflow: WorkflowDefinition, value: string) => {
    setWorkflowVariablesDrafts((current) => ({ ...current, [workflow.id]: value }));
  };

  const updateWorkflowRunInputDraft = (workflow: WorkflowDefinition, value: string) => {
    setWorkflowRunInputDrafts((current) => ({ ...current, [workflow.id]: value }));
  };

  const updateWorkflowWebhookPayloadDraft = (workflow: WorkflowDefinition, value: string) => {
    setWorkflowWebhookPayloadDrafts((current) => ({ ...current, [workflow.id]: value }));
  };

  const updateWorkflowTriggerDraft = (workflow: WorkflowDefinition, value: string) => {
    setWorkflowTriggerDrafts((current) => ({ ...current, [workflow.id]: value }));
    try {
      const triggers = parseJsonObject(value);
      if (!dirtyStructuredTriggerDrafts[workflow.id]?.conversation) {
        setConversationTriggerDrafts((current) => ({
          ...current,
          [workflow.id]: conversationTriggerDraftFromTriggers(triggers),
        }));
      }
      if (!dirtyStructuredTriggerDrafts[workflow.id]?.schedule) {
        setScheduleTriggerDrafts((current) => ({
          ...current,
          [workflow.id]: scheduleTriggerDraftFromTriggers(triggers),
        }));
      }
      if (!dirtyStructuredTriggerDrafts[workflow.id]?.semantic) {
        setSemanticTriggerDrafts((current) => ({
          ...current,
          [workflow.id]: semanticTriggerDraftFromTriggers(triggers),
        }));
      }
      if (!dirtyStructuredTriggerDrafts[workflow.id]?.webhook) {
        setWebhookTriggerDrafts((current) => ({
          ...current,
          [workflow.id]: webhookTriggerDraftFromTriggers(triggers),
        }));
      }
    } catch {
      // Keep the structured trigger controls stable while the raw JSON draft is temporarily invalid.
    }
  };

  const syncWorkflowTriggerDraftState = (workflowId: string, definition: Record<string, unknown>) => {
    const triggers = getWorkflowTriggers(definition);
    setWorkflowTriggerDrafts((current) => ({
      ...current,
      [workflowId]: formatJson(triggers),
    }));
    setConversationTriggerDrafts((current) => ({
      ...current,
      [workflowId]: conversationTriggerDraftFromTriggers(triggers),
    }));
    setScheduleTriggerDrafts((current) => ({
      ...current,
      [workflowId]: scheduleTriggerDraftFromTriggers(triggers),
    }));
    setSemanticTriggerDrafts((current) => ({
      ...current,
      [workflowId]: semanticTriggerDraftFromTriggers(triggers),
    }));
    setWebhookTriggerDrafts((current) => ({
      ...current,
      [workflowId]: webhookTriggerDraftFromTriggers(triggers),
    }));
    setDirtyStructuredTriggerDrafts((current) => ({
      ...current,
      [workflowId]: {},
    }));
  };

  const updateConversationTriggerDraft = (workflow: WorkflowDefinition, patch: Partial<ConversationTriggerDraft>) => {
    setDirtyStructuredTriggerDrafts((current) => ({
      ...current,
      [workflow.id]: {
        ...current[workflow.id],
        conversation: true,
      },
    }));
    setConversationTriggerDrafts((current) => ({
      ...current,
      [workflow.id]: {
        ...(current[workflow.id] ?? conversationTriggerDraftFromTriggers(getWorkflowTriggers(workflow.definition))),
        ...patch,
      },
    }));
  };

  const updateScheduleTriggerDraft = (workflow: WorkflowDefinition, patch: Partial<ScheduleTriggerDraft>) => {
    setDirtyStructuredTriggerDrafts((current) => ({
      ...current,
      [workflow.id]: {
        ...current[workflow.id],
        schedule: true,
      },
    }));
    setScheduleTriggerDrafts((current) => ({
      ...current,
      [workflow.id]: {
        ...(current[workflow.id] ?? scheduleTriggerDraftFromTriggers(getWorkflowTriggers(workflow.definition))),
        ...patch,
      },
    }));
  };

  const updateSemanticTriggerDraft = (workflow: WorkflowDefinition, patch: Partial<SemanticTriggerDraft>) => {
    setDirtyStructuredTriggerDrafts((current) => ({
      ...current,
      [workflow.id]: {
        ...current[workflow.id],
        semantic: true,
      },
    }));
    setSemanticTriggerDrafts((current) => ({
      ...current,
      [workflow.id]: {
        ...(current[workflow.id] ?? semanticTriggerDraftFromTriggers(getWorkflowTriggers(workflow.definition))),
        ...patch,
      },
    }));
  };

  const updateWebhookTriggerDraft = (workflow: WorkflowDefinition, patch: Partial<WebhookTriggerDraft>) => {
    setDirtyStructuredTriggerDrafts((current) => ({
      ...current,
      [workflow.id]: {
        ...current[workflow.id],
        webhook: true,
      },
    }));
    setWebhookTriggerDrafts((current) => ({
      ...current,
      [workflow.id]: {
        ...(current[workflow.id] ?? webhookTriggerDraftFromTriggers(getWorkflowTriggers(workflow.definition))),
        ...patch,
      },
    }));
  };

  const handleSaveWorkflowVariables = async (workflow: WorkflowDefinition) => {
    let variables: Record<string, unknown>;
    try {
      variables = parseJsonObject(workflowVariablesDrafts[workflow.id] ?? formatJson(workflow.variables));
    } catch {
      setError('Workflow variables JSON must be a JSON object.');
      return;
    }

    setBusyAction(`variables:${workflow.id}`);
    setError(null);

    try {
      const updatedWorkflow = await workflowsApi.updateWorkflow(workflow.id, {
        definition: workflow.definition,
        description: workflow.description,
        name: workflow.name,
        status: workflow.status,
        variables,
      });
      setWorkflows((current) =>
        current.map((currentWorkflow) =>
          currentWorkflow.id === updatedWorkflow.id ? updatedWorkflow : currentWorkflow
        )
      );
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [updatedWorkflow.id]: formatJson(updatedWorkflow.variables),
      }));
      syncWorkflowTriggerDraftState(updatedWorkflow.id, updatedWorkflow.definition);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to save workflow variables.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleSaveWorkflowTriggers = async (workflow: WorkflowDefinition) => {
    let triggers: Record<string, unknown>;
    try {
      triggers = parseJsonObject(workflowTriggerDrafts[workflow.id] ?? formatJson(getWorkflowTriggers(workflow.definition)));
    } catch {
      setError('Workflow triggers JSON must be a JSON object.');
      return;
    }
    if (dirtyStructuredTriggerDrafts[workflow.id]?.conversation) {
      triggers = applyConversationTriggerDraft(
        triggers,
        conversationTriggerDrafts[workflow.id] ?? conversationTriggerDraftFromTriggers(triggers)
      );
    }
    triggers = applyScheduleTriggerDraft(
      triggers,
      scheduleTriggerDrafts[workflow.id] ?? scheduleTriggerDraftFromTriggers(triggers)
    );
    if (dirtyStructuredTriggerDrafts[workflow.id]?.semantic) {
      try {
        triggers = applySemanticTriggerDraft(
          triggers,
          semanticTriggerDrafts[workflow.id] ?? semanticTriggerDraftFromTriggers(triggers)
        );
      } catch (caughtError) {
        setError(errorMessage(caughtError, 'Semantic threshold must be a number.'));
        return;
      }
    }
    if (dirtyStructuredTriggerDrafts[workflow.id]?.webhook) {
      triggers = applyWebhookTriggerDraft(
        triggers,
        webhookTriggerDrafts[workflow.id] ?? webhookTriggerDraftFromTriggers(triggers)
      );
    }

    setBusyAction(`triggers:${workflow.id}`);
    setError(null);

    try {
      const updatedWorkflow = await workflowsApi.updateWorkflow(workflow.id, {
        definition: {
          ...workflow.definition,
          triggers,
        },
        description: workflow.description,
        name: workflow.name,
        status: workflow.status,
        variables: workflow.variables,
      });
      setWorkflows((current) =>
        current.map((currentWorkflow) =>
          currentWorkflow.id === updatedWorkflow.id ? updatedWorkflow : currentWorkflow
        )
      );
      syncWorkflowTriggerDraftState(updatedWorkflow.id, updatedWorkflow.definition);
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [updatedWorkflow.id]: formatJson(updatedWorkflow.variables),
      }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to save workflow triggers.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handlePublishWorkflow = async (workflow: WorkflowDefinition) => {
    setBusyAction(`publish:${workflow.id}`);
    setError(null);

    try {
      const updatedWorkflow = await workflowsApi.updateWorkflow(workflow.id, {
        definition: workflow.definition,
        description: workflow.description,
        name: workflow.name,
        status: 'published',
        variables: workflow.variables,
      });
      setWorkflows((current) =>
        current.map((currentWorkflow) =>
          currentWorkflow.id === updatedWorkflow.id ? updatedWorkflow : currentWorkflow
        )
      );
      syncWorkflowTriggerDraftState(updatedWorkflow.id, updatedWorkflow.definition);
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [updatedWorkflow.id]: formatJson(updatedWorkflow.variables),
      }));
      await refreshScheduledTasks();
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to publish workflow.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleArchiveWorkflow = async (workflow: WorkflowDefinition) => {
    setBusyAction(`archive:${workflow.id}`);
    setError(null);

    try {
      const archivedWorkflow = await workflowsApi.deleteWorkflow(workflow.id);
      setWorkflows((current) =>
        current.map((currentWorkflow) =>
          currentWorkflow.id === archivedWorkflow.id ? archivedWorkflow : currentWorkflow
        )
      );
      syncWorkflowTriggerDraftState(archivedWorkflow.id, archivedWorkflow.definition);
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [archivedWorkflow.id]: formatJson(archivedWorkflow.variables),
      }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to archive workflow.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleRunScheduledTaskNow = async (task: ScheduledTask) => {
    setBusyAction(`scheduled-task:${task.id}:run`);
    setError(null);
    setScheduledTaskRun(null);

    try {
      const run = await scheduledTasksApi.runScheduledTaskNow(task.id);
      setScheduledTaskRun(run);
      setScheduledTaskRunsByTask((current) => ({
        ...current,
        [task.id]: [run, ...(current[task.id] ?? []).filter((currentRun) => currentRun.id !== run.id)],
      }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to run scheduled task.'));
    } finally {
      setBusyAction(null);
    }
  };

  const prependWorkflowExecution = (workflowId: string, nextExecution: WorkflowExecution) => {
    setExecution(nextExecution);
    setExecutionsByWorkflow((current) => ({
      ...current,
      [workflowId]: [
        nextExecution,
        ...(current[workflowId] ?? []).filter((currentExecution) => currentExecution.id !== nextExecution.id),
      ],
    }));
  };

  const handleExecuteWorkflow = async (workflow: WorkflowDefinition) => {
    let input: Record<string, unknown>;
    try {
      input = parseJsonObject(workflowRunInputDrafts[workflow.id] ?? '{}');
    } catch {
      setError('Run input JSON must be a JSON object.');
      return;
    }

    setRunningWorkflowId(workflow.id);
    setError(null);
    setExecution(null);

    try {
      const nextExecution = await workflowsApi.executeWorkflow(workflow.id, { executionMode: 'auto', input });
      prependWorkflowExecution(workflow.id, nextExecution);
    } catch {
      setError('Unable to execute workflow. The workflow list was preserved.');
    } finally {
      setRunningWorkflowId(null);
    }
  };

  const handleTriggerWorkflowWebhook = async (workflow: WorkflowDefinition) => {
    let payload: Record<string, unknown>;
    try {
      payload = parseJsonObject(workflowWebhookPayloadDrafts[workflow.id] ?? '{}');
    } catch {
      setError('Webhook payload JSON must be a JSON object.');
      return;
    }

    setBusyAction(`webhook:${workflow.id}`);
    setError(null);
    setExecution(null);

    try {
      const nextExecution = await workflowsApi.triggerWorkflowWebhook(workflow.id, payload);
      prependWorkflowExecution(workflow.id, nextExecution);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to trigger workflow webhook.'));
    } finally {
      setBusyAction(null);
    }
  };

  const updateDebugDraft = (workflowId: string, patch: Partial<NodeDebugDraft>) => {
    setDebugDrafts((current) => ({
      ...current,
      [workflowId]: {
        ...(current[workflowId] ?? emptyDebugDraft),
        ...patch,
      },
    }));
  };

  const updateResourceCheckDraft = (executionId: string, patch: Partial<ResourceCheckDraft>) => {
    setResourceCheckDrafts((current) => ({
      ...current,
      [executionId]: {
        ...(current[executionId] ?? emptyResourceCheckDraft),
        ...patch,
      },
    }));
  };

  const updateWorkflowResourcePolicyDraft = (workflowId: string, patch: Partial<WorkflowResourcePolicyDraft>) => {
    setWorkflowResourcePolicyDrafts((current) => ({
      ...current,
      [workflowId]: {
        ...(current[workflowId] ?? emptyWorkflowResourcePolicyDraft),
        ...patch,
      },
    }));
  };

  const updatePausedFailureDecisionDraft = (
    executionId: string,
    nodeId: string,
    patch: Partial<PausedFailureDecisionDraft>
  ) => {
    const key = pausedFailureDecisionDraftKey(executionId, nodeId);
    setPausedFailureDecisionDrafts((current) => ({
      ...current,
      [key]: {
        ...(current[key] ?? emptyPausedFailureDecisionDraft),
        ...patch,
      },
    }));
  };

  const updatePausedInputDraft = (executionId: string, nodeId: string, patch: Partial<PausedInputDraft>) => {
    const key = pausedInputDraftKey(executionId, nodeId);
    setPausedInputDrafts((current) => ({
      ...current,
      [key]: {
        ...(current[key] ?? emptyPausedInputDraft),
        ...patch,
      },
    }));
  };

  const updateBranchDraft = (workflow: WorkflowDefinition, version: WorkflowDefinition, patch: Partial<BranchDraft>) => {
    const key = workflowBranchDraftKey(workflow.id, version.version);
    setBranchDrafts((current) => ({
      ...current,
      [key]: {
        ...(current[key] ?? { ...emptyBranchDraft, name: defaultBranchName(workflow, version) }),
        ...patch,
      },
    }));
  };

  const updateWorkflowDefinitionDraft = (workflowId: string, patch: Partial<WorkflowDefinitionDraft>) => {
    setWorkflowDefinitionDrafts((current) => ({
      ...current,
      [workflowId]: {
        ...(current[workflowId] ?? emptyWorkflowDefinitionDraft),
        ...patch,
      },
    }));
  };

  const replaceWorkflowDefinitionDraft = (workflow: WorkflowDefinition, definition: Record<string, unknown>) => {
    setWorkflows((current) =>
      current.map((currentWorkflow) =>
        currentWorkflow.id === workflow.id
          ? {
              ...currentWorkflow,
              definition,
            }
          : currentWorkflow
      )
    );
    syncWorkflowTriggerDraftState(workflow.id, definition);
  };

  const handleAddWorkflowNode = (workflow: WorkflowDefinition) => {
    const draft = workflowDefinitionDrafts[workflow.id] ?? emptyWorkflowDefinitionDraft;
    const nodeId = draft.nodeId.trim();
    const nodeType = draft.nodeType.trim();
    if (nodeId === '' || nodeType === '') {
      setError('New workflow node ID and type are required.');
      return;
    }
    if (getWorkflowNodes(workflow).some((node) => node.id === nodeId)) {
      setError(`Workflow node ${nodeId} already exists.`);
      return;
    }

    let input: Record<string, unknown>;
    try {
      input = parseJsonObject(draft.nodeInputText);
    } catch {
      setError('New workflow node input JSON must be a JSON object.');
      return;
    }

    const definition = addDefinitionNode(workflow.definition, nodeId, nodeType, input);
    replaceWorkflowDefinitionDraft(workflow, definition);
    setWorkflowDefinitionDrafts((current) => ({
      ...current,
      [workflow.id]: {
        ...(current[workflow.id] ?? emptyWorkflowDefinitionDraft),
        nodeId: '',
        nodeInputText: '{}',
      },
    }));
    setSelectedNodeIds((current) => ({ ...current, [workflow.id]: nodeId }));
    updateDebugDraft(workflow.id, {
      inputText: formatJson(input),
      nodeId,
    });
    setError(null);
  };

  const handleAddWorkflowEdge = (workflow: WorkflowDefinition) => {
    const draft = workflowDefinitionDrafts[workflow.id] ?? emptyWorkflowDefinitionDraft;
    const source = draft.edgeSource.trim();
    const target = draft.edgeTarget.trim();
    const branch = draft.edgeBranch.trim();
    if (source === '' || target === '') {
      setError('New workflow edge source and target are required.');
      return;
    }

    const nodeIds = new Set(getWorkflowNodes(workflow).map((node) => node.id));
    if (!nodeIds.has(source) || !nodeIds.has(target)) {
      setError('New workflow edge endpoints must reference existing nodes.');
      return;
    }

    const definition = addDefinitionEdge(workflow.definition, source, target, branch);
    replaceWorkflowDefinitionDraft(workflow, definition);
    setWorkflowDefinitionDrafts((current) => ({
      ...current,
      [workflow.id]: {
        ...(current[workflow.id] ?? emptyWorkflowDefinitionDraft),
        edgeBranch: '',
        edgeSource: '',
        edgeTarget: '',
      },
    }));
    setError(null);
  };

  const handleRemoveWorkflowEdge = (workflow: WorkflowDefinition, edge: VisualWorkflowEdge) => {
    replaceWorkflowDefinitionDraft(workflow, removeDefinitionEdge(workflow.definition, edge.index));
    setError(null);
  };

  const handleRemoveWorkflowNode = (workflow: WorkflowDefinition, node: VisualWorkflowNode) => {
    replaceWorkflowDefinitionDraft(workflow, removeDefinitionNode(workflow.definition, node.id));
    setSelectedNodeIds((current) => ({
      ...current,
      [workflow.id]: current[workflow.id] === node.id ? undefined : current[workflow.id],
    }));
    setDebugDrafts((current) => ({
      ...current,
      [workflow.id]:
        current[workflow.id]?.nodeId === node.id
          ? emptyDebugDraft
          : (current[workflow.id] ?? emptyDebugDraft),
    }));
    setNodeFailureStrategyDrafts((current) => {
      const next = { ...current };
      delete next[nodeFailureStrategyDraftKey(workflow.id, node.id)];
      return next;
    });
    setNodeFailurePolicyDrafts((current) => {
      const next = { ...current };
      delete next[nodeFailurePolicyDraftKey(workflow.id, node.id)];
      return next;
    });
    setError(null);
  };

  const handleWorkflowCanvasNodeDragStop = (
    workflow: WorkflowDefinition,
    nodeId: string,
    position: WorkflowCanvasPosition
  ) => {
    const nextPosition = snapToGridByWorkflow[workflow.id] ?? true ? snapCanvasPosition(position) : position;
    const definition = updateDefinitionNodePosition(workflow.definition, nodeId, nextPosition);
    if (!definition) {
      return;
    }

    replaceWorkflowDefinitionDraft(workflow, definition);
    setSelectedNodeIds((current) => ({ ...current, [workflow.id]: nodeId }));
    setError(null);
  };

  const handleWorkflowCanvasConnect = (workflow: WorkflowDefinition, connection: Connection) => {
    if (!connection.source || !connection.target) {
      setError('New workflow edge source and target are required.');
      return;
    }

    const definition = addDefinitionEdge(workflow.definition, connection.source, connection.target);
    replaceWorkflowDefinitionDraft(workflow, definition);
    setError(null);
  };

  const handleWorkflowPaletteDragStart = (event: DragEvent<HTMLButtonElement>, nodeType: string) => {
    event.dataTransfer.effectAllowed = 'copy';
    event.dataTransfer.setData(workflowNodeDragDataType, nodeType);
  };

  const handleWorkflowCanvasDrop = (
    event: DragEvent<HTMLDivElement>,
    workflow: WorkflowDefinition,
    nodes: VisualWorkflowNode[]
  ) => {
    event.preventDefault();
    const nodeType = event.dataTransfer?.getData(workflowNodeDragDataType).trim() ?? '';
    if (nodeType === '') {
      return;
    }

    const canvasBounds = event.currentTarget.getBoundingClientRect();
    const fallbackPosition = fallbackCanvasPosition(nodes.length);
    const canvasLeft = finiteCanvasCoordinate(canvasBounds.left, 0);
    const canvasTop = finiteCanvasCoordinate(canvasBounds.top, 0);
    const position = snapCanvasPosition({
      x:
        finiteCanvasCoordinate(event.clientX, canvasLeft + fallbackPosition.x) -
        canvasLeft +
        finiteCanvasCoordinate(event.currentTarget.scrollLeft, 0),
      y:
        finiteCanvasCoordinate(event.clientY, canvasTop + fallbackPosition.y) -
        canvasTop +
        finiteCanvasCoordinate(event.currentTarget.scrollTop, 0),
    });
    const nodeId = nextWorkflowPaletteNodeId(nodes, nodeType);
    const definition = addDefinitionNodeFromPalette(workflow.definition, nodes, nodeType, position);
    replaceWorkflowDefinitionDraft(workflow, definition);
    setSelectedNodeIds((current) => ({ ...current, [workflow.id]: nodeId }));
    updateDebugDraft(workflow.id, {
      inputText: formatJson({}),
      nodeId,
    });
    updateWorkflowDefinitionDraft(workflow.id, {
      nodeId: '',
      nodeInputText: '{}',
      nodeType,
    });
    setError(null);
  };

  const handleSaveWorkflowDefinition = async (workflow: WorkflowDefinition) => {
    setBusyAction(`definition:${workflow.id}`);
    setError(null);

    try {
      const updatedWorkflow = await workflowsApi.updateWorkflow(workflow.id, {
        definition: workflow.definition,
        description: workflow.description,
        name: workflow.name,
        status: workflow.status,
        variables: workflow.variables,
      });
      setWorkflows((current) =>
        current.map((currentWorkflow) =>
          currentWorkflow.id === updatedWorkflow.id ? updatedWorkflow : currentWorkflow
        )
      );
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [updatedWorkflow.id]: formatJson(updatedWorkflow.variables),
      }));
      syncWorkflowTriggerDraftState(updatedWorkflow.id, updatedWorkflow.definition);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to save workflow definition.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleAutoArrangeWorkflow = async (workflow: WorkflowDefinition) => {
    const definition = autoArrangeWorkflowDefinition(workflow.definition);
    replaceWorkflowDefinitionDraft(workflow, definition);
    setBusyAction(`layout:${workflow.id}`);
    setError(null);

    try {
      const updatedWorkflow = await workflowsApi.updateWorkflow(workflow.id, {
        definition,
        description: workflow.description,
        name: workflow.name,
        status: workflow.status,
        variables: workflow.variables,
      });
      setWorkflows((current) =>
        current.map((currentWorkflow) =>
          currentWorkflow.id === updatedWorkflow.id ? updatedWorkflow : currentWorkflow
        )
      );
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [updatedWorkflow.id]: formatJson(updatedWorkflow.variables),
      }));
      syncWorkflowTriggerDraftState(updatedWorkflow.id, updatedWorkflow.definition);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to auto arrange workflow nodes.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleSaveWorkflowResourcePolicy = async (workflow: WorkflowDefinition) => {
    let definition: Record<string, unknown>;
    try {
      definition = applyWorkflowResourcePolicyDraft(
        workflow.definition,
        workflowResourcePolicyDrafts[workflow.id] ?? workflowResourcePolicyDraftFromDefinition(workflow.definition)
      );
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Workflow resource policy values must be positive whole numbers.'));
      return;
    }

    setBusyAction(`resource-policy:${workflow.id}`);
    setError(null);

    try {
      const updatedWorkflow = await workflowsApi.updateWorkflow(workflow.id, {
        definition,
        description: workflow.description,
        name: workflow.name,
        status: workflow.status,
        variables: workflow.variables,
      });
      setWorkflows((current) =>
        current.map((currentWorkflow) =>
          currentWorkflow.id === updatedWorkflow.id ? updatedWorkflow : currentWorkflow
        )
      );
      setWorkflowResourcePolicyDrafts((current) => ({
        ...current,
        [updatedWorkflow.id]: workflowResourcePolicyDraftFromDefinition(updatedWorkflow.definition),
      }));
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [updatedWorkflow.id]: formatJson(updatedWorkflow.variables),
      }));
      syncWorkflowTriggerDraftState(updatedWorkflow.id, updatedWorkflow.definition);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to save workflow resource policy.'));
    } finally {
      setBusyAction(null);
    }
  };

  const showBranchForm = (workflow: WorkflowDefinition, version: WorkflowDefinition) => {
    const key = workflowBranchDraftKey(workflow.id, version.version);
    setBranchForms((current) => ({ ...current, [key]: true }));
    setBranchDrafts((current) => ({
      ...current,
      [key]: current[key] ?? { ...emptyBranchDraft, name: defaultBranchName(workflow, version) },
    }));
  };

  const handleSelectNode = (workflowId: string, node: VisualWorkflowNode) => {
    setSelectedNodeIds((current) => ({ ...current, [workflowId]: node.id }));
    setNodeFailureStrategyDrafts((current) => ({
      ...current,
      [nodeFailureStrategyDraftKey(workflowId, node.id)]:
        current[nodeFailureStrategyDraftKey(workflowId, node.id)] ?? node.failureStrategy,
    }));
    setNodeFailurePolicyDrafts((current) => ({
      ...current,
      [nodeFailurePolicyDraftKey(workflowId, node.id)]:
        current[nodeFailurePolicyDraftKey(workflowId, node.id)] ?? nodeFailurePolicyDraftFromNode(node),
    }));
    updateDebugDraft(workflowId, {
      inputText: formatJson(workflowNodeDebugInput(node)),
      nodeId: node.id,
    });
  };

  const updateNodeFailureStrategyDraft = (
    workflowId: string,
    nodeId: string,
    strategy: WorkflowNodeFailureStrategy
  ) => {
    setNodeFailureStrategyDrafts((current) => ({
      ...current,
      [nodeFailureStrategyDraftKey(workflowId, nodeId)]: strategy,
    }));
  };

  const updateNodeFailurePolicyDraft = (workflowId: string, nodeId: string, patch: Partial<NodeFailurePolicyDraft>) => {
    const key = nodeFailurePolicyDraftKey(workflowId, nodeId);
    setNodeFailurePolicyDrafts((current) => ({
      ...current,
      [key]: {
        ...(current[key] ?? emptyNodeFailurePolicyDraft),
        ...patch,
      },
    }));
  };

  const handleSaveSelectedNodeConfig = async (workflow: WorkflowDefinition, node: VisualWorkflowNode) => {
    const strategy =
      nodeFailureStrategyDrafts[nodeFailureStrategyDraftKey(workflow.id, node.id)] ?? node.failureStrategy;
    const failurePolicyDraft =
      nodeFailurePolicyDrafts[nodeFailurePolicyDraftKey(workflow.id, node.id)] ?? nodeFailurePolicyDraftFromNode(node);
    let input: Record<string, unknown>;
    try {
      input = parseJsonObject((debugDrafts[workflow.id] ?? emptyDebugDraft).inputText);
    } catch {
      setError('Node input JSON must be a JSON object.');
      return;
    }

    let maxRetries: number | undefined;
    try {
      maxRetries = parseOptionalInteger(failurePolicyDraft.maxRetries, 'Max retries');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Max retries must be a non-negative whole number.'));
      return;
    }

    const failurePolicyPatch: Record<string, unknown> = {};
    if (maxRetries !== undefined) {
      failurePolicyPatch.maxRetries = maxRetries;
    }
    const retryDelays = parseRetryDelays(failurePolicyDraft.retryDelaysText);
    if (retryDelays.length > 0) {
      failurePolicyPatch.retryDelays = retryDelays;
    }
    const failureBranchNodeId = failurePolicyDraft.failureBranchNodeId.trim();
    if (failureBranchNodeId !== '') {
      failurePolicyPatch.failureBranchNodeId = failureBranchNodeId;
    }

    const definition = updateDefinitionNodeConfig(workflow.definition, node.id, strategy, failurePolicyPatch, input);

    if (!definition) {
      setError(`Unable to find node ${node.id} in the workflow definition.`);
      return;
    }

    setBusyAction(`node-config:${workflow.id}:${node.id}`);
    setError(null);

    try {
      const updatedWorkflow = await workflowsApi.updateWorkflow(workflow.id, {
        definition,
        description: workflow.description,
        name: workflow.name,
        status: workflow.status,
        variables: workflow.variables,
      });
      setWorkflows((current) =>
        current.map((currentWorkflow) =>
          currentWorkflow.id === updatedWorkflow.id ? updatedWorkflow : currentWorkflow
        )
      );
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [updatedWorkflow.id]: formatJson(updatedWorkflow.variables),
      }));
      syncWorkflowTriggerDraftState(updatedWorkflow.id, updatedWorkflow.definition);
      const updatedNode = getWorkflowNodes(updatedWorkflow).find((currentNode) => currentNode.id === node.id);
      if (updatedNode) {
        setNodeFailureStrategyDrafts((current) => ({
          ...current,
          [nodeFailureStrategyDraftKey(updatedWorkflow.id, updatedNode.id)]: updatedNode.failureStrategy,
        }));
        setNodeFailurePolicyDrafts((current) => ({
          ...current,
          [nodeFailurePolicyDraftKey(updatedWorkflow.id, updatedNode.id)]: nodeFailurePolicyDraftFromNode(updatedNode),
        }));
      }
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to save workflow node configuration.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleTestNode = async (workflow: WorkflowDefinition) => {
    const draft = debugDrafts[workflow.id] ?? emptyDebugDraft;
    const nodeId = draft.nodeId.trim();
    if (nodeId === '') {
      setError('Enter a node ID before testing a workflow node.');
      return;
    }

    let input: Record<string, unknown>;
    try {
      input = parseJsonObject(draft.inputText);
    } catch {
      setError('Node input JSON must be a JSON object.');
      return;
    }

    setBusyAction(`test:${workflow.id}`);
    setError(null);

    try {
      const result = await workflowsApi.testNode(workflow.id, { input, nodeId });
      setNodeResults((current) => ({ ...current, [workflow.id]: result }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to test workflow node.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleLoadExecutions = async (workflow: WorkflowDefinition) => {
    setBusyAction(`executions:${workflow.id}`);
    setError(null);

    try {
      const nextExecutions = await workflowsApi.listExecutions(workflow.id);
      setExecutionsByWorkflow((current) => ({ ...current, [workflow.id]: nextExecutions }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to load workflow executions.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleLoadExecutionDetails = async (workflow: WorkflowDefinition, workflowExecution: WorkflowExecution) => {
    setBusyAction(`execution-detail:${workflowExecution.id}`);
    setError(null);

    try {
      const snapshot = await workflowsApi.getExecutionDebugSnapshot(workflow.id, workflowExecution.id);
      setExecutionDebugSnapshotsById((current) => ({ ...current, [snapshot.executionId]: snapshot }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to load workflow execution debug snapshot.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleLoadVersions = async (workflow: WorkflowDefinition) => {
    setBusyAction(`versions:${workflow.id}`);
    setError(null);

    try {
      const versions = await workflowsApi.listWorkflowVersions(workflow.id);
      setVersionsByWorkflow((current) => ({ ...current, [workflow.id]: versions }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to load workflow versions.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleRollbackWorkflow = async (workflow: WorkflowDefinition, version: WorkflowDefinition) => {
    setBusyAction(`rollback:${workflow.id}:${version.version}`);
    setError(null);

    try {
      const rolledBack = await workflowsApi.rollbackWorkflow(workflow.id, { version: version.version });
      setWorkflows((current) =>
        current.map((currentWorkflow) => (currentWorkflow.id === rolledBack.id ? rolledBack : currentWorkflow))
      );
      setVersionsByWorkflow((current) => ({
        ...current,
        [workflow.id]: [...(current[workflow.id] ?? []), rolledBack],
      }));
      syncWorkflowTriggerDraftState(rolledBack.id, rolledBack.definition);
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [rolledBack.id]: formatJson(rolledBack.variables),
      }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to roll back workflow.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleCreateWorkflowBranch = async (workflow: WorkflowDefinition, version: WorkflowDefinition) => {
    const key = workflowBranchDraftKey(workflow.id, version.version);
    const draft = branchDrafts[key] ?? { ...emptyBranchDraft, name: defaultBranchName(workflow, version) };
    const branchName = draft.name.trim();
    if (branchName === '') {
      setError('Branch name is required.');
      return;
    }

    let trafficPercent: number | undefined;
    try {
      trafficPercent = parseOptionalInteger(draft.trafficPercent, 'Traffic percent');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Traffic percent must be a whole number.'));
      return;
    }

    const payload: CreateWorkflowBranchRequest = {
      name: branchName,
      version: version.version,
    };
    const description = draft.description.trim();
    const experimentKey = draft.experimentKey.trim();
    if (description !== '') {
      payload.description = description;
    }
    if (experimentKey !== '') {
      payload.experimentKey = experimentKey;
    }
    if (trafficPercent !== undefined) {
      payload.trafficPercent = trafficPercent;
    }

    setBusyAction(`branch:${workflow.id}:${version.version}`);
    setError(null);

    try {
      const branchedWorkflow = await workflowsApi.createWorkflowBranch(workflow.id, payload);
      setWorkflows((current) => [
        branchedWorkflow,
        ...current.filter((currentWorkflow) => currentWorkflow.id !== branchedWorkflow.id),
      ]);
      syncWorkflowTriggerDraftState(branchedWorkflow.id, branchedWorkflow.definition);
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [branchedWorkflow.id]: formatJson(branchedWorkflow.variables),
      }));
      const versions = await workflowsApi.listWorkflowVersions(workflow.id);
      setVersionsByWorkflow((current) => ({ ...current, [workflow.id]: versions }));
      setBranchForms((current) => ({ ...current, [key]: false }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to create workflow branch.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handlePublishWorkflowBranch = async (workflow: WorkflowDefinition, branch: WorkflowDefinition) => {
    setBusyAction(`publish-branch:${workflow.id}:${branch.id}`);
    setError(null);

    try {
      const publishedBranch = await workflowsApi.publishWorkflowBranch(workflow.id, branch.id, {
        name: branch.name,
      });
      setWorkflows((current) => [
        publishedBranch,
        ...current.filter((currentWorkflow) => currentWorkflow.id !== publishedBranch.id),
      ]);
      syncWorkflowTriggerDraftState(publishedBranch.id, publishedBranch.definition);
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [publishedBranch.id]: formatJson(publishedBranch.variables),
      }));
      const versions = await workflowsApi.listWorkflowVersions(workflow.id);
      setVersionsByWorkflow((current) => ({ ...current, [workflow.id]: versions }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to publish workflow branch.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleMergeWorkflowBranch = async (workflow: WorkflowDefinition, branch: WorkflowDefinition) => {
    setBusyAction(`merge-branch:${workflow.id}:${branch.id}`);
    setError(null);

    try {
      const mergedWorkflow = await workflowsApi.mergeWorkflowBranch(workflow.id, branch.id);
      setWorkflows((current) =>
        current.map((currentWorkflow) => (currentWorkflow.id === mergedWorkflow.id ? mergedWorkflow : currentWorkflow))
      );
      syncWorkflowTriggerDraftState(mergedWorkflow.id, mergedWorkflow.definition);
      setWorkflowVariablesDrafts((current) => ({
        ...current,
        [mergedWorkflow.id]: formatJson(mergedWorkflow.variables),
      }));
      const versions = await workflowsApi.listWorkflowVersions(workflow.id);
      setVersionsByWorkflow((current) => ({ ...current, [workflow.id]: versions }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to merge workflow branch.'));
    } finally {
      setBusyAction(null);
    }
  };

  const replaceExecution = (workflowId: string, nextExecution: WorkflowExecution) => {
    setExecution(nextExecution);
    setExecutionsByWorkflow((current) => ({
      ...current,
      [workflowId]: (current[workflowId] ?? []).map((currentExecution) =>
        currentExecution.id === nextExecution.id ? nextExecution : currentExecution
      ),
    }));
  };

  const handleExecutionAction = async (
    workflow: WorkflowDefinition,
    workflowExecution: WorkflowExecution,
    action: 'pause' | 'resume' | 'cancel'
  ) => {
    setBusyAction(`${action}:${workflowExecution.id}`);
    setError(null);

    try {
      const nextExecution =
        action === 'pause'
          ? await workflowsApi.pauseExecution(workflow.id, workflowExecution.id)
          : action === 'resume'
            ? await workflowsApi.resumeExecution(workflow.id, workflowExecution.id)
            : await workflowsApi.cancelExecution(workflow.id, workflowExecution.id);
      replaceExecution(workflow.id, nextExecution);
    } catch (caughtError) {
      setError(errorMessage(caughtError, `Unable to ${action} workflow execution.`));
    } finally {
      setBusyAction(null);
    }
  };

  const handleResumeWithInput = async (
    workflow: WorkflowDefinition,
    workflowExecution: WorkflowExecution,
    pendingNodeExecution: WorkflowNodeExecution
  ) => {
    const nodeId = pendingNodeExecution.nodeId?.trim();
    if (!nodeId) {
      setError('Pending node ID is required before resuming workflow execution.');
      return;
    }

    let input: Record<string, unknown>;
    const draft =
      pausedInputDrafts[pausedInputDraftKey(workflowExecution.id, nodeId)] ??
      defaultPausedInputDraft(pendingNodeExecution);
    try {
      input = parseJsonObject(draft.inputText);
    } catch {
      setError('Resume input JSON must be a JSON object.');
      return;
    }

    const payload: ResumeWorkflowExecutionRequest = { input, nodeId };
    setBusyAction(`resume-input:${workflowExecution.id}:${nodeId}`);
    setError(null);

    try {
      const nextExecution = await workflowsApi.resumeExecution(workflow.id, workflowExecution.id, payload);
      replaceExecution(workflow.id, nextExecution);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to submit workflow input.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleResolvePausedFailure = async (
    workflow: WorkflowDefinition,
    workflowExecution: WorkflowExecution,
    failedNodeExecution: WorkflowNodeExecution,
    action: ResolvePausedFailureRequest['action'],
    useEditedInput = false
  ) => {
    const nodeId = failedNodeExecution.nodeId?.trim();
    if (!nodeId) {
      setError('Failed node ID is required before resolving a paused workflow failure.');
      return;
    }

    const payload: ResolvePausedFailureRequest = { action, nodeId };
    const draft =
      pausedFailureDecisionDrafts[pausedFailureDecisionDraftKey(workflowExecution.id, nodeId)] ??
      emptyPausedFailureDecisionDraft;
    if (action === 'branch') {
      const nextNodeId = draft.nextNodeId.trim();
      if (nextNodeId === '') {
        setError('Failure branch target is required.');
        return;
      }
      payload.nextNodeId = nextNodeId;
    }
    if (useEditedInput) {
      try {
        payload.input = parseJsonObject(draft.inputText);
      } catch {
        setError('Edited retry input JSON must be a JSON object.');
        return;
      }
    }

    setBusyAction(`decision:${workflowExecution.id}:${nodeId}:${action}${useEditedInput ? ':input' : ''}`);
    setError(null);

    try {
      const nextExecution = await workflowsApi.resolvePausedFailure(workflow.id, workflowExecution.id, payload);
      replaceExecution(workflow.id, nextExecution);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to resolve paused workflow failure.'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleCheckResourceLimits = async (workflow: WorkflowDefinition, workflowExecution: WorkflowExecution) => {
    const draft = resourceCheckDrafts[workflowExecution.id] ?? emptyResourceCheckDraft;
    let payload: CheckWorkflowResourceLimitsRequest;

    try {
      payload = {
        nodeExecutionCount: parseOptionalInteger(draft.nodeExecutionCount, 'Node executions'),
        totalTokens: parseOptionalInteger(draft.totalTokens, 'Total tokens'),
      };
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Resource limits must use whole numbers.'));
      return;
    }

    setBusyAction(`resource-check:${workflowExecution.id}`);
    setError(null);

    try {
      const nextExecution = await workflowsApi.checkWorkflowResourceLimits(
        workflow.id,
        workflowExecution.id,
        payload
      );
      replaceExecution(workflow.id, nextExecution);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to check workflow execution resource limits.'));
    } finally {
      setBusyAction(null);
    }
  };

  return (
    <main aria-labelledby="workflows-title" className="mx-auto max-w-6xl space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Workspace automation</p>
        <h1 className="font-heading text-3xl font-semibold text-[#181611]" id="workflows-title">
          Workflows
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-[#625b4f]">
          Draft a manual workflow, review definitions, test individual nodes, and control recent executions.
        </p>
      </header>

      <section aria-label="Create workflow" className="rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] p-5">
        <h2 className="text-base font-semibold">Create draft</h2>
        <form
          className="mt-4 grid gap-4 md:grid-cols-[minmax(0,1fr)_auto]"
          onSubmit={(event) => {
            event.preventDefault();
            void handleCreateDraft();
          }}
        >
          <label className="block text-sm font-medium" htmlFor="workflow-name">
            Workflow name
            <input
              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
              id="workflow-name"
              name="workflow-name"
              onChange={(event) => setWorkflowName(event.target.value)}
              placeholder="Manual review"
              type="text"
              value={workflowName}
            />
          </label>
          <div className="flex items-end">
            <button
              className="min-h-11 rounded-lg bg-[#181611] px-4 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
              disabled={isCreating || workflowName.trim() === ''}
              type="submit"
            >
              {isCreating ? 'Creating...' : 'Create draft workflow'}
            </button>
          </div>
          <label className="block text-sm font-medium md:col-span-2" htmlFor="workflow-variables">
            Workflow variables JSON
            <textarea
              className="mt-2 min-h-24 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm"
              id="workflow-variables"
              onChange={(event) => setWorkflowCreateVariablesText(event.target.value)}
              value={workflowCreateVariablesText}
            />
          </label>
        </form>
      </section>

      <section aria-label="Workflow trigger matching" className="rounded-lg border border-[#d7d2c4] bg-white p-5">
        <h2 className="text-base font-semibold">Trigger matching</h2>
        <div className="mt-3 grid gap-4 lg:grid-cols-2">
          <form
            aria-label="Conversation trigger matching"
            className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3"
            onSubmit={(event) => {
              event.preventDefault();
              void handleMatchConversationTriggers();
            }}
          >
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h3 className="text-sm font-semibold text-[#181611]">Conversation trigger matching</h3>
              <button
                className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                disabled={busyAction === 'conversation-matches'}
                type="submit"
              >
                {busyAction === 'conversation-matches' ? 'Checking...' : 'Check conversation matches'}
              </button>
            </div>
            <label className="mt-3 block text-sm font-medium" htmlFor="conversation-match-id">
              Conversation match ID
              <input
                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm"
                id="conversation-match-id"
                onChange={(event) => setConversationMatchId(event.target.value)}
                placeholder="conversation_123"
                type="text"
                value={conversationMatchId}
              />
            </label>
            {conversationMatchResults ? (
              <div aria-label="Conversation trigger match results" className="mt-4 rounded-lg border border-[#e4dfd2] bg-white p-3">
                {conversationMatchResults.length === 0 ? (
                  <p className="text-sm text-[#625b4f]">No conversation trigger matches.</p>
                ) : (
                  <ol className="space-y-2">
                    {conversationMatchResults.map((match, index) => (
                      <li
                        className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] px-3 py-2"
                        key={`${match.workflowId}:${match.triggerId ?? index}:${match.conversationId}`}
                      >
                        <p className="text-sm font-semibold text-[#181611]">
                          {match.workflowName}
                          {match.workflowVersion !== undefined ? ` v${match.workflowVersion}` : ''}
                        </p>
                        <p className="mt-1 break-all font-mono text-xs text-[#625b4f]">
                          {formatConversationTriggerMatch(match)}
                        </p>
                      </li>
                    ))}
                  </ol>
                )}
              </div>
            ) : null}
          </form>

          <form
            aria-label="Semantic trigger matching"
            className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3"
            onSubmit={(event) => {
              event.preventDefault();
              void handleMatchSemanticTriggers();
            }}
          >
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h3 className="text-sm font-semibold text-[#181611]">Semantic trigger matching</h3>
              <button
                className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                disabled={busyAction === 'semantic-matches'}
                type="submit"
              >
                {busyAction === 'semantic-matches' ? 'Checking...' : 'Check semantic matches'}
              </button>
            </div>
            <label className="mt-3 block text-sm font-medium" htmlFor="semantic-match-message">
              Semantic match message
              <textarea
                className="mt-2 min-h-24 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                id="semantic-match-message"
                onChange={(event) => setSemanticMatchMessage(event.target.value)}
                value={semanticMatchMessage}
              />
            </label>
            {semanticMatchResults ? (
              <div aria-label="Semantic trigger match results" className="mt-4 rounded-lg border border-[#e4dfd2] bg-white p-3">
                {semanticMatchResults.length === 0 ? (
                  <p className="text-sm text-[#625b4f]">No semantic trigger matches.</p>
                ) : (
                  <ol className="space-y-2">
                    {semanticMatchResults.map((match, index) => (
                      <li
                        className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] px-3 py-2"
                        key={`${match.workflowId}:${match.triggerId ?? index}:${match.keyword}`}
                      >
                        <p className="text-sm font-semibold text-[#181611]">
                          {match.workflowName}
                          {match.workflowVersion !== undefined ? ` v${match.workflowVersion}` : ''}
                        </p>
                        <p className="mt-1 break-all font-mono text-xs text-[#625b4f]">
                          {formatSemanticTriggerMatch(match)}
                        </p>
                      </li>
                    ))}
                  </ol>
                )}
              </div>
            ) : null}
          </form>
        </div>
      </section>

      {error ? (
        <p className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800" role="alert">
          {error}
        </p>
      ) : null}
      {execution ? (
        <p className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
          Execution {execution.id} status: {formatExecutionStatus(execution.status)}.
        </p>
      ) : null}
      {scheduledTaskRun ? (
        <p className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
          Scheduled task run {scheduledTaskRun.id} status: {scheduledTaskRun.status}.
        </p>
      ) : null}

      <section aria-label="Workflow list" className="rounded-lg border border-[#d7d2c4] bg-white p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-base font-semibold">Workflow list</h2>
          <p className="text-sm text-[#625b4f]">{workflows.length === 1 ? '1 workflow' : `${workflows.length} workflows`}</p>
        </div>

        {isLoading ? <p className="mt-5 text-sm text-[#625b4f]">Loading workflows...</p> : null}
        {!isLoading && workflows.length === 0 ? (
          <p className="mt-5 text-sm text-[#625b4f]">No workflows yet. Create a draft workflow to start.</p>
        ) : null}
        {workflows.length > 0 ? (
          <ul className="mt-5 space-y-4">
            {workflows.map((workflow) => {
              const debugDraft = debugDrafts[workflow.id] ?? emptyDebugDraft;
              const workflowNodes = getWorkflowNodes(workflow);
              const workflowEdges = getWorkflowEdges(workflow, workflowNodes);
              const activeWorkflowEdges = workflowEdges.filter((edge) => edge.status === 'active');
              const invalidWorkflowEdges = workflowEdges.filter((edge) => edge.status === 'invalid');
              const nodeIds = workflowNodes.map((node) => node.id);
              const nodeResult = nodeResults[workflow.id];
              const workflowExecutions = executionsByWorkflow[workflow.id] ?? [];
              const workflowScheduleTasks = workflowScheduledTasks(scheduledTasks, workflow.id);
              const latestWorkflowExecution = workflowExecutions[0];
              const nodeExecutionMap = buildNodeExecutionMap(latestWorkflowExecution);
              const selectedNode = workflowNodes.find((node) => node.id === selectedNodeIds[workflow.id]);
              const selectedNodeExecution = selectedNode ? nodeExecutionMap.get(selectedNode.id) : undefined;
              const selectedNodeStatus = selectedNode ? visualNodeStatus(selectedNode, selectedNodeExecution) : undefined;
              const selectedNodeFailureStrategy = selectedNode
                ? (nodeFailureStrategyDrafts[nodeFailureStrategyDraftKey(workflow.id, selectedNode.id)] ??
                  selectedNode.failureStrategy)
                : undefined;
              const selectedNodeFailurePolicyDraft = selectedNode
                ? (nodeFailurePolicyDrafts[nodeFailurePolicyDraftKey(workflow.id, selectedNode.id)] ??
                  nodeFailurePolicyDraftFromNode(selectedNode))
                : undefined;
              const selectedIncomingEdges = selectedNode
                ? activeWorkflowEdges.filter((edge) => edge.target === selectedNode.id)
                : [];
              const selectedOutgoingEdges = selectedNode
                ? activeWorkflowEdges.filter((edge) => edge.source === selectedNode.id)
                : [];
              const workflowTriggers = getWorkflowTriggers(workflow.definition);
              const workflowVersions = versionsByWorkflow[workflow.id] ?? [];
              const conversationTriggerDraft =
                conversationTriggerDrafts[workflow.id] ?? conversationTriggerDraftFromTriggers(workflowTriggers);
              const scheduleTriggerDraft =
                scheduleTriggerDrafts[workflow.id] ?? scheduleTriggerDraftFromTriggers(workflowTriggers);
              const semanticTriggerDraft =
                semanticTriggerDrafts[workflow.id] ?? semanticTriggerDraftFromTriggers(workflowTriggers);
              const webhookTriggerDraft =
                webhookTriggerDrafts[workflow.id] ?? webhookTriggerDraftFromTriggers(workflowTriggers);
              const signedWebhookPath = buildSignedWebhookPath(workflow, webhookTriggerDraft);
              const signedWebhookHasSecret = hasSignedWebhookSecret(workflow, webhookTriggerDraft);
              const signedWebhookCommand = signedWebhookCurlCommand(signedWebhookPath);
              const workflowRunInputText = workflowRunInputDrafts[workflow.id] ?? '{}';
              const workflowWebhookPayloadText = workflowWebhookPayloadDrafts[workflow.id] ?? '{}';
              const workflowTriggersText = workflowTriggerDrafts[workflow.id] ?? formatJson(workflowTriggers);
              const workflowVariablesText = workflowVariablesDrafts[workflow.id] ?? formatJson(workflow.variables);
              const workflowDefinitionDraft = workflowDefinitionDrafts[workflow.id] ?? emptyWorkflowDefinitionDraft;
              const workflowResourcePolicyDraft =
                workflowResourcePolicyDrafts[workflow.id] ??
                workflowResourcePolicyDraftFromDefinition(workflow.definition);
              const snapEnabled = snapToGridByWorkflow[workflow.id] ?? true;
              const reactFlowNodes = buildWorkflowReactFlowNodes(
                workflow.id,
                workflowNodes,
                nodeExecutionMap,
                selectedNode?.id,
                snapEnabled,
                handleSelectNode
              );
              const reactFlowEdges = buildWorkflowReactFlowEdges(activeWorkflowEdges);

              return (
                <li className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-4" key={workflow.id}>
                  <article>
                    <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-start">
                      <div>
                        <h3 className="text-base font-semibold text-[#181611]">{workflow.name}</h3>
                        {workflow.description ? (
                          <p className="mt-2 text-sm leading-6 text-[#625b4f]">{workflow.description}</p>
                        ) : null}
                        <div className="mt-3 flex flex-wrap gap-2 text-xs font-semibold text-[#625b4f]">
                          <span className="rounded-lg bg-white px-3 py-1">Status: {workflow.status}</span>
                          <span className="rounded-lg bg-white px-3 py-1">Version: {workflow.version}</span>
                          <span className="rounded-lg bg-white px-3 py-1">{describeNodeCount(workflow)}</span>
                        </div>
                        <div className="mt-3 flex flex-wrap gap-2">
                          <button
                            aria-label={`Publish ${workflow.name}`}
                            className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                            disabled={
                              workflow.status === 'published' ||
                              workflow.status === 'archived' ||
                              busyAction === `publish:${workflow.id}`
                            }
                            onClick={() => void handlePublishWorkflow(workflow)}
                            type="button"
                          >
                            {busyAction === `publish:${workflow.id}` ? 'Publishing...' : 'Publish'}
                          </button>
                          <button
                            aria-label={`Archive ${workflow.name}`}
                            className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#6f2d1f] transition hover:border-[#6f2d1f] hover:bg-[#f7ece8] disabled:cursor-not-allowed disabled:opacity-50"
                            disabled={workflow.status === 'archived' || busyAction === `archive:${workflow.id}`}
                            onClick={() => void handleArchiveWorkflow(workflow)}
                            type="button"
                          >
                            {busyAction === `archive:${workflow.id}` ? 'Archiving...' : 'Archive'}
                          </button>
                        </div>
                      </div>
                      <div className="min-w-0 md:w-80">
                        <label
                          className="block text-xs font-semibold text-[#625b4f]"
                          htmlFor={`workflow-run-input-${workflow.id}`}
                        >
                          Run input JSON for {workflow.name}
                          <textarea
                            className="mt-2 min-h-20 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-xs text-[#181611]"
                            id={`workflow-run-input-${workflow.id}`}
                            onChange={(event) => updateWorkflowRunInputDraft(workflow, event.target.value)}
                            value={workflowRunInputText}
                          />
                        </label>
                        <button
                          className="mt-2 min-h-10 w-full rounded-lg border border-[#cfc8b7] bg-white px-4 text-sm font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                          disabled={runningWorkflowId === workflow.id}
                          onClick={() => void handleExecuteWorkflow(workflow)}
                          type="button"
                        >
                          {runningWorkflowId === workflow.id ? 'Running...' : `Run ${workflow.name}`}
                        </button>
                        <label
                          className="mt-3 block text-xs font-semibold text-[#625b4f]"
                          htmlFor={`workflow-webhook-payload-${workflow.id}`}
                        >
                          Webhook payload JSON for {workflow.name}
                          <textarea
                            className="mt-2 min-h-20 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-xs text-[#181611]"
                            id={`workflow-webhook-payload-${workflow.id}`}
                            onChange={(event) => updateWorkflowWebhookPayloadDraft(workflow, event.target.value)}
                            value={workflowWebhookPayloadText}
                          />
                        </label>
                        <button
                          aria-label={`Trigger webhook for ${workflow.name}`}
                          className="mt-2 min-h-10 w-full rounded-lg border border-[#cfc8b7] bg-white px-4 text-sm font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                          disabled={busyAction === `webhook:${workflow.id}`}
                          onClick={() => void handleTriggerWorkflowWebhook(workflow)}
                          type="button"
                        >
                          {busyAction === `webhook:${workflow.id}` ? 'Triggering webhook...' : 'Trigger webhook'}
                        </button>
                      </div>
                    </div>

                    <form
                      aria-label={`Variables ${workflow.name}`}
                      className="mt-4 rounded-lg border border-[#d7d2c4] bg-white p-4"
                      onSubmit={(event) => {
                        event.preventDefault();
                        void handleSaveWorkflowVariables(workflow);
                      }}
                    >
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <h4 className="text-sm font-semibold text-[#181611]">Variables</h4>
                        <button
                          aria-label={`Save variables for ${workflow.name}`}
                          className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                          disabled={busyAction === `variables:${workflow.id}`}
                          type="submit"
                        >
                          {busyAction === `variables:${workflow.id}` ? 'Saving...' : 'Save variables'}
                        </button>
                      </div>
                      <label className="mt-3 block text-sm font-medium" htmlFor={`workflow-variables-${workflow.id}`}>
                        Variables JSON for {workflow.name}
                        <textarea
                          className="mt-2 min-h-28 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 font-mono text-sm"
                          id={`workflow-variables-${workflow.id}`}
                          onChange={(event) => updateWorkflowVariablesDraft(workflow, event.target.value)}
                          value={workflowVariablesText}
                        />
                      </label>
                    </form>

                    <form
                      aria-label={`Triggers ${workflow.name}`}
                      className="mt-4 rounded-lg border border-[#d7d2c4] bg-white p-4"
                      onSubmit={(event) => {
                        event.preventDefault();
                        void handleSaveWorkflowTriggers(workflow);
                      }}
                    >
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <h4 className="text-sm font-semibold text-[#181611]">Triggers</h4>
                        <button
                          aria-label={`Save triggers for ${workflow.name}`}
                          className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                          disabled={busyAction === `triggers:${workflow.id}`}
                          type="submit"
                        >
                          {busyAction === `triggers:${workflow.id}` ? 'Saving...' : 'Save triggers'}
                        </button>
                      </div>
                      <div className="mt-3 grid gap-2 text-xs font-semibold text-[#625b4f] md:grid-cols-2">
                        {workflowTriggerKinds.map((triggerKind) => (
                          <p className="break-all rounded-lg bg-[#fbfaf7] px-3 py-2" key={triggerKind.value}>
                            {triggerKind.label}: {formatTriggerValue(triggerKind.value, workflowTriggers[triggerKind.value])}
                          </p>
                        ))}
                      </div>
                      <div className="mt-3 rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3">
                        <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                          Conversation trigger
                        </p>
                        <div className="mt-3 grid gap-3 md:grid-cols-2">
                          <label
                            className="block text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-conversation-trigger-id-${workflow.id}`}
                          >
                            Conversation trigger ID for {workflow.name}
                            <input
                              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                              id={`workflow-conversation-trigger-id-${workflow.id}`}
                              onChange={(event) =>
                                updateConversationTriggerDraft(workflow, { id: event.target.value })
                              }
                              placeholder="conversation-main"
                              type="text"
                              value={conversationTriggerDraft.id}
                            />
                          </label>
                          <label
                            className="block text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-conversation-id-${workflow.id}`}
                          >
                            Conversation ID for {workflow.name}
                            <input
                              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                              id={`workflow-conversation-id-${workflow.id}`}
                              onChange={(event) =>
                                updateConversationTriggerDraft(workflow, { conversationId: event.target.value })
                              }
                              placeholder="conversation_123"
                              type="text"
                              value={conversationTriggerDraft.conversationId}
                            />
                          </label>
                        </div>
                      </div>
                      <div className="mt-3 rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3">
                        <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                          Schedule trigger
                        </p>
                        <div className="mt-3 grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
                          <label
                            className="block text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-schedule-id-${workflow.id}`}
                          >
                            Schedule trigger ID for {workflow.name}
                            <input
                              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                              id={`workflow-schedule-id-${workflow.id}`}
                              onChange={(event) =>
                                updateScheduleTriggerDraft(workflow, { id: event.target.value })
                              }
                              placeholder="daily-report"
                              type="text"
                              value={scheduleTriggerDraft.id}
                            />
                          </label>
                          <label
                            className="block text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-schedule-cron-${workflow.id}`}
                          >
                            Schedule cron for {workflow.name}
                            <input
                              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                              id={`workflow-schedule-cron-${workflow.id}`}
                              onChange={(event) =>
                                updateScheduleTriggerDraft(workflow, { cron: event.target.value })
                              }
                              placeholder="0 9 * * 1"
                              type="text"
                              value={scheduleTriggerDraft.cron}
                            />
                          </label>
                          <label
                            className="flex min-h-16 items-end gap-2 text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-schedule-enabled-${workflow.id}`}
                          >
                            <input
                              checked={scheduleTriggerDraft.enabled}
                              className="mb-2 size-4 rounded border-[#d7d2c4]"
                              id={`workflow-schedule-enabled-${workflow.id}`}
                              onChange={(event) =>
                                updateScheduleTriggerDraft(workflow, { enabled: event.target.checked })
                              }
                              type="checkbox"
                            />
                            <span className="pb-2">Schedule enabled for {workflow.name}</span>
                          </label>
                        </div>
                      </div>
                      <div className="mt-3 rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3">
                        <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                          Semantic trigger
                        </p>
                        <div className="mt-3 grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)_8rem]">
                          <label
                            className="block text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-semantic-trigger-id-${workflow.id}`}
                          >
                            Semantic trigger ID for {workflow.name}
                            <input
                              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                              id={`workflow-semantic-trigger-id-${workflow.id}`}
                              onChange={(event) =>
                                updateSemanticTriggerDraft(workflow, { id: event.target.value })
                              }
                              placeholder="urgent-ticket"
                              type="text"
                              value={semanticTriggerDraft.id}
                            />
                          </label>
                          <label
                            className="block text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-semantic-keywords-${workflow.id}`}
                          >
                            Semantic keywords for {workflow.name}
                            <input
                              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                              id={`workflow-semantic-keywords-${workflow.id}`}
                              onChange={(event) =>
                                updateSemanticTriggerDraft(workflow, { keywordsText: event.target.value })
                              }
                              placeholder="incident, sev1"
                              type="text"
                              value={semanticTriggerDraft.keywordsText}
                            />
                          </label>
                          <label
                            className="block text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-semantic-threshold-${workflow.id}`}
                          >
                            Semantic threshold for {workflow.name}
                            <input
                              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                              id={`workflow-semantic-threshold-${workflow.id}`}
                              inputMode="decimal"
                              onChange={(event) =>
                                updateSemanticTriggerDraft(workflow, { threshold: event.target.value })
                              }
                              placeholder="0.85"
                              type="text"
                              value={semanticTriggerDraft.threshold}
                            />
                          </label>
                        </div>
                      </div>
                      <div className="mt-3 rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3">
                        <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                          Webhook trigger
                        </p>
                        <div className="mt-3 grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)_minmax(0,1fr)]">
                          <label
                            className="block text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-webhook-trigger-id-${workflow.id}`}
                          >
                            Webhook trigger ID for {workflow.name}
                            <input
                              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                              id={`workflow-webhook-trigger-id-${workflow.id}`}
                              onChange={(event) =>
                                updateWebhookTriggerDraft(workflow, { id: event.target.value })
                              }
                              placeholder="github"
                              type="text"
                              value={webhookTriggerDraft.id}
                            />
                          </label>
                          <label
                            className="block text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-webhook-path-${workflow.id}`}
                          >
                            Webhook path for {workflow.name}
                            <input
                              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                              id={`workflow-webhook-path-${workflow.id}`}
                              onChange={(event) =>
                                updateWebhookTriggerDraft(workflow, { path: event.target.value })
                              }
                              placeholder={`/api/v1/workflows/webhooks/{org}/${workflow.id}`}
                              type="text"
                              value={webhookTriggerDraft.path}
                            />
                          </label>
                          <label
                            className="block text-xs font-semibold text-[#625b4f]"
                            htmlFor={`workflow-webhook-secret-${workflow.id}`}
                          >
                            Webhook secret for {workflow.name}
                            <input
                              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                              id={`workflow-webhook-secret-${workflow.id}`}
                              onChange={(event) =>
                                updateWebhookTriggerDraft(workflow, { secret: event.target.value })
                              }
                              placeholder="shared secret"
                              type="password"
                              value={webhookTriggerDraft.secret}
                            />
                          </label>
                        </div>
                        <section
                          aria-label={`Signed webhook helper for ${workflow.name}`}
                          className="mt-3 border-t border-[#e4dfd2] pt-3"
                        >
                          <div className="flex flex-wrap items-start justify-between gap-3">
                            <div>
                              <div className="flex items-center gap-2 text-sm font-semibold text-[#181611]">
                                <RiWebhookLine className="size-4 text-[#1a614f]" aria-hidden="true" />
                                <span>Public signed webhook</span>
                              </div>
                              <p className="mt-1 text-xs leading-5 text-[#625b4f]">
                                {webhookTriggerDraft.path.trim() === ''
                                  ? 'Webhook path is not configured.'
                                  : 'Signed calls can use the public route without a login session.'}
                              </p>
                            </div>
                            <div className="flex flex-wrap gap-2 text-xs font-semibold text-[#625b4f]">
                              <span className="inline-flex items-center gap-1 rounded-lg bg-white px-2 py-1">
                                <RiKey2Line className="size-3.5" aria-hidden="true" />
                                X-Oblivious-Timestamp
                              </span>
                              <span className="inline-flex items-center gap-1 rounded-lg bg-white px-2 py-1">
                                <RiShieldKeyholeLine className="size-3.5" aria-hidden="true" />
                                X-Oblivious-Signature
                              </span>
                            </div>
                          </div>
                          <div className="mt-3 grid gap-3 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
                            <div className="min-w-0 rounded-lg border border-[#e4dfd2] bg-white p-3">
                              <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                Public path
                              </p>
                              <p className="mt-2 break-all font-mono text-xs text-[#181611]">{signedWebhookPath}</p>
                              <p className="mt-3 text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                Signature
                              </p>
                              <p className="mt-2 font-mono text-xs text-[#181611]">
                                HMAC-SHA256(timestamp + "." + raw_body, webhook secret)
                              </p>
                              {!signedWebhookHasSecret ? (
                                <p className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs font-semibold text-amber-800">
                                  Webhook secret is required before public signed calls work.
                                </p>
                              ) : null}
                            </div>
                            <div className="min-w-0 rounded-lg border border-[#2f2b24] bg-[#181611] p-3 text-[#f7f1df]">
                              <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-[#d7cfae]">
                                <RiCodeBoxLine className="size-4" aria-hidden="true" />
                                curl template
                              </div>
                              <pre className="mt-3 max-h-64 overflow-auto whitespace-pre-wrap break-words font-mono text-xs leading-5">
                                {signedWebhookCommand}
                              </pre>
                            </div>
                          </div>
                        </section>
                      </div>
                      <label className="mt-3 block text-sm font-medium" htmlFor={`workflow-triggers-${workflow.id}`}>
                        Triggers JSON for {workflow.name}
                        <textarea
                          className="mt-2 min-h-36 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 font-mono text-sm"
                          id={`workflow-triggers-${workflow.id}`}
                          onChange={(event) => updateWorkflowTriggerDraft(workflow, event.target.value)}
                          value={workflowTriggersText}
                        />
                      </label>
                    </form>

                    <section
                      aria-label={`Scheduled tasks for ${workflow.name}`}
                      className="mt-4 rounded-lg border border-[#d7d2c4] bg-white p-4"
                    >
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <h4 className="text-sm font-semibold text-[#181611]">Scheduled tasks</h4>
                        <p className="text-xs font-semibold text-[#625b4f]">
                          {workflowScheduleTasks.length === 1
                            ? '1 synced task'
                            : `${workflowScheduleTasks.length} synced tasks`}
                        </p>
                      </div>
                      {workflowScheduleTasks.length === 0 ? (
                        <p className="mt-3 text-sm text-[#625b4f]">No synced scheduled tasks.</p>
                      ) : (
                        <ul className="mt-3 space-y-2">
                          {workflowScheduleTasks.map((task) => (
                            <li
                              className="grid gap-3 rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-start"
                              key={task.id}
                            >
                              <div className="space-y-3">
                                <div className="grid gap-2 text-xs font-semibold text-[#625b4f] md:grid-cols-4">
                                  <p className="break-all rounded-lg bg-white px-3 py-2">{task.id}</p>
                                  <p className="break-all rounded-lg bg-white px-3 py-2">
                                    {task.workflowTriggerId ?? 'schedule'}
                                  </p>
                                  <p className="break-all rounded-lg bg-white px-3 py-2">{task.cronExpression}</p>
                                  <p className="break-all rounded-lg bg-white px-3 py-2">
                                    Next: {task.nextRunAt ?? 'not scheduled'}
                                  </p>
                                </div>
                                {(scheduledTaskRunsByTask[task.id] ?? []).length > 0 ? (
                                  <div className="rounded-lg border border-[#e4dfd2] bg-white px-3 py-2">
                                    <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                      Recent runs
                                    </p>
                                    <ul className="mt-2 space-y-1">
                                      {(scheduledTaskRunsByTask[task.id] ?? []).map((run) => (
                                        <li
                                          className="flex flex-wrap items-center gap-2 text-xs font-semibold text-[#625b4f]"
                                          key={run.id}
                                        >
                                          <span className="break-all font-mono text-[#181611]">{run.id}</span>
                                          <span className="rounded-md bg-[#eef5f1] px-2 py-0.5 text-[#1a614f]">
                                            {formatScheduledTaskRunStatus(run.status)}
                                          </span>
                                        </li>
                                      ))}
                                    </ul>
                                  </div>
                                ) : null}
                              </div>
                              <button
                                aria-label={`Run scheduled task ${task.id} now`}
                                className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                                disabled={busyAction === `scheduled-task:${task.id}:run`}
                                onClick={() => void handleRunScheduledTaskNow(task)}
                                type="button"
                              >
                                {busyAction === `scheduled-task:${task.id}:run` ? 'Running...' : 'Run now'}
                              </button>
                            </li>
                          ))}
                        </ul>
                      )}
                    </section>

                    <div
                      aria-label={`Visual editor for ${workflow.name}`}
                      className="mt-4 rounded-lg border border-[#d7d2c4] bg-white p-4"
                    >
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <h4 className="text-sm font-semibold text-[#181611]">Visual editor</h4>
                        <div className="flex flex-wrap items-center gap-3 text-xs font-semibold text-[#625b4f]">
                          <span className="rounded-lg bg-[#fbfaf7] px-3 py-1">Grid: {workflowCanvasGridSize}px</span>
                          <label className="inline-flex items-center gap-2 rounded-lg bg-[#fbfaf7] px-3 py-1" htmlFor={`workflow-snap-grid-${workflow.id}`}>
                            <input
                              checked={snapToGridByWorkflow[workflow.id] ?? true}
                              className="size-4 rounded border-[#d7d2c4]"
                              id={`workflow-snap-grid-${workflow.id}`}
                              onChange={(event) =>
                                setSnapToGridByWorkflow((current) => ({
                                  ...current,
                                  [workflow.id]: event.target.checked,
                                }))
                              }
                              type="checkbox"
                            />
                            <span>Snap to grid for {workflow.name}</span>
                          </label>
                          <button
                            aria-label={`Auto arrange nodes for ${workflow.name}`}
                            className="min-h-8 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                            disabled={busyAction === `layout:${workflow.id}` || workflowNodes.length === 0}
                            onClick={() => void handleAutoArrangeWorkflow(workflow)}
                            type="button"
                          >
                            {busyAction === `layout:${workflow.id}` ? 'Arranging...' : 'Auto arrange nodes'}
                          </button>
                        </div>
                      </div>

                      {workflowNodes.length === 0 ? (
                        <p className="mt-3 rounded-lg border border-dashed border-[#cfc8b7] bg-[#fbfaf7] px-4 py-5 text-sm text-[#625b4f]">
                          No nodes in this workflow definition yet.
                        </p>
                      ) : (
                        <div className="mt-4 grid gap-3 xl:grid-cols-[220px_minmax(0,1fr)_minmax(260px,340px)]">
                          <aside
                            aria-label={`Node palette for ${workflow.name}`}
                            className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3"
                          >
                            <div className="flex items-center justify-between gap-2">
                              <h5 className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Node palette</h5>
                              <span className="font-mono text-xs font-semibold text-[#625b4f]">{workflowNodePalette.length}</span>
                            </div>
                            <div className="mt-3 grid grid-cols-2 gap-2 xl:grid-cols-1">
                              {workflowNodePalette.map((paletteNode) => (
                                <button
                                  aria-label={`Add ${paletteNode.label} node template to ${workflow.name}`}
                                  className="min-h-9 rounded-lg border border-[#d7d2c4] bg-white px-3 text-left text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee]"
                                  data-node-type={paletteNode.type}
                                  draggable
                                  key={paletteNode.type}
                                  onDragStart={(event) => handleWorkflowPaletteDragStart(event, paletteNode.type)}
                                  onClick={() => updateWorkflowDefinitionDraft(workflow.id, { nodeType: paletteNode.type })}
                                  title={`${paletteNode.label} node`}
                                  type="button"
                                >
                                  <span>{paletteNode.label}</span>
                                  <span className="ml-2 font-mono text-[11px] text-[#6d6658]">{paletteNode.type}</span>
                                </button>
                              ))}
                            </div>
                          </aside>

                          <div className="min-w-0 space-y-3">
                            <div
                              aria-label={`React Flow canvas for ${workflow.name}`}
                              className="relative min-h-[360px] overflow-auto rounded-lg border border-[#d7d2c4] bg-[#fbfaf7]"
                              onDragOver={(event) => {
                                event.preventDefault();
                                if (event.dataTransfer) {
                                  event.dataTransfer.dropEffect = 'copy';
                                }
                              }}
                              onDrop={(event) => handleWorkflowCanvasDrop(event, workflow, workflowNodes)}
                            >
                              <div className="h-[520px] min-w-[820px]">
                                <ReactFlowProvider>
                                  <ReactFlow
                                    edges={reactFlowEdges}
                                    fitView
                                    fitViewOptions={{ padding: 0.2 }}
                                    nodeTypes={workflowReactFlowNodeTypes}
                                    nodes={reactFlowNodes}
                                    nodesDraggable
                                    onConnect={(connection) => handleWorkflowCanvasConnect(workflow, connection)}
                                    onNodeClick={(_event, reactFlowNode) => {
                                      handleSelectNode(workflow.id, reactFlowNode.data.node);
                                    }}
                                    onNodeDragStop={(_event, reactFlowNode) => {
                                      handleWorkflowCanvasNodeDragStop(workflow, reactFlowNode.data.node.id, {
                                        x: reactFlowNode.position.x,
                                        y: reactFlowNode.position.y,
                                      });
                                    }}
                                    snapGrid={workflowReactFlowSnapGrid}
                                    snapToGrid={snapEnabled}
                                  >
                                    <Background color="#e4dfd2" gap={workflowCanvasGridSize} size={1} />
                                    <Controls position="top-right" />
                                    <MiniMap
                                      maskColor="rgba(251, 250, 247, 0.78)"
                                      nodeColor="#d7d2c4"
                                      pannable
                                      position="bottom-right"
                                      zoomable
                                    />
                                  </ReactFlow>
                                </ReactFlowProvider>
                                <div className="absolute bottom-3 left-3 right-3 flex flex-wrap gap-2">
                                  {activeWorkflowEdges.map((edge) => (
                                    <span
                                      aria-label={`Canvas edge ${edge.source} to ${edge.target}${edge.branch ? ` branch ${edge.branch}` : ''}`}
                                      className="rounded-md border border-[#d7d2c4] bg-white/90 px-2 py-1 font-mono text-[11px] font-semibold text-[#625b4f]"
                                      key={`canvas-edge-label-${edge.id}`}
                                    >
                                      {edge.source} to {edge.target}
                                      {edge.branch ? ` / ${edge.branch}` : ''}
                                    </span>
                                  ))}
                                </div>
                              </div>
                            </div>

                            <ol className="grid gap-3 md:grid-cols-3" aria-label={`Node sequence for ${workflow.name}`}>
                              {workflowNodes.map((node, index) => {
                                const isSelected = selectedNode?.id === node.id;
                                const nodeExecution = nodeExecutionMap.get(node.id);
                                const hasNodeExecution = nodeExecution !== undefined;
                                const nodeStatus = visualNodeStatus(node, nodeExecution);
                                const statusBadgeClass = visualNodeBadgeClass(nodeStatus, hasNodeExecution);

                                return (
                                  <li className="min-w-0" key={`${workflow.id}-${node.id}-${index}`}>
                                    <button
                                      aria-label={visualNodeAriaLabel(node, index, nodeExecution)}
                                      aria-pressed={isSelected}
                                      className={`flex h-full min-h-28 w-full flex-col items-start justify-between rounded-lg border p-3 text-left transition ${
                                        isSelected
                                          ? `${visualNodeCardClass(nodeStatus, hasNodeExecution)} ring-2 ring-[#1a614f] ring-offset-1`
                                          : visualNodeCardClass(nodeStatus, hasNodeExecution)
                                      }`}
                                      onClick={() => handleSelectNode(workflow.id, node)}
                                      type="button"
                                    >
                                      <span className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                        Node {index + 1}
                                      </span>
                                      <span className="mt-2 break-all font-mono text-sm font-semibold text-[#181611]">
                                        {node.id}
                                      </span>
                                      <span className="mt-3 flex flex-wrap gap-2 text-xs font-semibold">
                                        <span className="rounded-lg border border-[#d7d2c4] bg-white px-2 py-1 text-[#625b4f]">
                                          {node.type}
                                        </span>
                                        <span className={`rounded-lg border px-2 py-1 ${statusBadgeClass}`}>
                                          {nodeStatus}
                                        </span>
                                        {hasNodeExecution ? (
                                          <span className={`rounded-lg border px-2 py-1 ${statusBadgeClass}`}>
                                            {formatDuration(nodeExecution.durationMs)}
                                          </span>
                                        ) : null}
                                      </span>
                                    </button>
                                  </li>
                                );
                              })}
                            </ol>

                            {workflowEdges.length > 0 ? (
                              <div
                                aria-label={`Edges for ${workflow.name}`}
                                className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3"
                              >
                                <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                  Edges: {activeWorkflowEdges.length} active, {invalidWorkflowEdges.length} invalid
                                </p>
                                <ul className="mt-2 space-y-2 text-xs font-mono text-[#181611]">
                                  {activeWorkflowEdges.map((edge) => (
                                    <li
                                      className="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-white px-3 py-2"
                                      key={edge.id}
                                    >
                                      <span className="break-all">
                                        {edge.source} -&gt; {edge.target}
                                        {edge.branch ? ` [${edge.branch}]` : ''}
                                      </span>
                                      <button
                                        aria-label={`Remove edge ${edge.source} to ${edge.target} from ${workflow.name}`}
                                        className="min-h-7 rounded-lg border border-[#cfc8b7] bg-white px-2 text-[11px] font-semibold text-[#625b4f] transition hover:border-red-300 hover:bg-red-50 hover:text-red-800"
                                        onClick={() => handleRemoveWorkflowEdge(workflow, edge)}
                                        type="button"
                                      >
                                        Remove
                                      </button>
                                    </li>
                                  ))}
                                  {invalidWorkflowEdges.map((edge) => (
                                    <li
                                      className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-amber-800"
                                      key={edge.id}
                                    >
                                      <span className="break-all">
                                        Invalid edge: {edge.source} -&gt; {edge.target}
                                        {edge.branch ? ` [${edge.branch}]` : ''}
                                      </span>
                                      <button
                                        aria-label={`Remove edge ${edge.source} to ${edge.target} from ${workflow.name}`}
                                        className="min-h-7 rounded-lg border border-amber-300 bg-white px-2 text-[11px] font-semibold text-amber-800 transition hover:border-red-300 hover:bg-red-50 hover:text-red-800"
                                        onClick={() => handleRemoveWorkflowEdge(workflow, edge)}
                                        type="button"
                                      >
                                        Remove
                                      </button>
                                    </li>
                                  ))}
                                </ul>
                              </div>
                            ) : workflowNodes.length > 0 ? (
                              <p className="break-all rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] px-3 py-2 font-mono text-xs text-[#625b4f]">
                                {workflowNodes.map((node) => node.id).join(' -> ')}
                              </p>
                            ) : null}
                          </div>

                          <aside className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3">
                            {selectedNode ? (
                              <>
                                <p className="break-all text-sm font-semibold text-[#181611]">
                                  Selected node: {selectedNode.id}
                                </p>
                                <div className="mt-3 grid grid-cols-2 gap-2 text-xs font-semibold text-[#625b4f]">
                                  <p className="rounded-lg bg-white px-3 py-2">Type: {selectedNode.type}</p>
                                  <p className="rounded-lg bg-white px-3 py-2">Status: {selectedNodeStatus}</p>
                                  <p className="rounded-lg bg-white px-3 py-2">
                                    Failure strategy: {selectedNode.failureStrategy}
                                  </p>
                                  {selectedNodeExecution ? (
                                    <p className="rounded-lg bg-white px-3 py-2">
                                      Duration: {formatDuration(selectedNodeExecution.durationMs)}
                                    </p>
                                  ) : null}
                                </div>
                                <div className="mt-3 space-y-2 text-xs font-semibold text-[#625b4f]">
                                  <p className="break-all rounded-lg bg-white px-3 py-2">
                                    Incoming:{' '}
                                    {selectedIncomingEdges.length > 0
                                      ? selectedIncomingEdges.map((edge) => edge.source).join(', ')
                                      : 'none'}
                                  </p>
                                  <p className="break-all rounded-lg bg-white px-3 py-2">
                                    Outgoing:{' '}
                                    {selectedOutgoingEdges.length > 0
                                      ? selectedOutgoingEdges.map((edge) => edge.target).join(', ')
                                      : 'none'}
                                  </p>
                                </div>
                                <button
                                  aria-label={`Remove node ${selectedNode.id} from ${workflow.name}`}
                                  className="mt-3 min-h-9 w-full rounded-lg border border-red-200 bg-white px-3 text-xs font-semibold text-red-700 transition hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50"
                                  disabled={busyAction === `definition:${workflow.id}`}
                                  onClick={() => handleRemoveWorkflowNode(workflow, selectedNode)}
                                  type="button"
                                >
                                  Remove node
                                </button>
                                <pre className="mt-3 max-h-48 overflow-auto rounded-lg bg-[#181611] p-3 font-mono text-xs leading-5 text-[#f7f4ea]">
                                  {formatJson(selectedNode.input)}
                                </pre>
                                <form
                                  aria-label={`Node configuration for ${selectedNode.id}`}
                                  className="mt-3 border-t border-[#e4dfd2] pt-3"
                                  onSubmit={(event) => {
                                    event.preventDefault();
                                    void handleSaveSelectedNodeConfig(workflow, selectedNode);
                                  }}
                                >
                                  <label
                                    className="block text-xs font-semibold text-[#625b4f]"
                                    htmlFor={`node-failure-strategy-${workflow.id}-${selectedNode.id}`}
                                  >
                                    Failure strategy for {selectedNode.id}
                                    <select
                                      className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm font-semibold text-[#181611]"
                                      id={`node-failure-strategy-${workflow.id}-${selectedNode.id}`}
                                      onChange={(event) =>
                                        updateNodeFailureStrategyDraft(
                                          workflow.id,
                                          selectedNode.id,
                                          event.target.value as WorkflowNodeFailureStrategy
                                        )
                                      }
                                      value={selectedNodeFailureStrategy}
                                    >
                                      {workflowNodeFailureStrategies.map((strategy) => (
                                        <option key={strategy.value} value={strategy.value}>
                                          {strategy.label}
                                        </option>
                                      ))}
                                    </select>
                                  </label>
                                  <label
                                    className="mt-3 block text-xs font-semibold text-[#625b4f]"
                                    htmlFor={`node-max-retries-${workflow.id}-${selectedNode.id}`}
                                  >
                                    Max retries for {selectedNode.id}
                                    <input
                                      className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                      id={`node-max-retries-${workflow.id}-${selectedNode.id}`}
                                      min="0"
                                      onChange={(event) =>
                                        updateNodeFailurePolicyDraft(workflow.id, selectedNode.id, {
                                          maxRetries: event.target.value,
                                        })
                                      }
                                      type="number"
                                      value={selectedNodeFailurePolicyDraft?.maxRetries ?? ''}
                                    />
                                  </label>
                                  <label
                                    className="mt-3 block text-xs font-semibold text-[#625b4f]"
                                    htmlFor={`node-retry-delays-${workflow.id}-${selectedNode.id}`}
                                  >
                                    Retry delays for {selectedNode.id}
                                    <input
                                      className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                      id={`node-retry-delays-${workflow.id}-${selectedNode.id}`}
                                      onChange={(event) =>
                                        updateNodeFailurePolicyDraft(workflow.id, selectedNode.id, {
                                          retryDelaysText: event.target.value,
                                        })
                                      }
                                      placeholder="1s, 5s, 30s"
                                      type="text"
                                      value={selectedNodeFailurePolicyDraft?.retryDelaysText ?? ''}
                                    />
                                  </label>
                                  <label
                                    className="mt-3 block text-xs font-semibold text-[#625b4f]"
                                    htmlFor={`node-failure-branch-${workflow.id}-${selectedNode.id}`}
                                  >
                                    Failure branch node for {selectedNode.id}
                                    <select
                                      className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                      id={`node-failure-branch-${workflow.id}-${selectedNode.id}`}
                                      onChange={(event) =>
                                        updateNodeFailurePolicyDraft(workflow.id, selectedNode.id, {
                                          failureBranchNodeId: event.target.value,
                                        })
                                      }
                                      value={selectedNodeFailurePolicyDraft?.failureBranchNodeId ?? ''}
                                    >
                                      <option value="">No failure branch</option>
                                      {nodeIds
                                        .filter((nodeId) => nodeId !== selectedNode.id)
                                        .map((nodeId) => (
                                          <option key={nodeId} value={nodeId}>
                                            {nodeId}
                                          </option>
                                        ))}
                                    </select>
                                  </label>
                                  <button
                                    aria-label={`Save node ${selectedNode.id} configuration`}
                                    className="mt-3 min-h-9 w-full rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                                    disabled={busyAction === `node-config:${workflow.id}:${selectedNode.id}`}
                                    type="submit"
                                  >
                                    {busyAction === `node-config:${workflow.id}:${selectedNode.id}`
                                      ? 'Saving...'
                                      : 'Save node configuration'}
                                  </button>
                                </form>
                              </>
                            ) : (
                              <p className="text-sm text-[#625b4f]">Select a node to inspect input JSON.</p>
                            )}
                          </aside>
                        </div>
                      )}

                      <div className="mt-4 border-t border-[#e4dfd2] pt-4">
                        <div className="flex flex-wrap items-center justify-between gap-3">
                          <h5 className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                            Definition builder
                          </h5>
                          <button
                            aria-label={`Save definition for ${workflow.name}`}
                            className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                            disabled={busyAction === `definition:${workflow.id}`}
                            onClick={() => void handleSaveWorkflowDefinition(workflow)}
                            type="button"
                          >
                            {busyAction === `definition:${workflow.id}` ? 'Saving...' : 'Save definition'}
                          </button>
                        </div>

                        <form
                          aria-label={`Resource policy ${workflow.name}`}
                          className="mt-3 rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3"
                          onSubmit={(event) => {
                            event.preventDefault();
                            void handleSaveWorkflowResourcePolicy(workflow);
                          }}
                        >
                          <div className="flex flex-wrap items-center justify-between gap-3">
                            <h6 className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                              Resource policy
                            </h6>
                            <button
                              aria-label={`Save resource policy for ${workflow.name}`}
                              className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                              disabled={busyAction === `resource-policy:${workflow.id}`}
                              type="submit"
                            >
                              {busyAction === `resource-policy:${workflow.id}` ? 'Saving...' : 'Save resource policy'}
                            </button>
                          </div>
                          <div className="mt-3 grid gap-3 md:grid-cols-3">
                            <label
                              className="block text-xs font-semibold text-[#625b4f]"
                              htmlFor={`workflow-max-concurrent-${workflow.id}`}
                            >
                              Max concurrent executions for {workflow.name}
                              <input
                                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                id={`workflow-max-concurrent-${workflow.id}`}
                                min="1"
                                onChange={(event) =>
                                  updateWorkflowResourcePolicyDraft(workflow.id, {
                                    maxConcurrentExecutions: event.target.value,
                                  })
                                }
                                type="number"
                                value={workflowResourcePolicyDraft.maxConcurrentExecutions}
                              />
                            </label>
                            <label
                              className="block text-xs font-semibold text-[#625b4f]"
                              htmlFor={`workflow-concurrency-overflow-${workflow.id}`}
                            >
                              Concurrency overflow for {workflow.name}
                              <select
                                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                id={`workflow-concurrency-overflow-${workflow.id}`}
                                onChange={(event) =>
                                  updateWorkflowResourcePolicyDraft(workflow.id, {
                                    concurrencyOverflow: event.target.value === 'reject' ? 'reject' : 'queue',
                                  })
                                }
                                value={workflowResourcePolicyDraft.concurrencyOverflow}
                              >
                                <option value="queue">queue</option>
                                <option value="reject">reject</option>
                              </select>
                            </label>
                            <label
                              className="block text-xs font-semibold text-[#625b4f]"
                              htmlFor={`workflow-max-duration-${workflow.id}`}
                            >
                              Max execution duration seconds for {workflow.name}
                              <input
                                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                id={`workflow-max-duration-${workflow.id}`}
                                min="1"
                                onChange={(event) =>
                                  updateWorkflowResourcePolicyDraft(workflow.id, {
                                    maxExecutionDurationSeconds: event.target.value,
                                  })
                                }
                                type="number"
                                value={workflowResourcePolicyDraft.maxExecutionDurationSeconds}
                              />
                            </label>
                            <label
                              className="block text-xs font-semibold text-[#625b4f]"
                              htmlFor={`workflow-token-budget-${workflow.id}`}
                            >
                              Max tokens budget for {workflow.name}
                              <input
                                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                id={`workflow-token-budget-${workflow.id}`}
                                min="1"
                                onChange={(event) =>
                                  updateWorkflowResourcePolicyDraft(workflow.id, {
                                    maxTokensBudget: event.target.value,
                                  })
                                }
                                type="number"
                                value={workflowResourcePolicyDraft.maxTokensBudget}
                              />
                            </label>
                            <label
                              className="block text-xs font-semibold text-[#625b4f]"
                              htmlFor={`workflow-max-node-executions-${workflow.id}`}
                            >
                              Max node executions for {workflow.name}
                              <input
                                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                id={`workflow-max-node-executions-${workflow.id}`}
                                min="1"
                                onChange={(event) =>
                                  updateWorkflowResourcePolicyDraft(workflow.id, {
                                    maxNodeExecutions: event.target.value,
                                  })
                                }
                                type="number"
                                value={workflowResourcePolicyDraft.maxNodeExecutions}
                              />
                            </label>
                          </div>
                        </form>

                        <div className="mt-3 grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]">
                          <form
                            aria-label={`Add node to ${workflow.name}`}
                            className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3"
                            onSubmit={(event) => {
                              event.preventDefault();
                              handleAddWorkflowNode(workflow);
                            }}
                          >
                            <div className="grid gap-3 md:grid-cols-2">
                              <label
                                className="block text-xs font-semibold text-[#625b4f]"
                                htmlFor={`workflow-node-id-${workflow.id}`}
                              >
                                New node ID for {workflow.name}
                                <input
                                  className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                  id={`workflow-node-id-${workflow.id}`}
                                  onChange={(event) =>
                                    updateWorkflowDefinitionDraft(workflow.id, { nodeId: event.target.value })
                                  }
                                  placeholder="notify-team"
                                  type="text"
                                  value={workflowDefinitionDraft.nodeId}
                                />
                              </label>
                              <label
                                className="block text-xs font-semibold text-[#625b4f]"
                                htmlFor={`workflow-node-type-${workflow.id}`}
                              >
                                New node type for {workflow.name}
                                <select
                                  className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                  id={`workflow-node-type-${workflow.id}`}
                                  onChange={(event) =>
                                    updateWorkflowDefinitionDraft(workflow.id, { nodeType: event.target.value })
                                  }
                                  value={workflowDefinitionDraft.nodeType}
                                >
                                  {workflowDefinitionNodeTypes.map((nodeType) => (
                                    <option key={nodeType} value={nodeType}>
                                      {nodeType}
                                    </option>
                                  ))}
                                </select>
                              </label>
                              <label
                                className="block text-xs font-semibold text-[#625b4f] md:col-span-2"
                                htmlFor={`workflow-node-input-${workflow.id}`}
                              >
                                New node input JSON for {workflow.name}
                                <textarea
                                  className="mt-2 min-h-24 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                  id={`workflow-node-input-${workflow.id}`}
                                  onChange={(event) =>
                                    updateWorkflowDefinitionDraft(workflow.id, { nodeInputText: event.target.value })
                                  }
                                  value={workflowDefinitionDraft.nodeInputText}
                                />
                              </label>
                            </div>
                            <button
                              className="mt-3 min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee]"
                              type="submit"
                            >
                              Add node to {workflow.name}
                            </button>
                          </form>

                          <form
                            aria-label={`Add edge to ${workflow.name}`}
                            className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-3"
                            onSubmit={(event) => {
                              event.preventDefault();
                              handleAddWorkflowEdge(workflow);
                            }}
                          >
                            <div className="grid gap-3">
                              <label
                                className="block text-xs font-semibold text-[#625b4f]"
                                htmlFor={`workflow-edge-source-${workflow.id}`}
                              >
                                New edge source for {workflow.name}
                                <select
                                  className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                  id={`workflow-edge-source-${workflow.id}`}
                                  onChange={(event) =>
                                    updateWorkflowDefinitionDraft(workflow.id, { edgeSource: event.target.value })
                                  }
                                  value={workflowDefinitionDraft.edgeSource}
                                >
                                  <option value="">Select source</option>
                                  {nodeIds.map((nodeId) => (
                                    <option key={nodeId} value={nodeId}>
                                      {nodeId}
                                    </option>
                                  ))}
                                </select>
                              </label>
                              <label
                                className="block text-xs font-semibold text-[#625b4f]"
                                htmlFor={`workflow-edge-target-${workflow.id}`}
                              >
                                New edge target for {workflow.name}
                                <select
                                  className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                  id={`workflow-edge-target-${workflow.id}`}
                                  onChange={(event) =>
                                    updateWorkflowDefinitionDraft(workflow.id, { edgeTarget: event.target.value })
                                  }
                                  value={workflowDefinitionDraft.edgeTarget}
                                >
                                  <option value="">Select target</option>
                                  {nodeIds.map((nodeId) => (
                                    <option key={nodeId} value={nodeId}>
                                      {nodeId}
                                    </option>
                                  ))}
                                </select>
                              </label>
                              <label
                                className="block text-xs font-semibold text-[#625b4f]"
                                htmlFor={`workflow-edge-branch-${workflow.id}`}
                              >
                                New edge branch for {workflow.name}
                                <select
                                  className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                  id={`workflow-edge-branch-${workflow.id}`}
                                  onChange={(event) =>
                                    updateWorkflowDefinitionDraft(workflow.id, { edgeBranch: event.target.value })
                                  }
                                  value={workflowDefinitionDraft.edgeBranch}
                                >
                                  <option value="">Any branch</option>
                                  <option value="true">true</option>
                                  <option value="false">false</option>
                                </select>
                              </label>
                            </div>
                            <button
                              className="mt-3 min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee]"
                              type="submit"
                            >
                              Add edge to {workflow.name}
                            </button>
                          </form>
                        </div>
                      </div>
                    </div>

                    <div className="mt-4 border-t border-[#e4dfd2] pt-4" aria-label={`Debug ${workflow.name}`}>
                      <div className="mb-4 border-b border-[#e4dfd2] pb-4" aria-label={`Version history ${workflow.name}`}>
                        <div className="flex flex-wrap items-center justify-between gap-3">
                          <h4 className="text-sm font-semibold text-[#181611]">Version history</h4>
                          <button
                            aria-label={`Load versions for ${workflow.name}`}
                            className="min-h-10 rounded-lg border border-[#cfc8b7] bg-white px-4 text-sm font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                            disabled={busyAction === `versions:${workflow.id}`}
                            onClick={() => void handleLoadVersions(workflow)}
                            type="button"
                          >
                            {busyAction === `versions:${workflow.id}` ? 'Loading...' : 'Load versions'}
                          </button>
                        </div>

                        {workflowVersions.length === 0 ? (
                          <p className="mt-3 text-sm text-[#625b4f]">No versions loaded.</p>
                        ) : (
                          <div className="mt-3 divide-y divide-[#e4dfd2]">
                            {workflowVersions.map((version) => {
                              const branchKey = workflowBranchDraftKey(workflow.id, version.version);
                              const branchDraft =
                                branchDrafts[branchKey] ?? { ...emptyBranchDraft, name: defaultBranchName(workflow, version) };
                              const isBranchVersion = isWorkflowBranchVersion(workflow, version);

                              return (
                                <div
                                  className="grid gap-3 py-3 md:grid-cols-[minmax(0,1fr)_120px_auto] md:items-center"
                                  key={`${workflow.id}-${version.version}`}
                                >
                                  <div>
                                    <p className="text-sm font-semibold text-[#181611]">Version {version.version}</p>
                                    <p className="mt-1 text-xs text-[#625b4f]">{describeNodeCount(version)}</p>
                                    <p className="mt-1 break-all font-mono text-xs text-[#625b4f]">
                                      {getWorkflowNodeIds(version).join(', ') || 'No nodes'}
                                    </p>
                                  </div>
                                  <p
                                    aria-label={`Workflow version ${version.version} status`}
                                    className="text-sm font-semibold text-[#625b4f]"
                                  >
                                    {version.status}
                                  </p>
                                  <div className="flex flex-wrap gap-2">
                                    <button
                                      aria-label={`Rollback ${workflow.name} to version ${version.version}`}
                                      className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-50"
                                      disabled={
                                        busyAction === `rollback:${workflow.id}:${version.version}` ||
                                        version.version === workflow.version
                                      }
                                      onClick={() => void handleRollbackWorkflow(workflow, version)}
                                      type="button"
                                    >
                                      {busyAction === `rollback:${workflow.id}:${version.version}`
                                        ? 'Rolling back...'
                                        : 'Rollback'}
                                    </button>
                                    <button
                                      aria-label={`Create branch from ${workflow.name} version ${version.version}`}
                                      className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-50"
                                      disabled={busyAction === `branch:${workflow.id}:${version.version}`}
                                      onClick={() => showBranchForm(workflow, version)}
                                      type="button"
                                    >
                                      Create branch
                                    </button>
                                    {isBranchVersion ? (
                                      <>
                                        <button
                                          aria-label={`Publish branch ${version.name}`}
                                          className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-50"
                                          disabled={
                                            busyAction === `publish-branch:${workflow.id}:${version.id}` ||
                                            version.status === 'published'
                                          }
                                          onClick={() => void handlePublishWorkflowBranch(workflow, version)}
                                          type="button"
                                        >
                                          {busyAction === `publish-branch:${workflow.id}:${version.id}`
                                            ? 'Publishing...'
                                            : 'Publish branch'}
                                        </button>
                                        <button
                                          aria-label={`Merge branch ${version.name} into ${workflow.name}`}
                                          className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-50"
                                          disabled={busyAction === `merge-branch:${workflow.id}:${version.id}`}
                                          onClick={() => void handleMergeWorkflowBranch(workflow, version)}
                                          type="button"
                                        >
                                          {busyAction === `merge-branch:${workflow.id}:${version.id}`
                                            ? 'Merging...'
                                            : 'Merge branch'}
                                        </button>
                                      </>
                                    ) : null}
                                  </div>
                                  {branchForms[branchKey] ? (
                                    <form
                                      aria-label={`Branch ${workflow.name} version ${version.version}`}
                                      className="grid gap-3 rounded-lg border border-[#e4dfd2] bg-white p-3 md:col-span-3 md:grid-cols-2"
                                      onSubmit={(event) => {
                                        event.preventDefault();
                                        void handleCreateWorkflowBranch(workflow, version);
                                      }}
                                    >
                                      <label
                                        className="block text-xs font-semibold text-[#625b4f]"
                                        htmlFor={`branch-name-${workflow.id}-${version.version}`}
                                      >
                                        Branch name for {workflow.name} version {version.version}
                                        <input
                                          className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 text-sm text-[#181611]"
                                          id={`branch-name-${workflow.id}-${version.version}`}
                                          onChange={(event) =>
                                            updateBranchDraft(workflow, version, { name: event.target.value })
                                          }
                                          type="text"
                                          value={branchDraft.name}
                                        />
                                      </label>
                                      <label
                                        className="block text-xs font-semibold text-[#625b4f]"
                                        htmlFor={`branch-description-${workflow.id}-${version.version}`}
                                      >
                                        Branch description for {workflow.name} version {version.version}
                                        <input
                                          className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 text-sm text-[#181611]"
                                          id={`branch-description-${workflow.id}-${version.version}`}
                                          onChange={(event) =>
                                            updateBranchDraft(workflow, version, { description: event.target.value })
                                          }
                                          type="text"
                                          value={branchDraft.description}
                                        />
                                      </label>
                                      <label
                                        className="block text-xs font-semibold text-[#625b4f]"
                                        htmlFor={`branch-experiment-${workflow.id}-${version.version}`}
                                      >
                                        Experiment key for {workflow.name} version {version.version}
                                        <input
                                          className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 text-sm text-[#181611]"
                                          id={`branch-experiment-${workflow.id}-${version.version}`}
                                          onChange={(event) =>
                                            updateBranchDraft(workflow, version, { experimentKey: event.target.value })
                                          }
                                          type="text"
                                          value={branchDraft.experimentKey}
                                        />
                                      </label>
                                      <label
                                        className="block text-xs font-semibold text-[#625b4f]"
                                        htmlFor={`branch-traffic-${workflow.id}-${version.version}`}
                                      >
                                        Traffic percent for {workflow.name} version {version.version}
                                        <input
                                          className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 text-sm text-[#181611]"
                                          id={`branch-traffic-${workflow.id}-${version.version}`}
                                          inputMode="numeric"
                                          onChange={(event) =>
                                            updateBranchDraft(workflow, version, { trafficPercent: event.target.value })
                                          }
                                          type="number"
                                          value={branchDraft.trafficPercent}
                                        />
                                      </label>
                                      <div className="flex items-end md:col-span-2">
                                        <button
                                          aria-label={`Submit branch for ${workflow.name} version ${version.version}`}
                                          className="min-h-9 rounded-lg bg-[#181611] px-3 text-xs font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                                          disabled={
                                            busyAction === `branch:${workflow.id}:${version.version}` ||
                                            branchDraft.name.trim() === ''
                                          }
                                          type="submit"
                                        >
                                          {busyAction === `branch:${workflow.id}:${version.version}`
                                            ? 'Creating branch...'
                                            : 'Create branch'}
                                        </button>
                                      </div>
                                    </form>
                                  ) : null}
                                </div>
                              );
                            })}
                          </div>
                        )}
                      </div>

                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <h4 className="text-sm font-semibold text-[#181611]">Debug</h4>
                        {nodeIds.length > 0 ? (
                          <p className="break-all text-xs text-[#625b4f]">Known nodes: {nodeIds.join(', ')}</p>
                        ) : null}
                      </div>

                      <form
                        className="mt-3 grid gap-3 md:grid-cols-[180px_minmax(0,1fr)_auto]"
                        onSubmit={(event) => {
                          event.preventDefault();
                          void handleTestNode(workflow);
                        }}
                      >
                        <label className="block text-sm font-medium" htmlFor={`node-id-${workflow.id}`}>
                          Node ID
                          <input
                            className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm"
                            id={`node-id-${workflow.id}`}
                            onChange={(event) => updateDebugDraft(workflow.id, { nodeId: event.target.value })}
                            placeholder={nodeIds[0] ?? 'node_id'}
                            type="text"
                            value={debugDraft.nodeId}
                          />
                        </label>
                        <label className="block text-sm font-medium" htmlFor={`node-input-${workflow.id}`}>
                          Node input JSON
                          <textarea
                            className="mt-2 min-h-24 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm"
                            id={`node-input-${workflow.id}`}
                            onChange={(event) => updateDebugDraft(workflow.id, { inputText: event.target.value })}
                            value={debugDraft.inputText}
                          />
                        </label>
                        <div className="flex items-end">
                          <button
                            className="min-h-10 rounded-lg bg-[#181611] px-4 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                            disabled={busyAction === `test:${workflow.id}`}
                            type="submit"
                          >
                            {busyAction === `test:${workflow.id}` ? 'Testing...' : 'Test node'}
                          </button>
                        </div>
                      </form>

                      {nodeResult ? (
                        <div className="mt-4 border-t border-[#e4dfd2] pt-3">
                          <p className="text-sm font-semibold text-[#181611]">
                            Node {nodeResult.nodeId} returned {nodeResult.status}
                          </p>
                          <pre className="mt-2 max-h-56 overflow-auto rounded-lg bg-[#181611] p-3 font-mono text-xs leading-5 text-[#f7f4ea]">
                            {formatJson(nodeTestResultPayload(nodeResult))}
                          </pre>
                        </div>
                      ) : null}

                      <div className="mt-4 border-t border-[#e4dfd2] pt-4">
                        <div className="flex flex-wrap items-center justify-between gap-3">
                          <h4 className="text-sm font-semibold text-[#181611]">Recent executions</h4>
                          <button
                            className="min-h-10 rounded-lg border border-[#cfc8b7] bg-white px-4 text-sm font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                            disabled={busyAction === `executions:${workflow.id}`}
                            onClick={() => void handleLoadExecutions(workflow)}
                            type="button"
                          >
                            {busyAction === `executions:${workflow.id}` ? 'Loading...' : 'Load executions'}
                          </button>
                        </div>

                        {workflowExecutions.length === 0 ? (
                          <p className="mt-3 text-sm text-[#625b4f]">No executions loaded.</p>
                        ) : (
                          <div className="mt-3 divide-y divide-[#e4dfd2]">
                            {workflowExecutions.map((workflowExecution) => {
                              const nodeExecutions = workflowExecution.nodeExecutions ?? [];
                              const debugSummary = buildExecutionDebugSummary(nodeExecutions);
                              const executionDebugSnapshot = executionDebugSnapshotsById[workflowExecution.id];
                              const failedNodeExecution = latestFailedNodeExecution(workflowExecution);
                              const failedNodeId = failedNodeExecution?.nodeId?.trim();
                              const pendingInputNodeExecution = latestPendingResumeInputNode(workflowExecution);
                              const pendingInputNodeId = pendingInputNodeExecution?.nodeId?.trim();
                              const pendingInputDraft =
                                pendingInputNodeId !== undefined
                                  ? (pausedInputDrafts[
                                      pausedInputDraftKey(workflowExecution.id, pendingInputNodeId)
                                    ] ?? defaultPausedInputDraft(pendingInputNodeExecution))
                                  : emptyPausedInputDraft;
                              const decisionDraft =
                                failedNodeId !== undefined
                                  ? (pausedFailureDecisionDrafts[
                                      pausedFailureDecisionDraftKey(workflowExecution.id, failedNodeId)
                                    ] ?? { inputText: formatJson(failedNodeExecution?.input) })
                                  : emptyPausedFailureDecisionDraft;
                              const longestNodeExecution = debugSummary.longestNodeExecution;

                              return (
                                <div
                                  className="grid gap-3 py-3 md:grid-cols-[minmax(0,1fr)_140px_auto] md:items-start"
                                  key={workflowExecution.id}
                                >
                                  <div>
                                    <p className="break-all font-mono text-sm font-semibold text-[#181611]">
                                      {workflowExecution.id}
                                    </p>
                                    {workflowExecution.output ? (
                                      <pre className="mt-2 max-h-40 overflow-auto rounded-lg bg-white p-3 font-mono text-xs leading-5 text-[#181611]">
                                        {formatJson(workflowExecution.output)}
                                      </pre>
                                    ) : null}
                                  </div>
                                  <p className="text-sm font-semibold text-[#625b4f]">
                                    Status:{' '}
                                    <span
                                      aria-label={`Workflow execution ${workflowExecution.id} status`}
                                      className={`inline-flex rounded-lg border px-2.5 py-1 text-xs font-semibold ${executionStatusClass(
                                        workflowExecution.status
                                      )}`}
                                    >
                                      {formatExecutionStatus(workflowExecution.status)}
                                    </span>
                                  </p>
                                  <div className="flex flex-wrap gap-2">
                                    <button
                                      aria-label={`Pause ${workflowExecution.id}`}
                                      className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-50"
                                      disabled={busyAction === `pause:${workflowExecution.id}`}
                                      onClick={() => void handleExecutionAction(workflow, workflowExecution, 'pause')}
                                      type="button"
                                    >
                                      Pause
                                    </button>
                                    <button
                                      aria-label={`Resume ${workflowExecution.id}`}
                                      className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-50"
                                      disabled={busyAction === `resume:${workflowExecution.id}`}
                                      onClick={() => void handleExecutionAction(workflow, workflowExecution, 'resume')}
                                      type="button"
                                    >
                                      Resume
                                    </button>
                                    <button
                                      aria-label={`Cancel ${workflowExecution.id}`}
                                      className="min-h-9 rounded-lg border border-red-200 bg-white px-3 text-xs font-semibold text-red-700 disabled:cursor-not-allowed disabled:opacity-50"
                                      disabled={busyAction === `cancel:${workflowExecution.id}`}
                                      onClick={() => void handleExecutionAction(workflow, workflowExecution, 'cancel')}
                                      type="button"
                                    >
                                      Cancel
                                    </button>
                                    <button
                                      aria-label={`View details for ${workflowExecution.id}`}
                                      className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                                      disabled={busyAction === `execution-detail:${workflowExecution.id}`}
                                      onClick={() => void handleLoadExecutionDetails(workflow, workflowExecution)}
                                      type="button"
                                    >
                                      {busyAction === `execution-detail:${workflowExecution.id}`
                                        ? 'Loading details...'
                                        : 'View details'}
                                    </button>
                                  </div>
                                  {executionDebugSnapshot ? (
                                    <section
                                      aria-label={`Execution debug details for ${workflowExecution.id}`}
                                      className="rounded-lg border border-[#cfe4dc] bg-[#f5faf7] p-3 md:col-span-3"
                                    >
                                      <div className="flex flex-wrap items-center justify-between gap-2">
                                        <h5 className="text-xs font-semibold uppercase tracking-wide text-[#1a614f]">
                                          Execution debug details
                                        </h5>
                                        <p className="break-all font-mono text-xs text-[#625b4f]">
                                          {executionDebugSnapshot.executionId} |{' '}
                                          {formatExecutionStatus(executionDebugSnapshot.status)}
                                        </p>
                                      </div>
                                      <div className="mt-3 grid gap-3 lg:grid-cols-2">
                                        <section className="min-w-0 rounded-lg border border-[#e4dfd2] bg-white p-3">
                                          <h6 className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                            Variables
                                          </h6>
                                          <pre className="mt-2 max-h-52 overflow-auto rounded-lg bg-[#181611] p-3 font-mono text-xs leading-5 text-[#f7f4ea]">
                                            {formatJson(executionDebugSnapshot.variableSnapshot)}
                                          </pre>
                                        </section>
                                        <section className="min-w-0 rounded-lg border border-[#e4dfd2] bg-white p-3">
                                          <h6 className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                            Call chain
                                          </h6>
                                          <p className="mt-2 break-all rounded-lg bg-[#fbfaf7] px-3 py-2 font-mono text-xs text-[#181611]">
                                            {debugSnapshotCallChain(executionDebugSnapshot)}
                                          </p>
                                        </section>
                                        <section className="min-w-0 rounded-lg border border-[#e4dfd2] bg-white p-3">
                                          <h6 className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                            Outputs
                                          </h6>
                                          <pre className="mt-2 max-h-52 overflow-auto rounded-lg bg-[#181611] p-3 font-mono text-xs leading-5 text-[#f7f4ea]">
                                            {formatJson(executionDebugSnapshot.outputs)}
                                          </pre>
                                        </section>
                                        <section className="min-w-0 rounded-lg border border-[#e4dfd2] bg-white p-3">
                                          <h6 className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                            Errors
                                          </h6>
                                          <pre className="mt-2 max-h-52 overflow-auto rounded-lg bg-[#181611] p-3 font-mono text-xs leading-5 text-[#f7f4ea]">
                                            {formatJson(buildDebugSnapshotErrors(executionDebugSnapshot))}
                                          </pre>
                                        </section>
                                        <section className="min-w-0 rounded-lg border border-[#e4dfd2] bg-white p-3 lg:col-span-2">
                                          <h6 className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                            Logs
                                          </h6>
                                          {executionDebugSnapshot.logs.length > 0 ? (
                                            <ol className="mt-2 space-y-2">
                                              {executionDebugSnapshot.logs.map((logEntry, logIndex) => (
                                                <li
                                                  className="break-all rounded-lg bg-[#fbfaf7] px-3 py-2 font-mono text-xs text-[#181611]"
                                                  key={`${executionDebugSnapshot.executionId}-log-${logIndex}`}
                                                >
                                                  {[logEntry.level, logEntry.nodeId, logEntry.message]
                                                    .filter(Boolean)
                                                    .join(' | ')}
                                                </li>
                                              ))}
                                            </ol>
                                          ) : (
                                            <p className="mt-2 rounded-lg bg-[#fbfaf7] px-3 py-2 text-xs font-semibold text-[#625b4f]">
                                              No logs recorded
                                            </p>
                                          )}
                                        </section>
                                        <section className="min-w-0 rounded-lg border border-[#e4dfd2] bg-white p-3 lg:col-span-2">
                                          <h6 className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                            Performance
                                          </h6>
                                          <div className="mt-2 grid gap-2 text-xs font-semibold text-[#625b4f] md:grid-cols-2">
                                            <p className="rounded-lg bg-[#fbfaf7] px-3 py-2">
                                              Total duration:{' '}
                                              {formatDuration(executionDebugSnapshot.performance.totalDurationMs)}
                                            </p>
                                            <p className="break-all rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-amber-800">
                                              {debugSnapshotBottleneckText(executionDebugSnapshot)}
                                            </p>
                                          </div>
                                          <pre className="mt-2 max-h-40 overflow-auto rounded-lg bg-[#181611] p-3 font-mono text-xs leading-5 text-[#f7f4ea]">
                                            {formatJson(executionDebugSnapshot.performance.nodeDurationsMs)}
                                          </pre>
                                        </section>
                                      </div>
                                    </section>
                                  ) : null}
                                  {nodeExecutions.length > 0 ? (
                                    <section
                                      aria-label={`Debug and performance summary for ${workflowExecution.id}`}
                                      className="rounded-lg border border-[#e4dfd2] bg-white p-3 md:col-span-3"
                                    >
                                      <div className="flex flex-wrap items-center justify-between gap-2">
                                        <h5 className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">
                                          Debug summary
                                        </h5>
                                        <p className="break-all font-mono text-xs text-[#625b4f]">
                                          Call chain:{' '}
                                          {nodeExecutions
                                            .map((nodeExecution, index) => getNodeExecutionId(nodeExecution, index))
                                            .join(' -> ')}
                                        </p>
                                      </div>
                                      <div className="mt-3 grid gap-2 text-xs font-semibold text-[#625b4f] md:grid-cols-4">
                                        <p className="rounded-lg bg-[#fbfaf7] px-3 py-2">
                                          Total duration: {formatDuration(debugSummary.totalDurationMs)}
                                        </p>
                                        <p className="rounded-lg bg-[#fbfaf7] px-3 py-2">
                                          Failed nodes: {debugSummary.failedNodeCount}
                                        </p>
                                        <p className="rounded-lg bg-[#fbfaf7] px-3 py-2">
                                          Retrying nodes: {debugSummary.retryingNodeCount}
                                        </p>
                                        <p className="break-all rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-amber-800">
                                          Slowest node:{' '}
                                          {longestNodeExecution
                                            ? `${getNodeExecutionId(
                                                longestNodeExecution,
                                                nodeExecutions.indexOf(longestNodeExecution)
                                              )} (${formatDuration(longestNodeExecution.durationMs)})`
                                            : 'none'}
                                        </p>
                                      </div>
                                      <ol className="mt-3 space-y-2">
                                        {nodeExecutions.map((nodeExecution, index) => {
                                          const nodeExecutionId = getNodeExecutionId(nodeExecution, index);
                                          const isLongestNode = nodeExecution === longestNodeExecution;

                                          return (
                                            <li
                                              className={`rounded-lg border p-3 ${
                                                isLongestNode
                                                  ? 'border-amber-200 bg-amber-50'
                                                  : 'border-[#e4dfd2] bg-[#fbfaf7]'
                                              }`}
                                              key={`${workflowExecution.id}-${nodeExecutionId}-${index}`}
                                            >
                                              <p
                                                className={`break-all font-mono text-xs font-semibold ${
                                                  isLongestNode ? 'text-amber-900' : 'text-[#181611]'
                                                }`}
                                              >
                                                {nodeExecutionId} | {nodeExecution.status ?? 'unknown'} |{' '}
                                                {formatDuration(nodeExecution.durationMs)} |{' '}
                                                {formatTokens(nodeExecution.tokens)} | {formatCost(nodeExecution.costUsd)}
                                              </p>
                                              {nodeExecution.output !== undefined ? (
                                                <pre className="mt-2 max-h-32 overflow-auto rounded-lg bg-white p-3 font-mono text-xs leading-5 text-[#181611]">
                                                  {formatJson(nodeExecution.output)}
                                                </pre>
                                              ) : null}
                                            </li>
                                          );
                                        })}
                                      </ol>
                                    </section>
                                  ) : null}
                                  {pendingInputNodeExecution && pendingInputNodeId ? (
                                    <section
                                      aria-label={`Paused input for ${workflowExecution.id}`}
                                      className="rounded-lg border border-amber-200 bg-amber-50 p-3 md:col-span-3"
                                    >
                                      <div className="flex flex-wrap items-center justify-between gap-2">
                                        <h5 className="text-xs font-semibold uppercase tracking-wide text-amber-900">
                                          Paused input
                                        </h5>
                                        <p className="break-all font-mono text-xs font-semibold text-amber-900">
                                          Paused on {formatWaitReason(pendingInputNodeExecution)} at {pendingInputNodeId}
                                        </p>
                                      </div>
                                      {pendingInputNodeExecution.input !== undefined ? (
                                        <pre className="mt-2 max-h-32 overflow-auto rounded-lg bg-white p-3 font-mono text-xs leading-5 text-[#181611]">
                                          {formatJson(pendingInputNodeExecution.input)}
                                        </pre>
                                      ) : null}
                                      <form
                                        className="mt-3 grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end"
                                        onSubmit={(event) => {
                                          event.preventDefault();
                                          void handleResumeWithInput(workflow, workflowExecution, pendingInputNodeExecution);
                                        }}
                                      >
                                        <label
                                          className="block text-xs font-semibold text-amber-900"
                                          htmlFor={`paused-input-${workflowExecution.id}-${pendingInputNodeId}`}
                                        >
                                          Resume input JSON for {pendingInputNodeId} in {workflowExecution.id}
                                          <textarea
                                            className="mt-2 min-h-24 w-full rounded-lg border border-amber-200 bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                            id={`paused-input-${workflowExecution.id}-${pendingInputNodeId}`}
                                            onChange={(event) =>
                                              updatePausedInputDraft(workflowExecution.id, pendingInputNodeId, {
                                                inputText: event.target.value,
                                              })
                                            }
                                            value={pendingInputDraft.inputText}
                                          />
                                        </label>
                                        <button
                                          aria-label={`Submit input for ${pendingInputNodeId} in ${workflowExecution.id}`}
                                          className="min-h-9 rounded-lg border border-[#1a614f] bg-white px-3 text-xs font-semibold text-[#1a614f] disabled:cursor-not-allowed disabled:opacity-50"
                                          disabled={
                                            busyAction === `resume-input:${workflowExecution.id}:${pendingInputNodeId}`
                                          }
                                          type="submit"
                                        >
                                          Submit input
                                        </button>
                                      </form>
                                    </section>
                                  ) : null}
                                  {failedNodeExecution && failedNodeId ? (
                                    <section
                                      aria-label={`Paused failure decisions for ${workflowExecution.id}`}
                                      className="rounded-lg border border-amber-200 bg-amber-50 p-3 md:col-span-3"
                                    >
                                      <div className="flex flex-wrap items-center justify-between gap-2">
                                        <h5 className="text-xs font-semibold uppercase tracking-wide text-amber-900">
                                          Paused failure decision
                                        </h5>
                                        <p className="break-all font-mono text-xs font-semibold text-amber-900">
                                          Paused on failed node {failedNodeId}
                                        </p>
                                      </div>
                                      {failedNodeExecution.error ? (
                                        <pre className="mt-2 max-h-32 overflow-auto rounded-lg bg-white p-3 font-mono text-xs leading-5 text-[#181611]">
                                          {formatJson(failedNodeExecution.error)}
                                        </pre>
                                      ) : null}
                                      <div className="mt-3 grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
                                        <label
                                          className="block text-xs font-semibold text-amber-900"
                                          htmlFor={`paused-failure-input-${workflowExecution.id}-${failedNodeId}`}
                                        >
                                          Edited retry input for {failedNodeId} in {workflowExecution.id}
                                          <textarea
                                            className="mt-2 min-h-24 w-full rounded-lg border border-amber-200 bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                            id={`paused-failure-input-${workflowExecution.id}-${failedNodeId}`}
                                            onChange={(event) =>
                                              updatePausedFailureDecisionDraft(workflowExecution.id, failedNodeId, {
                                                inputText: event.target.value,
                                              })
                                            }
                                            value={decisionDraft.inputText}
                                          />
                                        </label>
                                        <div className="flex flex-wrap gap-2">
                                          <label
                                            className="basis-full text-xs font-semibold text-amber-900"
                                            htmlFor={`paused-failure-branch-${workflowExecution.id}-${failedNodeId}`}
                                          >
                                            Failure branch target for {failedNodeId} in {workflowExecution.id}
                                            <select
                                              className="mt-2 w-full rounded-lg border border-amber-200 bg-white px-3 py-2 font-mono text-sm text-[#181611]"
                                              id={`paused-failure-branch-${workflowExecution.id}-${failedNodeId}`}
                                              onChange={(event) =>
                                                updatePausedFailureDecisionDraft(workflowExecution.id, failedNodeId, {
                                                  nextNodeId: event.target.value,
                                                })
                                              }
                                              value={decisionDraft.nextNodeId}
                                            >
                                              <option value="">Select branch target</option>
                                              {nodeIds
                                                .filter((nodeId) => nodeId !== failedNodeId)
                                                .map((nodeId) => (
                                                  <option key={nodeId} value={nodeId}>
                                                    {nodeId}
                                                  </option>
                                                ))}
                                            </select>
                                          </label>
                                          <button
                                            aria-label={`Retry failed node ${failedNodeId} for ${workflowExecution.id}`}
                                            className="min-h-9 rounded-lg border border-amber-300 bg-white px-3 text-xs font-semibold text-amber-900 disabled:cursor-not-allowed disabled:opacity-50"
                                            disabled={busyAction === `decision:${workflowExecution.id}:${failedNodeId}:retry`}
                                            onClick={() =>
                                              void handleResolvePausedFailure(
                                                workflow,
                                                workflowExecution,
                                                failedNodeExecution,
                                                'retry'
                                              )
                                            }
                                            type="button"
                                          >
                                            Retry
                                          </button>
                                          <button
                                            aria-label={`Skip failed node ${failedNodeId} for ${workflowExecution.id}`}
                                            className="min-h-9 rounded-lg border border-amber-300 bg-white px-3 text-xs font-semibold text-amber-900 disabled:cursor-not-allowed disabled:opacity-50"
                                            disabled={busyAction === `decision:${workflowExecution.id}:${failedNodeId}:continue`}
                                            onClick={() =>
                                              void handleResolvePausedFailure(
                                                workflow,
                                                workflowExecution,
                                                failedNodeExecution,
                                                'continue'
                                              )
                                            }
                                            type="button"
                                          >
                                            Skip
                                          </button>
                                          <button
                                            aria-label={`Retry ${failedNodeId} with edited input for ${workflowExecution.id}`}
                                            className="min-h-9 rounded-lg border border-[#1a614f] bg-white px-3 text-xs font-semibold text-[#1a614f] disabled:cursor-not-allowed disabled:opacity-50"
                                            disabled={
                                              busyAction === `decision:${workflowExecution.id}:${failedNodeId}:retry:input`
                                            }
                                            onClick={() =>
                                              void handleResolvePausedFailure(
                                                workflow,
                                                workflowExecution,
                                                failedNodeExecution,
                                                'retry',
                                                true
                                              )
                                            }
                                            type="button"
                                          >
                                            Retry with edited input
                                          </button>
                                          <button
                                            aria-label={`Branch from failed node ${failedNodeId} for ${workflowExecution.id}`}
                                            className="min-h-9 rounded-lg border border-[#1a614f] bg-white px-3 text-xs font-semibold text-[#1a614f] disabled:cursor-not-allowed disabled:opacity-50"
                                            disabled={busyAction === `decision:${workflowExecution.id}:${failedNodeId}:branch`}
                                            onClick={() =>
                                              void handleResolvePausedFailure(
                                                workflow,
                                                workflowExecution,
                                                failedNodeExecution,
                                                'branch'
                                              )
                                            }
                                            type="button"
                                          >
                                            Branch
                                          </button>
                                          <button
                                            aria-label={`Terminate workflow ${workflowExecution.id} after ${failedNodeId} failure`}
                                            className="min-h-9 rounded-lg border border-red-200 bg-white px-3 text-xs font-semibold text-red-700 disabled:cursor-not-allowed disabled:opacity-50"
                                            disabled={busyAction === `decision:${workflowExecution.id}:${failedNodeId}:fail`}
                                            onClick={() =>
                                              void handleResolvePausedFailure(
                                                workflow,
                                                workflowExecution,
                                                failedNodeExecution,
                                                'fail'
                                              )
                                            }
                                            type="button"
                                          >
                                            Terminate workflow
                                          </button>
                                        </div>
                                      </div>
                                    </section>
                                  ) : null}
                                  <form
                                    aria-label={`Resource limits for ${workflowExecution.id}`}
                                    className="rounded-lg border border-[#e4dfd2] bg-white p-3 md:col-span-3"
                                    onSubmit={(event) => {
                                      event.preventDefault();
                                      void handleCheckResourceLimits(workflow, workflowExecution);
                                    }}
                                  >
                                    <div className="grid gap-3 md:grid-cols-[160px_180px_auto] md:items-end">
                                      <label
                                        className="block text-xs font-semibold text-[#625b4f]"
                                        htmlFor={`resource-total-tokens-${workflowExecution.id}`}
                                      >
                                        Total tokens for {workflowExecution.id}
                                        <input
                                          className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 font-mono text-sm text-[#181611]"
                                          id={`resource-total-tokens-${workflowExecution.id}`}
                                          min="0"
                                          onChange={(event) =>
                                            updateResourceCheckDraft(workflowExecution.id, {
                                              totalTokens: event.target.value,
                                            })
                                          }
                                          placeholder="0"
                                          type="number"
                                          value={
                                            (resourceCheckDrafts[workflowExecution.id] ?? emptyResourceCheckDraft)
                                              .totalTokens
                                          }
                                        />
                                      </label>
                                      <label
                                        className="block text-xs font-semibold text-[#625b4f]"
                                        htmlFor={`resource-node-executions-${workflowExecution.id}`}
                                      >
                                        Node executions for {workflowExecution.id}
                                        <input
                                          className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 font-mono text-sm text-[#181611]"
                                          id={`resource-node-executions-${workflowExecution.id}`}
                                          min="0"
                                          onChange={(event) =>
                                            updateResourceCheckDraft(workflowExecution.id, {
                                              nodeExecutionCount: event.target.value,
                                            })
                                          }
                                          placeholder="0"
                                          type="number"
                                          value={
                                            (resourceCheckDrafts[workflowExecution.id] ?? emptyResourceCheckDraft)
                                              .nodeExecutionCount
                                          }
                                        />
                                      </label>
                                      <button
                                        aria-label={`Check resources for ${workflowExecution.id}`}
                                        className="min-h-9 rounded-lg border border-[#cfc8b7] bg-white px-3 text-xs font-semibold text-[#181611] transition hover:border-[#1a614f] hover:bg-[#e9f2ee] disabled:cursor-not-allowed disabled:opacity-50"
                                        disabled={busyAction === `resource-check:${workflowExecution.id}`}
                                        type="submit"
                                      >
                                        {busyAction === `resource-check:${workflowExecution.id}`
                                          ? 'Checking...'
                                          : 'Check resources'}
                                      </button>
                                    </div>
                                  </form>
                                </div>
                              );
                            })}
                          </div>
                        )}
                      </div>
                    </div>
                  </article>
                </li>
              );
            })}
          </ul>
        ) : null}
      </section>
    </main>
  );
}
