package query

import (
	"strings"
	"testing"

	"github.com/dan-strohschein/cartograph/pkg/graph"
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

func TestCallStackZeroCallers(t *testing.T) {
	g := graph.NewGraph()
	// An orphan function with no callers or callees.
	orphan := graph.Node{
		ID:       graph.MakeNodeID("m", graph.KindFunction, "Orphan"),
		Kind:     graph.KindFunction,
		Name:     "Orphan",
		Module:   "m",
		Metadata: map[string]string{},
	}
	g.AddNode(orphan)

	engine := NewQueryEngine(g, 10)
	result, err := engine.CallStack("Orphan", Reverse)
	if err != nil {
		t.Fatalf("CallStack failed: %v", err)
	}
	if !strings.Contains(result.Summary, "Found 0 callers") {
		t.Errorf("expected summary to contain 'Found 0 callers', got: %s", result.Summary)
	}
	t.Logf("Summary: %s", result.Summary)
}

func TestFuzzyMatchSuggestions(t *testing.T) {
	g := graph.NewGraph()
	for _, name := range []string{"GenerateEdits", "GenerateAidEdits", "GenerateDiff"} {
		g.AddNode(graph.Node{
			ID:       graph.MakeNodeID("m", graph.KindFunction, name),
			Kind:     graph.KindFunction,
			Name:     name,
			Module:   "m",
			Metadata: map[string]string{},
		})
	}

	engine := NewQueryEngine(g, 10)
	_, err := engine.CallStack("GenerateEdit", Forward)
	if err == nil {
		t.Fatal("expected NotFoundError for misspelled name")
	}
	nfe, ok := err.(*NotFoundError)
	if !ok {
		t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
	}
	if len(nfe.Suggestions) == 0 {
		t.Fatal("expected suggestions, got none")
	}
	found := false
	for _, s := range nfe.Suggestions {
		if s == "GenerateEdits" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'GenerateEdits' in suggestions, got: %v", nfe.Suggestions)
	}
	t.Logf("Suggestions: %v", nfe.Suggestions)
}

func TestMethodNameExpansion(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode(graph.Node{
		ID:       graph.MakeNodeID("m", graph.KindMethod, "DB.Compact"),
		Kind:     graph.KindMethod,
		Name:     "DB.Compact",
		Module:   "m",
		Metadata: map[string]string{},
	})

	engine := NewQueryEngine(g, 10)
	_, err := engine.CallStack("Compact", Forward)
	if err == nil {
		t.Fatal("expected NotFoundError (bare name should not resolve directly)")
	}
	nfe, ok := err.(*NotFoundError)
	if !ok {
		t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
	}
	found := false
	for _, s := range nfe.Suggestions {
		if s == "DB.Compact" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'DB.Compact' in suggestions, got: %v", nfe.Suggestions)
	}
	t.Logf("Suggestions: %v", nfe.Suggestions)
}

func TestSearch(t *testing.T) {
	g := buildTestGraph()
	engine := NewQueryEngine(g, 10)

	result, err := engine.Search("*User*", "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Total == 0 {
		t.Error("expected matches for '*User*'")
	}
	// Verify results are grouped by kind.
	for kind, nodes := range result.Matches {
		if len(nodes) == 0 {
			t.Errorf("empty slice for kind %s", kind)
		}
		// Verify sorted by name.
		for i := 1; i < len(nodes); i++ {
			if nodes[i-1].Name > nodes[i].Name {
				t.Errorf("kind %s not sorted: %s > %s", kind, nodes[i-1].Name, nodes[i].Name)
			}
		}
	}
	t.Logf("Search('*User*'): %d total matches across %d kinds", result.Total, len(result.Matches))
}

func TestSearchNoResults(t *testing.T) {
	g := buildTestGraph()
	engine := NewQueryEngine(g, 10)

	result, err := engine.Search("ZzzNonexistent999", "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 matches, got %d", result.Total)
	}
}

func TestListModule(t *testing.T) {
	g := buildTestGraph()
	engine := NewQueryEngine(g, 10)

	result, err := engine.ListModule("m")
	if err != nil {
		t.Fatalf("ListModule failed: %v", err)
	}
	if result.Total == 0 {
		t.Error("expected nodes in module 'm'")
	}
	// Verify nodes are grouped by kind.
	for kind, nodes := range result.Matches {
		if len(nodes) == 0 {
			t.Errorf("empty slice for kind %s", kind)
		}
	}
	t.Logf("ListModule('m'): %d total nodes across %d kinds", result.Total, len(result.Matches))
}

func TestTraverseMaxPaths(t *testing.T) {
	g := graph.NewGraph()
	root := graph.Node{
		ID:       graph.MakeNodeID("m", graph.KindFunction, "Root"),
		Kind:     graph.KindFunction,
		Name:     "Root",
		Module:   "m",
		Metadata: map[string]string{},
	}
	g.AddNode(root)

	// Build a wide, shallow graph: 50 mid nodes, each with 50 leaves = 2500 paths.
	for i := 0; i < 50; i++ {
		midName := "Mid" + strings.Repeat("X", 0) + string(rune('A'+i%26)) + strings.Repeat("0", i/26)
		mid := graph.Node{
			ID:       graph.MakeNodeID("m", graph.KindFunction, midName),
			Kind:     graph.KindFunction,
			Name:     midName,
			Module:   "m",
			Metadata: map[string]string{},
		}
		g.AddNode(mid)
		g.AddEdge(graph.Edge{Source: root.ID, Target: mid.ID, Kind: graph.EdgeCalls})
		for j := 0; j < 50; j++ {
			leafName := midName + "_Leaf" + string(rune('A'+j%26)) + strings.Repeat("0", j/26)
			leaf := graph.Node{
				ID:       graph.MakeNodeID("m", graph.KindFunction, leafName),
				Kind:     graph.KindFunction,
				Name:     leafName,
				Module:   "m",
				Metadata: map[string]string{},
			}
			g.AddNode(leaf)
			g.AddEdge(graph.Edge{Source: mid.ID, Target: leaf.ID, Kind: graph.EdgeCalls})
		}
	}

	engine := NewQueryEngine(g, 10)
	paths := engine.Traverse(root.ID, nil, Forward, 10)
	if len(paths) > maxTraversalPaths {
		t.Errorf("expected paths capped at %d, got %d", maxTraversalPaths, len(paths))
	}
	t.Logf("Traverse returned %d paths (max %d)", len(paths), maxTraversalPaths)
}

func TestListModulePartialMatch(t *testing.T) {
	g := graph.NewGraph()
	// Create nodes in a module with a longer name.
	g.AddNode(graph.Node{
		ID:       graph.MakeNodeID("github.com/example/pkg", graph.KindModule, "github.com/example/pkg"),
		Kind:     graph.KindModule,
		Name:     "github.com/example/pkg",
		Module:   "github.com/example/pkg",
		Metadata: map[string]string{},
	})
	g.AddNode(graph.Node{
		ID:       graph.MakeNodeID("github.com/example/pkg", graph.KindFunction, "DoWork"),
		Kind:     graph.KindFunction,
		Name:     "DoWork",
		Module:   "github.com/example/pkg",
		Metadata: map[string]string{},
	})

	engine := NewQueryEngine(g, 10)

	// Partial match: "example/pkg" should match "github.com/example/pkg".
	result, err := engine.ListModule("example/pkg")
	if err != nil {
		t.Fatalf("ListModule partial match failed: %v", err)
	}
	if result.Total == 0 {
		t.Error("expected nodes via partial module match")
	}
	t.Logf("ListModule('example/pkg') partial: %d total nodes", result.Total)
}
