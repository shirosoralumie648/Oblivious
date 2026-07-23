import { describe, it, expect, beforeEach } from 'vitest';
import { useWorkflowStore } from './workflow';

describe('useWorkflowStore', () => {
  beforeEach(() => {
    useWorkflowStore.setState({
      workflows: [],
      currentWorkflow: null,
      nodes: [],
      edges: [],
      executionState: null,
    });
  });

  it('should initialize with empty state', () => {
    const state = useWorkflowStore.getState();
    expect(state.workflows).toEqual([]);
    expect(state.currentWorkflow).toBeNull();
    expect(state.nodes).toEqual([]);
    expect(state.edges).toEqual([]);
    expect(state.executionState).toBeNull();
  });

  it('should set workflows', () => {
    const workflows = [{ id: 'wf1' }, { id: 'wf2' }];
    useWorkflowStore.getState().setWorkflows(workflows);
    expect(useWorkflowStore.getState().workflows).toEqual(workflows);
  });

  it('should set current workflow', () => {
    useWorkflowStore.getState().setCurrentWorkflow('wf1');
    expect(useWorkflowStore.getState().currentWorkflow).toBe('wf1');
  });

  it('should set nodes', () => {
    const nodes = [{ id: 'n1' }, { id: 'n2' }];
    useWorkflowStore.getState().setNodes(nodes);
    expect(useWorkflowStore.getState().nodes).toEqual(nodes);
  });

  it('should set edges', () => {
    const edges = [{ id: 'e1' }, { id: 'e2' }];
    useWorkflowStore.getState().setEdges(edges);
    expect(useWorkflowStore.getState().edges).toEqual(edges);
  });

  it('should update execution state', () => {
    const execState = { status: 'running', progress: 50 };
    useWorkflowStore.getState().updateExecutionState(execState);
    expect(useWorkflowStore.getState().executionState).toEqual(execState);
  });
});
