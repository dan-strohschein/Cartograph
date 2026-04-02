package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/dan-strohschein/aidkit/pkg/parser"
	"github.com/dan-strohschein/cartograph/pkg/graph"
)

const proofgoDir = "/Users/danstrohschein/Documents/CodeProjects/proofgo/backend/.aidocs"

func proofgoPaths(tb testing.TB) []string {
	tb.Helper()
	entries, err := os.ReadDir(proofgoDir)
	if err != nil {
		tb.Skipf("proofgo test data not available: %v", err)
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".aid") {
			paths = append(paths, filepath.Join(proofgoDir, e.Name()))
		}
	}
	return paths
}

// --- Existing load benchmarks ---

func BenchmarkLoadFromFiles_Proofgo(b *testing.B) {
	paths := proofgoPaths(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadFromFiles(paths)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadFromDirectory_Proofgo(b *testing.B) {
	if _, err := os.Stat(proofgoDir); err != nil {
		b.Skip("proofgo not available")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadFromDirectory(proofgoDir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadWithDiscovery_Proofgo(b *testing.B) {
	proofgoBackend := filepath.Dir(proofgoDir)
	if _, err := os.Stat(proofgoDir); err != nil {
		b.Skip("proofgo not available")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g, _, err := LoadWithDiscovery(proofgoBackend)
		if err != nil {
			b.Fatal(err)
		}
		if g == nil {
			b.Fatal("no graph returned")
		}
	}
}

// --- Cached load benchmark ---

func BenchmarkLoadFromDirectoryCached_Proofgo_Cold(b *testing.B) {
	if _, err := os.Stat(proofgoDir); err != nil {
		b.Skip("proofgo not available")
	}
	cachePath := filepath.Join(proofgoDir, "cartograph.cache")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		os.Remove(cachePath)
		b.StartTimer()
		_, err := LoadFromDirectoryCached(proofgoDir)
		if err != nil {
			b.Fatal(err)
		}
	}
	os.Remove(cachePath)
}

func BenchmarkLoadFromDirectoryCached_Proofgo_Warm(b *testing.B) {
	if _, err := os.Stat(proofgoDir); err != nil {
		b.Skip("proofgo not available")
	}
	// Ensure cache exists.
	_, err := LoadFromDirectoryCached(proofgoDir)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadFromDirectoryCached(proofgoDir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// --- #1: Graph memory footprint ---

func TestGraphMemoryFootprint_Proofgo(t *testing.T) {
	paths := proofgoPaths(t)

	// Force GC and measure baseline.
	runtime.GC()
	var mBefore runtime.MemStats
	runtime.ReadMemStats(&mBefore)

	g, err := LoadFromFiles(paths)
	if err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	var mAfter runtime.MemStats
	runtime.ReadMemStats(&mAfter)

	stats := g.Stats()
	heapDelta := mAfter.HeapAlloc - mBefore.HeapAlloc

	t.Logf("=== Graph Memory Footprint ===")
	t.Logf("  Nodes: %d, Edges: %d, Modules: %d", stats.NodeCount, stats.EdgeCount, stats.Modules)
	t.Logf("  Heap delta: %s (%d bytes)", formatBytes(heapDelta), heapDelta)
	t.Logf("  Bytes/node (approx): %d", heapDelta/uint64(stats.NodeCount))
	t.Logf("  Bytes/edge (approx): %d", heapDelta/uint64(stats.EdgeCount))

	// Estimate struct sizes.
	t.Logf("  sizeof(Node): %d bytes", unsafe.Sizeof(graph.Node{}))
	t.Logf("  sizeof(Edge): %d bytes", unsafe.Sizeof(graph.Edge{}))
}

// --- #2: Parallel vs sequential parse comparison ---

func BenchmarkLoadSequential_Proofgo(b *testing.B) {
	paths := proofgoPaths(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g := graph.NewGraph()
		nodeIndex := make(map[string]graph.NodeID)
		var aidFiles []*parser.AidFile

		// Sequential parse — the old code path.
		for _, path := range paths {
			af, _, err := parser.ParseFile(path)
			if err != nil {
				b.Fatal(err)
			}
			if af.IsManifest {
				continue
			}
			aidFiles = append(aidFiles, af)
			nodes := extractNodes(af)
			for _, n := range nodes {
				g.AddNode(n)
				nodeIndex[n.QualifiedName] = n.ID
				nodeIndex[n.Name] = n.ID
			}
		}

		for _, af := range aidFiles {
			edges := extractEdges(af, nodeIndex)
			for _, e := range edges {
				g.AddEdge(e)
			}
		}
	}
}

// --- #3: Graph construction throughput (isolated from I/O) ---

func BenchmarkGraphConstruction_AddNode(b *testing.B) {
	// Pre-create nodes to isolate construction from parsing.
	paths := proofgoPaths(b)
	var allNodes []graph.Node
	for _, path := range paths {
		af, _, err := parser.ParseFile(path)
		if err != nil {
			b.Fatal(err)
		}
		if af.IsManifest {
			continue
		}
		allNodes = append(allNodes, extractNodes(af)...)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g := graph.NewGraph()
		for _, n := range allNodes {
			g.AddNode(n)
		}
	}
	b.ReportMetric(float64(len(allNodes)), "nodes/op")
}

func BenchmarkGraphConstruction_AddEdge(b *testing.B) {
	paths := proofgoPaths(b)

	// Build the graph once to collect edges.
	g, err := LoadFromFiles(paths)
	if err != nil {
		b.Fatal(err)
	}
	allEdges := g.AllEdges()
	allNodes := g.AllNodes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g2 := graph.NewGraph()
		for _, n := range allNodes {
			g2.AddNode(n)
		}
		for _, e := range allEdges {
			g2.AddEdge(e)
		}
	}
	b.ReportMetric(float64(len(allEdges)), "edges/op")
}

// --- #7: Scaling curve ---

func BenchmarkLoadScaling_10files(b *testing.B) {
	benchLoadSubset(b, 10)
}

func BenchmarkLoadScaling_25files(b *testing.B) {
	benchLoadSubset(b, 25)
}

func BenchmarkLoadScaling_50files(b *testing.B) {
	benchLoadSubset(b, 50)
}

func BenchmarkLoadScaling_AllFiles(b *testing.B) {
	benchLoadSubset(b, 0) // 0 = all
}

func benchLoadSubset(b *testing.B, count int) {
	b.Helper()
	paths := proofgoPaths(b)
	if count > 0 && count < len(paths) {
		paths = paths[:count]
	}
	b.ReportMetric(float64(len(paths)), "files")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g, err := LoadFromFiles(paths)
		if err != nil {
			b.Fatal(err)
		}
		stats := g.Stats()
		_ = stats
	}
}

// TestScalingCurve_Proofgo prints a human-readable scaling table.
func TestScalingCurve_Proofgo(t *testing.T) {
	paths := proofgoPaths(t)
	counts := []int{10, 25, 50, len(paths)}

	t.Logf("=== Scaling Curve ===")
	t.Logf("  %-8s %-8s %-8s %-12s", "Files", "Nodes", "Edges", "Modules")
	for _, count := range counts {
		subset := paths
		if count < len(paths) {
			subset = paths[:count]
		}
		g, err := LoadFromFiles(subset)
		if err != nil {
			t.Fatalf("failed at %d files: %v", count, err)
		}
		stats := g.Stats()
		t.Logf("  %-8d %-8d %-8d %-12d", len(subset), stats.NodeCount, stats.EdgeCount, stats.Modules)
	}
}

// --- helpers ---

func formatBytes(b uint64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
