package loader

import (
	"testing"

	"github.com/dan-strohschein/cartograph/pkg/graph"
)

func loadTier4(t *testing.T) *graph.Graph {
	t.Helper()
	g, err := LoadFromDirectory("testdata/tier4")
	if err != nil {
		t.Fatalf("load tier4 fixture: %v", err)
	}
	return g
}

func nodeByName(g *graph.Graph, name string, kind graph.NodeKind) (graph.Node, bool) {
	for _, n := range g.NodesByName(name) {
		if n.Kind == kind {
			return n, true
		}
	}
	return graph.Node{}, false
}

func hasEdge(g *graph.Graph, srcName string, srcKind graph.NodeKind, kind graph.EdgeKind, label string) bool {
	src, ok := nodeByName(g, srcName, srcKind)
	if !ok {
		return false
	}
	for _, e := range g.OutEdges(src.ID) {
		if e.Kind == kind && (label == "" || e.Label == label) {
			return true
		}
	}
	return false
}

func TestTier4NodesCreated(t *testing.T) {
	g := loadTier4(t)
	cases := []struct {
		name string
		kind graph.NodeKind
	}{
		{"agent_loop", graph.KindGraph},
		{"web_search", graph.KindTool},
		{"fetch_url", graph.KindTool},
		{"researcher", graph.KindAgent},
		{"writer", graph.KindAgent},
		{"planner_llm", graph.KindModel},
		{"agent_loop.plan", graph.KindGraphNode},
		{"agent_loop.act", graph.KindGraphNode},
		{"agent_loop.observe", graph.KindGraphNode},
	}
	for _, c := range cases {
		if _, ok := nodeByName(g, c.name, c.kind); !ok {
			t.Errorf("missing %s node %q", c.kind, c.name)
		}
	}
}

func TestTier4Metadata(t *testing.T) {
	g := loadTier4(t)

	tool, ok := nodeByName(g, "web_search", graph.KindTool)
	if !ok {
		t.Fatal("web_search tool not found")
	}
	if tool.Metadata["invoked_by"] != "llm" {
		t.Errorf("web_search invoked_by = %q, want llm", tool.Metadata["invoked_by"])
	}
	if tool.Metadata["determinism"] != "nondeterministic" {
		t.Errorf("web_search determinism = %q", tool.Metadata["determinism"])
	}

	model, ok := nodeByName(g, "planner_llm", graph.KindModel)
	if !ok {
		t.Fatal("planner_llm model not found")
	}
	if model.Metadata["provider"] != "anthropic" {
		t.Errorf("planner_llm provider = %q", model.Metadata["provider"])
	}

	state, ok := nodeByName(g, "AgentState", graph.KindType)
	if !ok {
		t.Fatal("AgentState type not found")
	}
	if state.Metadata["channels"] != "true" {
		t.Errorf("AgentState should be marked @channels")
	}

	msgField, ok := nodeByName(g, "AgentState.messages", graph.KindField)
	if !ok {
		t.Fatal("AgentState.messages field not found")
	}
	if msgField.Metadata["reducer"] != "append" {
		t.Errorf("messages reducer = %q, want append", msgField.Metadata["reducer"])
	}
}

func TestTier4Edges(t *testing.T) {
	g := loadTier4(t)

	// Agent relationships.
	if !hasEdge(g, "researcher", graph.KindAgent, graph.EdgeUsesModel, "planner_llm") {
		t.Error("researcher -> UsesModel -> planner_llm missing")
	}
	if !hasEdge(g, "researcher", graph.KindAgent, graph.EdgeUsesTool, "web_search") {
		t.Error("researcher -> UsesTool -> web_search missing")
	}
	if !hasEdge(g, "researcher", graph.KindAgent, graph.EdgeHandsOffTo, "writer") {
		t.Error("researcher -> HandsOffTo -> writer missing")
	}
	if !hasEdge(g, "researcher", graph.KindAgent, graph.EdgeProducesOutput, "") {
		t.Error("researcher -> ProducesOutput missing")
	}

	// Model structured output.
	if !hasEdge(g, "planner_llm", graph.KindModel, graph.EdgeProducesOutput, "AgentDecision") {
		t.Error("planner_llm -> ProducesOutput -> AgentDecision missing")
	}

	// Graph topology.
	if !hasEdge(g, "agent_loop", graph.KindGraph, graph.EdgeContainsNode, "plan") {
		t.Error("agent_loop -> ContainsNode -> plan missing")
	}
	if !hasEdge(g, "agent_loop", graph.KindGraph, graph.EdgeUsesState, "AgentState") {
		t.Error("agent_loop -> UsesState -> AgentState missing")
	}
	if !hasEdge(g, "agent_loop", graph.KindGraph, graph.EdgeEntryNode, "plan") {
		t.Error("agent_loop -> EntryNode -> plan missing")
	}

	// Node implementation + flow.
	if !hasEdge(g, "agent_loop.plan", graph.KindGraphNode, graph.EdgeImplementedBy, "planner_node") {
		t.Error("plan -> ImplementedBy -> planner_node missing")
	}
	if !hasEdge(g, "agent_loop.act", graph.KindGraphNode, graph.EdgeFlowsTo, "observe") {
		t.Error("act -> FlowsTo -> observe missing")
	}
	// Conditional routing from plan.
	if !hasEdge(g, "agent_loop.plan", graph.KindGraphNode, graph.EdgeRoutedBy, "should_continue") {
		t.Error("plan -> RoutedBy -> should_continue missing")
	}
	if !hasEdge(g, "agent_loop.plan", graph.KindGraphNode, graph.EdgeConditionalFlow, "act") {
		t.Error("plan -> ConditionalFlow -> act missing")
	}
}
