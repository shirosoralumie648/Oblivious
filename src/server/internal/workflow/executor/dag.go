package executor

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrDAGCycleDetected = errors.New("workflow DAG contains a cycle")
	ErrDAGEmpty         = errors.New("workflow DAG has no nodes")
	ErrDAGInvalidEdge   = errors.New("workflow DAG edge references unknown node")
)

// DAGAnalysis holds the result of DAG parsing and analysis.
type DAGAnalysis struct {
	TopologicalOrder []string   `json:"topologicalOrder"`
	ParallelGroups   [][]string `json:"parallelGroups"`
	RootNodes        []string   `json:"rootNodes"`
	LeafNodes        []string   `json:"leafNodes"`
	HasCycle         bool       `json:"hasCycle"`
	CycleNodes       []string   `json:"cycleNodes,omitempty"`
}

// DAGNode represents a node in a directed acyclic graph.
type DAGNode struct {
	ID       string
	Type     string
	Children []string
	Parents  []string
}

// DAGEdge represents a directed edge in the graph.
type DAGEdge struct {
	From string
	To   string
}

// DAG represents a workflow directed acyclic graph with analysis capabilities.
type DAG struct {
	Nodes map[string]*DAGNode
	Edges []DAGEdge
}

// NewDAG creates a new DAG from node IDs and edges.
func NewDAG(nodeIDs []string, edges []DAGEdge) (*DAG, error) {
	if len(nodeIDs) == 0 {
		return nil, ErrDAGEmpty
	}

	nodes := make(map[string]*DAGNode, len(nodeIDs))
	for _, id := range nodeIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		nodes[trimmed] = &DAGNode{
			ID:       trimmed,
			Children: []string{},
			Parents:  []string{},
		}
	}

	dag := &DAG{
		Nodes: nodes,
		Edges: make([]DAGEdge, 0, len(edges)),
	}

	for _, edge := range edges {
		from := strings.TrimSpace(edge.From)
		to := strings.TrimSpace(edge.To)
		if from == "" || to == "" {
			continue
		}
		if _, ok := nodes[from]; !ok {
			return nil, fmt.Errorf("%w: source node %s", ErrDAGInvalidEdge, from)
		}
		if _, ok := nodes[to]; !ok {
			return nil, fmt.Errorf("%w: target node %s", ErrDAGInvalidEdge, to)
		}
		dag.Edges = append(dag.Edges, DAGEdge{From: from, To: to})
		nodes[from].Children = append(nodes[from].Children, to)
		nodes[to].Parents = append(nodes[to].Parents, from)
	}

	return dag, nil
}

// TopologicalSort returns a valid topological ordering of all nodes.
// Uses Kahn's algorithm (BFS-based).
func (d *DAG) TopologicalSort() ([]string, error) {
	if d == nil || len(d.Nodes) == 0 {
		return nil, ErrDAGEmpty
	}

	inDegree := make(map[string]int, len(d.Nodes))
	for id := range d.Nodes {
		inDegree[id] = 0
	}
	for _, edge := range d.Edges {
		inDegree[edge.To]++
	}

	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sortStrings(queue)

	order := make([]string, 0, len(d.Nodes))
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		order = append(order, nodeID)

		children := make([]string, len(d.Nodes[nodeID].Children))
		copy(children, d.Nodes[nodeID].Children)
		sortStrings(children)

		for _, childID := range children {
			inDegree[childID]--
			if inDegree[childID] == 0 {
				queue = append(queue, childID)
			}
		}
	}

	if len(order) != len(d.Nodes) {
		cycleNodes := d.findCycleNodes()
		return order, fmt.Errorf("%w: nodes %v", ErrDAGCycleDetected, cycleNodes)
	}

	return order, nil
}

// DetectCycles returns true if the DAG contains a cycle, along with the nodes involved.
func (d *DAG) DetectCycles() (bool, []string) {
	if d == nil || len(d.Nodes) == 0 {
		return false, nil
	}

	const (
		white = 0 // unvisited
		gray  = 1 // in progress
		black = 2 // done
	)

	color := make(map[string]int, len(d.Nodes))
	for id := range d.Nodes {
		color[id] = white
	}

	var cyclePath []string
	var visit func(string) bool
	visit = func(nodeID string) bool {
		color[nodeID] = gray
		for _, childID := range d.Nodes[nodeID].Children {
			switch color[childID] {
			case gray:
				cyclePath = []string{childID, nodeID}
				return true
			case white:
				if visit(childID) {
					if len(cyclePath) > 0 && cyclePath[len(cyclePath)-1] != nodeID {
						cyclePath = append(cyclePath, nodeID)
					}
					return true
				}
			}
		}
		color[nodeID] = black
		return false
	}

	ids := sortedNodeIDs(d.Nodes)
	for _, id := range ids {
		if color[id] == white && visit(id) {
			return true, cyclePath
		}
	}
	return false, nil
}

// DetectParallelGroups identifies sets of nodes that can execute in parallel.
// Each group represents a "level" in the DAG where all nodes have their
// dependencies satisfied by nodes in earlier groups.
func (d *DAG) DetectParallelGroups() [][]string {
	if d == nil || len(d.Nodes) == 0 {
		return nil
	}

	inDegree := make(map[string]int, len(d.Nodes))
	for id := range d.Nodes {
		inDegree[id] = 0
	}
	for _, edge := range d.Edges {
		inDegree[edge.To]++
	}

	remaining := make(map[string]bool, len(d.Nodes))
	for id := range d.Nodes {
		remaining[id] = true
	}

	groups := [][]string{}
	for len(remaining) > 0 {
		group := []string{}
		for id := range remaining {
			if inDegree[id] == 0 {
				group = append(group, id)
			}
		}
		if len(group) == 0 {
			break
		}
		sortStrings(group)
		groups = append(groups, group)

		for _, id := range group {
			delete(remaining, id)
			for _, childID := range d.Nodes[id].Children {
				inDegree[childID]--
			}
		}
	}

	return groups
}

// RootNodes returns node IDs with no incoming edges.
func (d *DAG) RootNodes() []string {
	if d == nil {
		return nil
	}
	roots := []string{}
	for id, node := range d.Nodes {
		if len(node.Parents) == 0 {
			roots = append(roots, id)
		}
	}
	sortStrings(roots)
	return roots
}

// LeafNodes returns node IDs with no outgoing edges.
func (d *DAG) LeafNodes() []string {
	if d == nil {
		return nil
	}
	leaves := []string{}
	for id, node := range d.Nodes {
		if len(node.Children) == 0 {
			leaves = append(leaves, id)
		}
	}
	sortStrings(leaves)
	return leaves
}

// Validate checks the DAG for structural integrity and returns an error if invalid.
func (d *DAG) Validate() error {
	if d == nil || len(d.Nodes) == 0 {
		return ErrDAGEmpty
	}

	if hasCycle, cycleNodes := d.DetectCycles(); hasCycle {
		return fmt.Errorf("%w: cycle through nodes %v", ErrDAGCycleDetected, cycleNodes)
	}

	roots := d.RootNodes()
	if len(roots) == 0 {
		return fmt.Errorf("%w: DAG must have at least one root node (no incoming edges)", ErrDAGCycleDetected)
	}

	return nil
}

// Analyze performs full DAG analysis and returns a comprehensive result.
func (d *DAG) Analyze() DAGAnalysis {
	analysis := DAGAnalysis{}

	if d == nil || len(d.Nodes) == 0 {
		analysis.HasCycle = false
		return analysis
	}

	analysis.RootNodes = d.RootNodes()
	analysis.LeafNodes = d.LeafNodes()
	analysis.ParallelGroups = d.DetectParallelGroups()

	hasCycle, cycleNodes := d.DetectCycles()
	analysis.HasCycle = hasCycle
	analysis.CycleNodes = cycleNodes

	if !hasCycle {
		order, err := d.TopologicalSort()
		if err == nil {
			analysis.TopologicalOrder = order
		}
	}

	return analysis
}

func (d *DAG) findCycleNodes() []string {
	hasCycle, nodes := d.DetectCycles()
	if !hasCycle {
		return nil
	}
	return nodes
}

func sortedNodeIDs(nodes map[string]*DAGNode) []string {
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sortStrings(ids)
	return ids
}

func sortStrings(items []string) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i] > items[j] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
