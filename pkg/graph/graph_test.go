package graph

import "testing"

func TestNewGraph(t *testing.T) {
	g := NewGraph()
	if g == nil {
		t.Fatal("NewGraph returned nil")
	}
	stats := g.Stats()
	if stats.NodeCount != 0 || stats.EdgeCount != 0 {
		t.Errorf("empty graph should have 0 nodes/edges, got %d/%d", stats.NodeCount, stats.EdgeCount)
	}
}

func TestAddNodeAndLookup(t *testing.T) {
	g := NewGraph()

	n := Node{
		ID:            MakeNodeID("mod", KindFunction, "Foo"),
		Kind:          KindFunction,
		Name:          "Foo",
		QualifiedName: "mod.Foo",
		Module:        "mod",
		Purpose:       "test func",
		Metadata:      map[string]string{},
	}
	g.AddNode(n)

	// Lookup by ID.
	found, err := g.NodeByID(n.ID)
	if err != nil {
		t.Fatalf("NodeByID failed: %v", err)
	}
	if found.Name != "Foo" {
		t.Errorf("expected name Foo, got %s", found.Name)
	}

	// Lookup by kind.
	funcs := g.NodesByKind(KindFunction)
	if len(funcs) != 1 || funcs[0].Name != "Foo" {
		t.Errorf("NodesByKind(Function) expected [Foo], got %v", funcs)
	}

	// Lookup by name.
	byName := g.NodesByName("Foo")
	if len(byName) != 1 {
		t.Errorf("NodesByName expected 1, got %d", len(byName))
	}

	// Not found.
	_, err = g.NodeByID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestAddEdge(t *testing.T) {
	g := NewGraph()

	n1 := Node{ID: MakeNodeID("m", KindFunction, "A"), Kind: KindFunction, Name: "A", Module: "m", Metadata: map[string]string{}}
	n2 := Node{ID: MakeNodeID("m", KindFunction, "B"), Kind: KindFunction, Name: "B", Module: "m", Metadata: map[string]string{}}
	g.AddNode(n1)
	g.AddNode(n2)

	e := Edge{Source: n1.ID, Target: n2.ID, Kind: EdgeCalls, Label: "A->B"}
	g.AddEdge(e)

	stats := g.Stats()
	if stats.EdgeCount != 1 {
		t.Errorf("expected 1 edge, got %d", stats.EdgeCount)
	}

	out := g.OutEdges(n1.ID)
	if len(out) != 1 || out[0].Kind != EdgeCalls {
		t.Error("expected 1 outgoing Calls edge from n1")
	}

	in := g.InEdges(n2.ID)
	if len(in) != 1 || in[0].Kind != EdgeCalls {
		t.Error("expected 1 incoming Calls edge to n2")
	}
}

func TestAddEdgeSkipsInvalidNodes(t *testing.T) {
	g := NewGraph()
	n1 := Node{ID: MakeNodeID("m", KindFunction, "A"), Kind: KindFunction, Name: "A", Module: "m", Metadata: map[string]string{}}
	g.AddNode(n1)

	// Edge to nonexistent node should be silently ignored.
	g.AddEdge(Edge{Source: n1.ID, Target: "nonexistent", Kind: EdgeCalls})
	if g.Stats().EdgeCount != 0 {
		t.Error("edge to nonexistent node should be ignored")
	}
}

func TestDuplicateNode(t *testing.T) {
	g := NewGraph()
	n := Node{ID: MakeNodeID("m", KindFunction, "A"), Kind: KindFunction, Name: "A", Module: "m", Metadata: map[string]string{}}
	g.AddNode(n)
	g.AddNode(n) // duplicate
	if g.Stats().NodeCount != 1 {
		t.Error("duplicate node should not increase count")
	}
}

func TestStats(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{ID: MakeNodeID("m1", KindFunction, "F"), Kind: KindFunction, Name: "F", Module: "m1", Metadata: map[string]string{}})
	g.AddNode(Node{ID: MakeNodeID("m1", KindType, "T"), Kind: KindType, Name: "T", Module: "m1", Metadata: map[string]string{}})
	g.AddNode(Node{ID: MakeNodeID("m2", KindFunction, "G"), Kind: KindFunction, Name: "G", Module: "m2", Metadata: map[string]string{}})

	stats := g.Stats()
	if stats.NodeCount != 3 {
		t.Errorf("expected 3 nodes, got %d", stats.NodeCount)
	}
	if stats.Modules != 2 {
		t.Errorf("expected 2 modules, got %d", stats.Modules)
	}
	if stats.NodesByKind["Function"] != 2 {
		t.Errorf("expected 2 functions, got %d", stats.NodesByKind["Function"])
	}
}

func TestEdgesByKind(t *testing.T) {
	g := NewGraph()
	n1 := Node{ID: MakeNodeID("m", KindFunction, "A"), Kind: KindFunction, Name: "A", Module: "m", Metadata: map[string]string{}}
	n2 := Node{ID: MakeNodeID("m", KindFunction, "B"), Kind: KindFunction, Name: "B", Module: "m", Metadata: map[string]string{}}
	n3 := Node{ID: MakeNodeID("m", KindType, "T"), Kind: KindType, Name: "T", Module: "m", Metadata: map[string]string{}}
	g.AddNode(n1)
	g.AddNode(n2)
	g.AddNode(n3)

	g.AddEdge(Edge{Source: n1.ID, Target: n2.ID, Kind: EdgeCalls, Label: "A->B"})
	g.AddEdge(Edge{Source: n1.ID, Target: n3.ID, Kind: EdgeReturns, Label: "A->T"})
	g.AddEdge(Edge{Source: n2.ID, Target: n3.ID, Kind: EdgeAccepts, Label: "B->T"})

	calls := g.EdgesByKind(EdgeCalls)
	if len(calls) != 1 {
		t.Errorf("expected 1 Calls edge, got %d", len(calls))
	}
	returns := g.EdgesByKind(EdgeReturns)
	if len(returns) != 1 {
		t.Errorf("expected 1 Returns edge, got %d", len(returns))
	}
	accepts := g.EdgesByKind(EdgeAccepts)
	if len(accepts) != 1 {
		t.Errorf("expected 1 Accepts edge, got %d", len(accepts))
	}
	empty := g.EdgesByKind(EdgeProducesError)
	if len(empty) != 0 {
		t.Errorf("expected 0 ProducesError edges, got %d", len(empty))
	}
}

func TestMakeNodeID(t *testing.T) {
	id1 := MakeNodeID("mod", KindFunction, "Foo")
	id2 := MakeNodeID("mod", KindFunction, "Foo")
	id3 := MakeNodeID("mod", KindFunction, "Bar")

	if id1 != id2 {
		t.Error("same inputs should produce same ID")
	}
	if id1 == id3 {
		t.Error("different inputs should produce different IDs")
	}
}
