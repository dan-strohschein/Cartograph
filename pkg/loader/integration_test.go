//go:build integration

// Package loader integration tests: aid-gen-go over external Go trees (e.g. chisel), then LoadFromDirectory + queries.
//
// Run (example paths):
//
//	CARTOGRAPH_AID_GEN_GO=/path/to/AID/tools/aid-gen-go \
//	CARTOGRAPH_CHISEL_RESOLVE=/path/to/chisel/resolve \
//	CARTOGRAPH_CHISEL_EDIT=/path/to/chisel/edit \
//	go test -tags=integration ./internal/loader -v
package loader

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dan-strohschein/cartograph/pkg/graph"
	"github.com/dan-strohschein/cartograph/pkg/query"
)

// Environment variables (all optional; tests skip if prerequisites missing):
//   - CARTOGRAPH_AID_GEN_GO: directory containing aid-gen-go main (argument to "go build ... <dir>")
//   - CARTOGRAPH_CHISEL_RESOLVE: Go package dir passed to aid-gen-go for resolve tests
//   - CARTOGRAPH_CHISEL_EDIT: Go package dir for cross-package tests
var (
	aidGenGoSource  = os.Getenv("CARTOGRAPH_AID_GEN_GO")
	chiselResolve   = os.Getenv("CARTOGRAPH_CHISEL_RESOLVE")
	chiselEdit      = os.Getenv("CARTOGRAPH_CHISEL_EDIT")
	aidGenGoBinary  string
	integrationSkip string // set by TestMain when aid-gen-go cannot be built
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "integration-test-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	aidGenGoBinary = filepath.Join(tmpDir, "aid-gen-go")

	if aidGenGoSource == "" {
		integrationSkip = "CARTOGRAPH_AID_GEN_GO not set"
		os.Exit(m.Run())
	}
	if _, err := os.Stat(aidGenGoSource); err != nil {
		integrationSkip = "CARTOGRAPH_AID_GEN_GO path not found: " + aidGenGoSource
		os.Exit(m.Run())
	}

	cmd := exec.Command("go", "build", "-o", aidGenGoBinary, aidGenGoSource)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		integrationSkip = "go build aid-gen-go failed"
		aidGenGoBinary = ""
	}

	os.Exit(m.Run())
}

// generateAID runs aid-gen-go on the given source dirs and writes .aid files to outDir.
func generateAID(t *testing.T, outDir string, sourceDirs ...string) {
	t.Helper()

	if aidGenGoBinary == "" {
		if integrationSkip != "" {
			t.Skip(integrationSkip)
		}
		t.Skip("aid-gen-go binary not available (build failed)")
	}

	for _, srcDir := range sourceDirs {
		if _, err := os.Stat(srcDir); err != nil {
			t.Skipf("source directory not available: %s", srcDir)
		}
	}

	for _, srcDir := range sourceDirs {
		cmd := exec.Command(aidGenGoBinary, "--output", outDir, srcDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("aid-gen-go failed on %s: %v\n%s", srcDir, err, out)
		}
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("reading output dir: %v", err)
	}
	aidCount := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".aid" {
			aidCount++
		}
	}
	if aidCount == 0 {
		t.Fatal("aid-gen-go produced no .aid files")
	}
	t.Logf("Generated %d .aid files in %s", aidCount, outDir)
}

func TestAidGenToCartographCallStack(t *testing.T) {
	if chiselResolve == "" {
		t.Skip("CARTOGRAPH_CHISEL_RESOLVE not set")
	}
	outDir := t.TempDir()
	generateAID(t, outDir, chiselResolve)

	g, err := LoadFromDirectory(outDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory failed: %v", err)
	}

	stats := g.Stats()
	t.Logf("Graph: %d nodes, %d edges, %d modules", stats.NodeCount, stats.EdgeCount, stats.Modules)
	for k, v := range stats.NodesByKind {
		t.Logf("  nodes[%s] = %d", k, v)
	}
	for k, v := range stats.EdgesByKind {
		t.Logf("  edges[%s] = %d", k, v)
	}

	callsCount, ok := stats.EdgesByKind["Calls"]
	if !ok || callsCount == 0 {
		t.Fatal("expected Calls edges in graph, got 0")
	}
	t.Logf("Calls edges: %d", callsCount)

	engine := query.NewQueryEngine(g, 10)

	result, err := engine.CallStack("FindSourceLocations", query.Reverse)
	if err != nil {
		t.Fatalf("CallStack(FindSourceLocations, Reverse) failed: %v", err)
	}
	t.Logf("CallStack(FindSourceLocations, Reverse): %s", result.Summary)
	if result.NodeCount <= 1 {
		t.Errorf("expected callers of FindSourceLocations (NodeCount > 1), got %d", result.NodeCount)
	}

	result, err = engine.CallStack("Resolver.Resolve", query.Forward)
	if err != nil {
		t.Fatalf("CallStack(Resolver.Resolve, Forward) failed: %v", err)
	}
	t.Logf("CallStack(Resolver.Resolve, Forward): %s", result.Summary)
	if result.NodeCount <= 1 {
		t.Errorf("expected callees of Resolver.Resolve (NodeCount > 1), got %d", result.NodeCount)
	}
}

func TestAidGenToCartographSearch(t *testing.T) {
	if chiselResolve == "" {
		t.Skip("CARTOGRAPH_CHISEL_RESOLVE not set")
	}
	outDir := t.TempDir()
	generateAID(t, outDir, chiselResolve)

	g, err := LoadFromDirectory(outDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory failed: %v", err)
	}

	engine := query.NewQueryEngine(g, 10)

	searchResult, err := engine.Search("Resolve*", "")
	if err != nil {
		t.Fatalf("Search(Resolve*) failed: %v", err)
	}
	t.Logf("Search(Resolve*): %d total matches", searchResult.Total)
	for kind, nodes := range searchResult.Matches {
		for _, n := range nodes {
			t.Logf("  [%s] %s (%s)", kind, n.Name, n.QualifiedName)
		}
	}
	if searchResult.Total < 2 {
		t.Errorf("expected multiple nodes matching Resolve*, got %d", searchResult.Total)
	}

	moduleResult, err := engine.ListModule("resolve")
	if err != nil {
		t.Fatalf("ListModule(resolve) failed: %v", err)
	}
	t.Logf("ListModule(resolve): %d total nodes", moduleResult.Total)
	if moduleResult.Total == 0 {
		t.Error("expected nodes in resolve module")
	}
	if len(moduleResult.Matches) == 0 {
		t.Error("expected matches grouped by kind")
	}
	for kind, nodes := range moduleResult.Matches {
		t.Logf("  [%s]: %d nodes", kind, len(nodes))
	}
}

func TestAidGenToCartographFuzzyMatch(t *testing.T) {
	if chiselResolve == "" {
		t.Skip("CARTOGRAPH_CHISEL_RESOLVE not set")
	}
	outDir := t.TempDir()
	generateAID(t, outDir, chiselResolve)

	g, err := LoadFromDirectory(outDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory failed: %v", err)
	}

	engine := query.NewQueryEngine(g, 10)

	_, err = engine.CallStack("Resolv", query.Forward)
	if err == nil {
		t.Fatal("expected error for partial name 'Resolv', got nil")
	}

	nfe, ok := err.(*query.NotFoundError)
	if !ok {
		t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
	}
	t.Logf("NotFoundError for 'Resolv': %s", nfe.Error())

	if len(nfe.Suggestions) == 0 {
		t.Error("expected suggestions in NotFoundError")
	} else {
		t.Logf("Suggestions: %v", nfe.Suggestions)
		found := false
		for _, s := range nfe.Suggestions {
			if len(s) > 0 {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected non-empty suggestions")
		}
	}
}

func TestAidGenToCartographCrossPackage(t *testing.T) {
	if chiselResolve == "" {
		t.Skip("CARTOGRAPH_CHISEL_RESOLVE not set")
	}
	if chiselEdit == "" {
		t.Skip("CARTOGRAPH_CHISEL_EDIT not set")
	}
	outDir := t.TempDir()
	generateAID(t, outDir, chiselResolve, chiselEdit)

	g, err := LoadFromDirectory(outDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory failed: %v", err)
	}

	stats := g.Stats()
	t.Logf("Cross-package graph: %d nodes, %d edges, %d modules", stats.NodeCount, stats.EdgeCount, stats.Modules)

	if stats.Modules < 2 {
		t.Errorf("expected at least 2 modules, got %d", stats.Modules)
	}

	engine := query.NewQueryEngine(g, 10)

	for _, name := range []string{"GenerateEdits", "GenerateRenameEdits"} {
		searchResult, err := engine.Search(name, "")
		if err != nil {
			t.Fatalf("Search(%s) failed: %v", name, err)
		}
		if searchResult.Total == 0 {
			t.Errorf("expected to find %s in combined graph", name)
		} else {
			for kind, nodes := range searchResult.Matches {
				for _, n := range nodes {
					t.Logf("  Found [%s] %s (module=%s)", kind, n.QualifiedName, n.Module)
				}
			}
		}
	}

	allEdges := g.AllEdges()
	crossPackageCalls := 0
	for _, e := range allEdges {
		if e.Kind != graph.EdgeCalls {
			continue
		}
		srcNode, err1 := g.NodeByID(e.Source)
		tgtNode, err2 := g.NodeByID(e.Target)
		if err1 != nil || err2 != nil {
			continue
		}
		if srcNode.Module != tgtNode.Module {
			crossPackageCalls++
			t.Logf("  Cross-package call: %s (%s) -> %s (%s)",
				srcNode.QualifiedName, srcNode.Module,
				tgtNode.QualifiedName, tgtNode.Module)
		}
	}
	t.Logf("Cross-package Calls edges: %d", crossPackageCalls)
	if crossPackageCalls == 0 {
		t.Errorf("expected cross-package Calls edges between resolve and edit, got 0")
	}

	// Exercise ErrorProducers on the first ProducesError edge label in the generated graph.
	for _, e := range allEdges {
		if e.Kind != graph.EdgeProducesError || e.Label == "" {
			continue
		}
		res, err := engine.ErrorProducers(e.Label)
		if err != nil {
			t.Logf("ErrorProducers(%q): %v (skipping)", e.Label, err)
			continue
		}
		if len(res.Paths) == 0 {
			t.Errorf("ErrorProducers(%q): expected non-empty paths", e.Label)
		} else {
			t.Logf("ErrorProducers(%q): %s", e.Label, res.Summary)
		}
		break
	}
}
