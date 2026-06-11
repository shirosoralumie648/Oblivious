import { create } from 'zustand';

interface Node {
  id: string;
  [key: string]: any;
}

interface Edge {
  id: string;
  [key: string]: any;
}

interface Workflow {
  id: string;
  [key: string]: any;
}

interface ExecutionState {
  [key: string]: any;
}

interface WorkflowState {
  workflows: Workflow[];
  currentWorkflow: string | null;
  nodes: Node[];
  edges: Edge[];
  executionState: ExecutionState | null;
  setWorkflows: (wfs: Workflow[]) => void;
  setCurrentWorkflow: (id: string | null) => void;
  setNodes: (nodes: Node[]) => void;
  setEdges: (edges: Edge[]) => void;
  updateExecutionState: (state: ExecutionState) => void;
}

export const useWorkflowStore = create<WorkflowState>((set) => ({
  workflows: [],
  currentWorkflow: null,
  nodes: [],
  edges: [],
  executionState: null,
  setWorkflows: (wfs) => set({ workflows: wfs }),
  setCurrentWorkflow: (id) => set({ currentWorkflow: id }),
  setNodes: (nodes) => set({ nodes }),
  setEdges: (edges) => set({ edges }),
  updateExecutionState: (state) => set({ executionState: state }),
}));
