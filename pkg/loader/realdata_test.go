package loader

import (
	"os"
	"testing"

	"github.com/dan-strohschein/cartograph/pkg/graph"
)

func TestLoadRealAIDFiles(t *testing.T) {
	dir := "/tmp/syndr-aid/"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("syndr-aid test data not available")
	}

	g, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory failed: %v", err)
	}

	stats := g.Stats()
	t.Logf("Real data: %d nodes, %d edges, %d modules", stats.NodeCount, stats.EdgeCount, stats.Modules)

	// Check a specific type exists.
	nodes := g.NodesByName("DocumentFactoryImpl")
	t.Logf("NodesByName('DocumentFactoryImpl'): %d results", len(nodes))
	for _, n := range nodes {
		t.Logf("  %s (kind=%s, module=%s)", n.QualifiedName, n.Kind, n.Module)
	}

	// Sample some type names.
	typeNodes := g.NodesByKind(graph.KindType)
	for i, n := range typeNodes {
		if i >= 5 {
			break
		}
		t.Logf("  Type: name=%q kind=%s", n.Name, n.Kind)
	}

	// Check that types are findable.
	if len(typeNodes) > 0 {
		target := typeNodes[0]
		found := g.NodesByName(target.Name)
		if len(found) == 0 {
			t.Errorf("NodesByName(%q) returned 0 results but node exists in NodesByKind", target.Name)
		}
	}
}
