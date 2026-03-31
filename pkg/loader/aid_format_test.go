package loader

import (
	"path/filepath"
	"testing"

	"github.com/dan-strohschein/cartograph/pkg/graph"
	"github.com/dan-strohschein/cartograph/pkg/query"
)

// formatCoverageDir returns the directory of hermetic AID fixtures covering loader edge extraction.
func formatCoverageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "format_coverage")
}

func countEdgesByKind(g *graph.Graph, k graph.EdgeKind) int {
	n := 0
	for _, e := range g.AllEdges() {
		if e.Kind == k {
			n++
		}
	}
	return n
}

func TestFormatCoverageLoadFromDirectory(t *testing.T) {
	dir := formatCoverageDir(t)
	g, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory: %v", err)
	}

	stats := g.Stats()
	if stats.Modules < 2 {
		t.Errorf("expected at least 2 modules (fmtcov/demo, fmtcov/peer), got %d", stats.Modules)
	}
	if stats.NodeCount < 10 {
		t.Errorf("expected many nodes from fixtures, got %d", stats.NodeCount)
	}

	// Node kinds that only appear when the corresponding AID constructs are parsed.
	// Note: @lock annotations require aidkit newer than v0.1.0 (lock is not in
	// annotationKinds there); loader support for KindLock/EdgeAcquires is covered
	// once the parser emits annotations.
	for _, pair := range []struct {
		kind  graph.NodeKind
		label string
	}{
		{graph.KindWorkflow, "Workflow"},
		{graph.KindTrait, "Trait"},
		{graph.KindConstant, "Constant"},
		{graph.KindMethod, "Method"},
	} {
		if len(g.NodesByKind(pair.kind)) == 0 {
			t.Errorf("expected at least one %s node", pair.label)
		}
	}

	// Edge kinds driven by @calls, @errors, @depends, @methods, @lock, @workflow @steps, etc.
	minByKind := map[graph.EdgeKind]int{
		graph.EdgeCalls:         2,
		graph.EdgeProducesError: 2,
		graph.EdgeDependsOn:     2,
		graph.EdgeHasMethod:     1,
		graph.EdgeStepOf:        1,
		graph.EdgeReferences:    1,
		graph.EdgeExtends:       1,
		graph.EdgeImplements:    1,
		graph.EdgeAccepts:       1,
		graph.EdgeHasField:      2,
	}
	for k, want := range minByKind {
		if got := countEdgesByKind(g, k); got < want {
			t.Errorf("edges kind %s: got %d, want >= %d", k, got, want)
		}
	}

	// Module-level DependsOn: demo module -> peer module.
	var demoModID, peerModID graph.NodeID
	for _, n := range g.AllNodes() {
		if n.Kind != graph.KindModule {
			continue
		}
		switch n.Name {
		case "fmtcov/demo":
			demoModID = n.ID
		case "fmtcov/peer":
			peerModID = n.ID
		}
	}
	if demoModID == "" || peerModID == "" {
		t.Fatal("expected fmtcov/demo and fmtcov/peer module nodes")
	}
	foundModDep := false
	for _, e := range g.OutEdges(demoModID) {
		if e.Kind == graph.EdgeDependsOn && e.Target == peerModID {
			foundModDep = true
			break
		}
	}
	if !foundModDep {
		t.Error("expected DependsOn edge from fmtcov/demo module to fmtcov/peer module")
	}
}

// TestFormatCoverageQueries runs the query engine on parsed fixtures (end-to-end AID → graph → query).
func TestFormatCoverageQueries(t *testing.T) {
	dir := formatCoverageDir(t)
	g, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory: %v", err)
	}

	engine := query.NewQueryEngine(g, 10)

	t.Run("ErrorProducers_ErrKnown", func(t *testing.T) {
		res, err := engine.ErrorProducers("ErrKnown")
		if err != nil {
			t.Fatalf("ErrorProducers(ErrKnown): %v", err)
		}
		if len(res.Paths) == 0 {
			t.Fatal("expected at least one path for ErrKnown producers")
		}
	})

	t.Run("ErrorProducers_OrphanError", func(t *testing.T) {
		// @errors line without a type node uses self-loop ProducesError; ErrorProducers matches by label.
		res, err := engine.ErrorProducers("OrphanError")
		if err != nil {
			t.Fatalf("ErrorProducers(OrphanError): %v", err)
		}
		if len(res.Paths) == 0 {
			t.Fatal("expected paths for OrphanError label")
		}
	})

	t.Run("FieldTouchers", func(t *testing.T) {
		res, err := engine.FieldTouchers("Widget", "count")
		if err != nil {
			t.Fatalf("FieldTouchers: %v", err)
		}
		if len(res.Paths) == 0 {
			t.Fatal("expected functions/methods touching Widget.count")
		}
	})

	t.Run("TypeDependents", func(t *testing.T) {
		res, err := engine.TypeDependents("Widget")
		if err != nil {
			t.Fatalf("TypeDependents(Widget): %v", err)
		}
		if len(res.Paths) == 0 {
			t.Fatal("expected dependents of Widget")
		}
	})

	t.Run("SideEffects", func(t *testing.T) {
		rep, err := engine.SideEffects("CallerChain")
		if err != nil {
			t.Fatalf("SideEffects(CallerChain): %v", err)
		}
		if _, ok := rep.Effects["Db"]; !ok {
			t.Fatalf("expected Db effect from CalleeWithFx, got %#v", rep.Effects)
		}
	})

	t.Run("CallStack_forward", func(t *testing.T) {
		res, err := engine.CallStack("CallerChain", query.Forward)
		if err != nil {
			t.Fatalf("CallStack forward: %v", err)
		}
		if len(res.Paths) == 0 {
			t.Fatal("expected callees of CallerChain")
		}
	})

	t.Run("Search_and_ListModule", func(t *testing.T) {
		sr, err := engine.Search("Widget*", "")
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if sr.Total == 0 {
			t.Error("expected Search matches for Widget*")
		}
		mr, err := engine.ListModule("fmtcov/demo")
		if err != nil {
			t.Fatalf("ListModule: %v", err)
		}
		if mr.Total == 0 {
			t.Error("expected nodes in fmtcov/demo")
		}
	})
}
