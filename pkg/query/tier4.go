package query

import (
	"sort"
	"strings"

	"github.com/dan-strohschein/cartograph/pkg/graph"
)

// GraphView is the resolved topology of a Tier 4 @graph entry.
type GraphView struct {
	Name        string            `json:"name"`
	Purpose     string            `json:"purpose,omitempty"`
	Engine      string            `json:"engine,omitempty"`
	State       string            `json:"state,omitempty"`
	Reducers    map[string]string `json:"reducers,omitempty"`
	EntryNodes  []string          `json:"entry_nodes,omitempty"`
	Nodes       []GraphNodeView   `json:"nodes"`
	Edges       []FlowEdge        `json:"edges,omitempty"`
	Conditional []CondEdge        `json:"conditional_edges,omitempty"`
}

// GraphNodeView is a single node inside a @graph.
type GraphNodeView struct {
	Name       string `json:"name"`
	Implements string `json:"implements,omitempty"`
	Effects    string `json:"effects,omitempty"`
}

// FlowEdge is an unconditional edge between two graph nodes.
type FlowEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Frame string `json:"frame,omitempty"`
}

// CondEdge is a routed edge: a source node, a router function, and targets.
type CondEdge struct {
	From    string   `json:"from"`
	Router  string   `json:"router,omitempty"`
	Targets []string `json:"targets"`
}

// AgentView is the resolved configuration of a Tier 4 @agent entry.
type AgentView struct {
	Name     string   `json:"name"`
	Purpose  string   `json:"purpose,omitempty"`
	Model    string   `json:"model,omitempty"`
	Tools    []string `json:"tools,omitempty"`
	Handoffs []string `json:"handoffs,omitempty"`
	Output   string   `json:"output,omitempty"`
	Autonomy string   `json:"autonomy,omitempty"`
	Memory   string   `json:"memory,omitempty"`
	Effects  string   `json:"effects,omitempty"`
}

// ToolView is a summary of a Tier 4 @tool entry.
type ToolView struct {
	Name        string `json:"name"`
	Purpose     string `json:"purpose,omitempty"`
	Signature   string `json:"signature,omitempty"`
	InvokedBy   string `json:"invoked_by,omitempty"`
	Determinism string `json:"determinism,omitempty"`
	Effects     string `json:"effects,omitempty"`
}

// GraphTopology resolves a @graph by name into a structured topology view.
func (qe *QueryEngine) GraphTopology(name string) (*GraphView, error) {
	gn, err := qe.resolveNode(name, graph.KindGraph)
	if err != nil {
		return nil, err
	}

	v := &GraphView{
		Name:     gn.Name,
		Purpose:  gn.Purpose,
		Engine:   gn.Metadata["engine"],
		Reducers: make(map[string]string),
	}

	for _, e := range qe.g.OutEdges(gn.ID) {
		switch e.Kind {
		case graph.EdgeUsesState:
			v.State = e.Label
		case graph.EdgeEntryNode:
			v.EntryNodes = append(v.EntryNodes, e.Label)
		case graph.EdgeContainsNode:
			node, err := qe.g.NodeByID(e.Target)
			if err != nil {
				continue
			}
			v.Nodes = append(v.Nodes, qe.graphNodeView(node))
			v.Edges = append(v.Edges, qe.flowEdges(node)...)
			if ce, ok := qe.condEdge(node); ok {
				v.Conditional = append(v.Conditional, ce)
			}
		}
	}

	// State channel reducers, if the @state type uses @channels.
	if v.State != "" {
		prefix := v.State + "."
		for _, n := range qe.g.NodesByKind(graph.KindField) {
			if strings.HasPrefix(n.Name, prefix) {
				if r := n.Metadata["reducer"]; r != "" {
					v.Reducers[strings.TrimPrefix(n.Name, prefix)] = r
				}
			}
		}
	}
	if len(v.Reducers) == 0 {
		v.Reducers = nil
	}

	sort.Slice(v.Nodes, func(i, j int) bool { return v.Nodes[i].Name < v.Nodes[j].Name })
	return v, nil
}

func (qe *QueryEngine) graphNodeView(node graph.Node) GraphNodeView {
	return GraphNodeView{
		Name:       shortNodeName(node.Name),
		Implements: node.Metadata["implements"],
		Effects:    node.Metadata["effects"],
	}
}

func (qe *QueryEngine) flowEdges(node graph.Node) []FlowEdge {
	var edges []FlowEdge
	from := shortNodeName(node.Name)
	frames := map[string]string{}
	for _, e := range qe.g.OutEdges(node.ID) {
		if e.Kind == graph.EdgeCarriesFrame {
			frames[from] = e.Label
		}
	}
	for _, e := range qe.g.OutEdges(node.ID) {
		if e.Kind == graph.EdgeFlowsTo {
			edges = append(edges, FlowEdge{From: from, To: e.Label, Frame: frames[from]})
		}
	}
	return edges
}

func (qe *QueryEngine) condEdge(node graph.Node) (CondEdge, bool) {
	ce := CondEdge{From: shortNodeName(node.Name)}
	found := false
	for _, e := range qe.g.OutEdges(node.ID) {
		switch e.Kind {
		case graph.EdgeRoutedBy:
			ce.Router = e.Label
			found = true
		case graph.EdgeConditionalFlow:
			ce.Targets = append(ce.Targets, e.Label)
			found = true
		}
	}
	return ce, found
}

// AgentInfo resolves an @agent by name into a structured view.
func (qe *QueryEngine) AgentInfo(name string) (*AgentView, error) {
	an, err := qe.resolveNode(name, graph.KindAgent)
	if err != nil {
		return nil, err
	}
	v := &AgentView{
		Name:     an.Name,
		Purpose:  an.Purpose,
		Autonomy: an.Metadata["autonomy"],
		Memory:   an.Metadata["memory"],
		Effects:  an.Metadata["effects"],
	}
	for _, e := range qe.g.OutEdges(an.ID) {
		switch e.Kind {
		case graph.EdgeUsesModel:
			v.Model = e.Label
		case graph.EdgeUsesTool:
			v.Tools = append(v.Tools, e.Label)
		case graph.EdgeHandsOffTo:
			v.Handoffs = append(v.Handoffs, e.Label)
		case graph.EdgeProducesOutput:
			v.Output = e.Label
		}
	}
	return v, nil
}

// ListTools returns all @tool entries in the graph, sorted by name.
func (qe *QueryEngine) ListTools() []ToolView {
	var tools []ToolView
	for _, n := range qe.g.NodesByKind(graph.KindTool) {
		tools = append(tools, ToolView{
			Name:        n.Name,
			Purpose:     n.Purpose,
			Signature:   n.Signature,
			InvokedBy:   n.Metadata["invoked_by"],
			Determinism: n.Metadata["determinism"],
			Effects:     n.Metadata["effects"],
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

// shortNodeName strips the "graphName." prefix from a GraphNode name.
func shortNodeName(name string) string {
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}
