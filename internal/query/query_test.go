package query

import (
	"testing"

	"github.com/dan-strohschein/cartograph/internal/graph"
)

func buildTestGraph() *graph.Graph {
	g := graph.NewGraph()

	// Module node.
	g.AddNode(graph.Node{ID: graph.MakeNodeID("m", graph.KindModule, "m"), Kind: graph.KindModule, Name: "m", QualifiedName: "m", Module: "m", Metadata: map[string]string{}})

	// Types.
	g.AddNode(graph.Node{ID: graph.MakeNodeID("m", graph.KindType, "User"), Kind: graph.KindType, Name: "User", QualifiedName: "m.User", Module: "m", Metadata: map[string]string{}})
	g.AddNode(graph.Node{ID: graph.MakeNodeID("m", graph.KindType, "HttpError"), Kind: graph.KindType, Name: "HttpError", QualifiedName: "m.HttpError", Module: "m", Metadata: map[string]string{}})

	// Fields.
	g.AddNode(graph.Node{ID: graph.MakeNodeID("m", graph.KindField, "User.email"), Kind: graph.KindField, Name: "User.email", QualifiedName: "m.User.email", Module: "m", Type: "str", Metadata: map[string]string{}})

	// Functions.
	g.AddNode(graph.Node{ID: graph.MakeNodeID("m", graph.KindFunction, "GetUser"), Kind: graph.KindFunction, Name: "GetUser", QualifiedName: "m.GetUser", Module: "m", Metadata: map[string]string{"effects": "Db"}})
	g.AddNode(graph.Node{ID: graph.MakeNodeID("m", graph.KindFunction, "HandleRequest"), Kind: graph.KindFunction, Name: "HandleRequest", QualifiedName: "m.HandleRequest", Module: "m", Metadata: map[string]string{"effects": "Net"}})
	g.AddNode(graph.Node{ID: graph.MakeNodeID("m", graph.KindFunction, "ValidateEmail"), Kind: graph.KindFunction, Name: "ValidateEmail", QualifiedName: "m.ValidateEmail", Module: "m", Metadata: map[string]string{}})

	userID := graph.MakeNodeID("m", graph.KindType, "User")
	errorID := graph.MakeNodeID("m", graph.KindType, "HttpError")
	fieldID := graph.MakeNodeID("m", graph.KindField, "User.email")
	getUserID := graph.MakeNodeID("m", graph.KindFunction, "GetUser")
	handleReqID := graph.MakeNodeID("m", graph.KindFunction, "HandleRequest")
	validateID := graph.MakeNodeID("m", graph.KindFunction, "ValidateEmail")

	// Edges.
	g.AddEdge(graph.Edge{Source: userID, Target: fieldID, Kind: graph.EdgeHasField, Label: "email"})
	g.AddEdge(graph.Edge{Source: getUserID, Target: userID, Kind: graph.EdgeReturns, Label: "User"})
	g.AddEdge(graph.Edge{Source: getUserID, Target: errorID, Kind: graph.EdgeProducesError, Label: "HttpError"})
	g.AddEdge(graph.Edge{Source: handleReqID, Target: getUserID, Kind: graph.EdgeCalls, Label: "GetUser"})
	g.AddEdge(graph.Edge{Source: getUserID, Target: validateID, Kind: graph.EdgeCalls, Label: "ValidateEmail"})
	g.AddEdge(graph.Edge{Source: validateID, Target: fieldID, Kind: graph.EdgeReadsField, Label: "email"})
	g.AddEdge(graph.Edge{Source: handleReqID, Target: userID, Kind: graph.EdgeAccepts, Label: "User"})

	return g
}

func TestErrorProducers(t *testing.T) {
	g := buildTestGraph()
	engine := NewQueryEngine(g, 10)

	result, err := engine.ErrorProducers("HttpError")
	if err != nil {
		t.Fatalf("ErrorProducers failed: %v", err)
	}
	if len(result.Paths) == 0 {
		t.Error("expected at least one path")
	}
	t.Logf("ErrorProducers: %s", result.Summary)
	for _, p := range result.Paths {
		names := make([]string, len(p.Nodes))
		for i, n := range p.Nodes {
			names[i] = n.Name
		}
		t.Logf("  path: %v (depth %d)", names, p.Depth)
	}
}

func TestErrorProducersNotFound(t *testing.T) {
	g := buildTestGraph()
	engine := NewQueryEngine(g, 10)

	_, err := engine.ErrorProducers("NonExistentError")
	if err == nil {
		t.Error("expected error for nonexistent error type")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestFieldTouchers(t *testing.T) {
	g := buildTestGraph()
	engine := NewQueryEngine(g, 10)

	result, err := engine.FieldTouchers("User", "email")
	if err != nil {
		t.Fatalf("FieldTouchers failed: %v", err)
	}
	if len(result.Paths) == 0 {
		t.Error("expected at least one path")
	}
	t.Logf("FieldTouchers: %s", result.Summary)
}

func TestCallStack(t *testing.T) {
	g := buildTestGraph()
	engine := NewQueryEngine(g, 10)

	// Reverse (callers of GetUser).
	result, err := engine.CallStack("GetUser", Reverse)
	if err != nil {
		t.Fatalf("CallStack Reverse failed: %v", err)
	}
	if len(result.Paths) == 0 {
		t.Error("expected callers")
	}
	t.Logf("CallStack(GetUser, up): %s", result.Summary)

	// Forward (callees of GetUser).
	result, err = engine.CallStack("GetUser", Forward)
	if err != nil {
		t.Fatalf("CallStack Forward failed: %v", err)
	}
	if len(result.Paths) == 0 {
		t.Error("expected callees")
	}
	t.Logf("CallStack(GetUser, down): %s", result.Summary)
}

func TestTypeDependents(t *testing.T) {
	g := buildTestGraph()
	engine := NewQueryEngine(g, 10)

	result, err := engine.TypeDependents("User")
	if err != nil {
		t.Fatalf("TypeDependents failed: %v", err)
	}
	if len(result.Paths) == 0 {
		t.Error("expected dependents")
	}
	t.Logf("TypeDependents(User): %s", result.Summary)
}

func TestSideEffects(t *testing.T) {
	g := buildTestGraph()
	engine := NewQueryEngine(g, 10)

	report, err := engine.SideEffects("HandleRequest")
	if err != nil {
		t.Fatalf("SideEffects failed: %v", err)
	}
	if len(report.Effects) == 0 {
		t.Error("expected effects")
	}
	t.Logf("SideEffects(HandleRequest): %d categories, %d callees", len(report.Effects), report.TotalCallees)
	for cat, nodes := range report.Effects {
		names := make([]string, len(nodes))
		for i, n := range nodes {
			names[i] = n.Name
		}
		t.Logf("  [%s]: %v", cat, names)
	}
}

func TestTraverseCycleDetection(t *testing.T) {
	g := graph.NewGraph()
	a := graph.Node{ID: graph.MakeNodeID("m", graph.KindFunction, "A"), Kind: graph.KindFunction, Name: "A", Module: "m", Metadata: map[string]string{}}
	b := graph.Node{ID: graph.MakeNodeID("m", graph.KindFunction, "B"), Kind: graph.KindFunction, Name: "B", Module: "m", Metadata: map[string]string{}}
	g.AddNode(a)
	g.AddNode(b)
	g.AddEdge(graph.Edge{Source: a.ID, Target: b.ID, Kind: graph.EdgeCalls})
	g.AddEdge(graph.Edge{Source: b.ID, Target: a.ID, Kind: graph.EdgeCalls}) // cycle

	engine := NewQueryEngine(g, 10)
	paths := engine.Traverse(a.ID, nil, Forward, 10)
	// Should terminate without infinite loop.
	if len(paths) == 0 {
		t.Error("expected at least one path")
	}
	for _, p := range paths {
		if p.Depth > 2 {
			t.Errorf("path too deep (cycle not detected?): depth=%d", p.Depth)
		}
	}
}

func TestCallStackNotFound(t *testing.T) {
	g := buildTestGraph()
	engine := NewQueryEngine(g, 10)
	_, err := engine.CallStack("NonExistent", Both)
	if err == nil {
		t.Error("expected error")
	}
}
