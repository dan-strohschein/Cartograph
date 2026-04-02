package query

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dan-strohschein/cartograph/pkg/graph"
	"github.com/dan-strohschein/cartograph/pkg/loader"
)

const proofgoDir = "/Users/danstrohschein/Documents/CodeProjects/proofgo/backend/.aidocs"

func loadProofgoGraph(tb testing.TB) *graph.Graph {
	tb.Helper()
	if _, err := os.Stat(proofgoDir); err != nil {
		tb.Skipf("proofgo test data not available: %v", err)
	}
	g, err := loader.LoadFromDirectory(proofgoDir)
	if err != nil {
		tb.Fatal(err)
	}
	return g
}

// --- Existing query benchmarks ---

func BenchmarkTypeDependents_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	types := g.NodesByKind(graph.KindType)
	if len(types) == 0 {
		b.Skip("no types in proofgo graph")
	}
	typeName := types[0].Name
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.TypeDependents(typeName)
	}
}

func BenchmarkErrorProducers_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	errorEdges := g.EdgesByKind(graph.EdgeProducesError)
	if len(errorEdges) == 0 {
		b.Skip("no ProducesError edges in proofgo graph")
	}
	errorLabel := errorEdges[0].Label
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.ErrorProducers(errorLabel)
	}
}

func BenchmarkCallStack_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	funcs := g.NodesByKind(graph.KindFunction)
	if len(funcs) == 0 {
		b.Skip("no functions in proofgo graph")
	}
	funcName := funcs[0].Name
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.CallStack(funcName, Both)
	}
}

func BenchmarkFieldTouchers_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	fields := g.NodesByKind(graph.KindField)
	if len(fields) == 0 {
		b.Skip("no fields in proofgo graph")
	}
	name := fields[0].Name
	parts := strings.SplitN(name, ".", 2)
	if len(parts) != 2 {
		b.Skipf("field name %q not in Type.Field format", name)
	}
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.FieldTouchers(parts[0], parts[1])
	}
}

func BenchmarkTraverse_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	funcs := g.NodesByKind(graph.KindFunction)
	if len(funcs) == 0 {
		b.Skip("no functions in proofgo graph")
	}
	engine := NewQueryEngine(g, 10)
	startID := funcs[0].ID
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.Traverse(startID, []graph.EdgeKind{graph.EdgeCalls}, Forward, 10)
	}
}

// --- #4: SideEffects benchmark ---

func BenchmarkSideEffects_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	funcs := g.NodesByKind(graph.KindFunction)
	if len(funcs) == 0 {
		b.Skip("no functions in proofgo graph")
	}
	// Find a function with call edges for a meaningful benchmark.
	var funcName string
	for _, f := range funcs {
		out := g.OutEdges(f.ID)
		for _, e := range out {
			if e.Kind == graph.EdgeCalls {
				funcName = f.Name
				break
			}
		}
		if funcName != "" {
			break
		}
	}
	if funcName == "" {
		funcName = funcs[0].Name
	}
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.SideEffects(funcName)
	}
}

// --- #5: Search and ListModule benchmarks ---

func BenchmarkSearch_Wildcard_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Search("*", "")
	}
}

func BenchmarkSearch_Pattern_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Search("*Handler*", "")
	}
}

func BenchmarkSearch_KindFilter_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Search("*", graph.KindFunction)
	}
}

func BenchmarkListModule_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	modules := g.NodesByKind(graph.KindModule)
	if len(modules) == 0 {
		b.Skip("no modules in proofgo graph")
	}
	moduleName := modules[0].Name
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.ListModule(moduleName)
	}
}

func BenchmarkListModule_Partial_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	// Use a partial module name that triggers substring fallback.
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.ListModule("handlers")
	}
}

// --- #6: Node resolution with fuzzy matching ---

func BenchmarkResolveFunction_ExactMatch(b *testing.B) {
	g := loadProofgoGraph(b)
	funcs := g.NodesByKind(graph.KindFunction)
	if len(funcs) == 0 {
		b.Skip("no functions")
	}
	engine := NewQueryEngine(g, 10)
	name := funcs[0].Name
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.resolveFunction(name)
	}
}

func BenchmarkResolveFunction_NotFound_WithFuzzy(b *testing.B) {
	g := loadProofgoGraph(b)
	engine := NewQueryEngine(g, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will miss exact lookup, fall back to AllNodes scan, then run fuzzy matching.
		_, _ = engine.resolveFunction("NonExistentFunctionXyz")
	}
}

func BenchmarkResolveNode_ExactMatch(b *testing.B) {
	g := loadProofgoGraph(b)
	types := g.NodesByKind(graph.KindType)
	if len(types) == 0 {
		b.Skip("no types")
	}
	engine := NewQueryEngine(g, 10)
	name := types[0].Name
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.resolveNode(name, graph.KindType)
	}
}

// --- #8: Stats computation ---

func BenchmarkStats_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Stats()
	}
}

// --- AllNodes / AllEdges iteration ---

func BenchmarkAllNodes_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodes := g.AllNodes()
		_ = nodes
	}
}

func BenchmarkAllEdges_Proofgo(b *testing.B) {
	g := loadProofgoGraph(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		edges := g.AllEdges()
		_ = edges
	}
}

// --- Integration test ---

func TestLoadProofgoAndQueryAll(t *testing.T) {
	g := loadProofgoGraph(t)
	stats := g.Stats()
	t.Logf("Proofgo: %d nodes, %d edges, %d modules", stats.NodeCount, stats.EdgeCount, stats.Modules)
	for k, v := range stats.NodesByKind {
		t.Logf("  %s: %d", k, v)
	}
	for k, v := range stats.EdgesByKind {
		t.Logf("  %s: %d", k, v)
	}

	engine := NewQueryEngine(g, 10)

	// TypeDependents
	types := g.NodesByKind(graph.KindType)
	if len(types) > 0 {
		result, err := engine.TypeDependents(types[0].Name)
		if err != nil {
			t.Logf("TypeDependents(%s): %v", types[0].Name, err)
		} else {
			t.Logf("TypeDependents(%s): %d paths", types[0].Name, len(result.Paths))
		}
	}

	// CallStack
	funcs := g.NodesByKind(graph.KindFunction)
	if len(funcs) > 0 {
		result, err := engine.CallStack(funcs[0].Name, Both)
		if err != nil {
			t.Logf("CallStack(%s): %v", funcs[0].Name, err)
		} else {
			t.Logf("CallStack(%s): %d paths", funcs[0].Name, len(result.Paths))
		}
	}

	// FieldTouchers
	fields := g.NodesByKind(graph.KindField)
	if len(fields) > 0 {
		parts := strings.SplitN(fields[0].Name, ".", 2)
		if len(parts) == 2 {
			result, err := engine.FieldTouchers(parts[0], parts[1])
			if err != nil {
				t.Logf("FieldTouchers(%s): %v", fields[0].Name, err)
			} else {
				t.Logf("FieldTouchers(%s): %d paths", fields[0].Name, len(result.Paths))
			}
		}
	}

	// Search
	searchResult, err := engine.Search("*", "")
	if err != nil {
		t.Logf("Search(*): %v", err)
	} else {
		t.Logf("Search(*): %d total matches", searchResult.Total)
	}

	// Effects
	if len(funcs) > 0 {
		report, err := engine.SideEffects(funcs[0].Name)
		if err != nil {
			t.Logf("SideEffects(%s): %v", funcs[0].Name, err)
		} else {
			t.Logf("SideEffects(%s): %d callees, %d effect categories",
				funcs[0].Name, report.TotalCallees, len(report.Effects))
		}
	}

	// ErrorProducers
	errorEdges := g.EdgesByKind(graph.EdgeProducesError)
	if len(errorEdges) > 0 {
		result, err := engine.ErrorProducers(errorEdges[0].Label)
		if err != nil {
			t.Logf("ErrorProducers(%s): %v", errorEdges[0].Label, err)
		} else {
			t.Logf("ErrorProducers(%s): %d paths", errorEdges[0].Label, len(result.Paths))
		}
	}

	_ = filepath.Join // ensure import used
}
