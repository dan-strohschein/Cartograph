package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromDirectory(t *testing.T) {
	// Use the cartograph's own .aidocs as test data.
	aidocsDir := findAidocs(t)

	g, err := LoadFromDirectory(aidocsDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory failed: %v", err)
	}

	stats := g.Stats()
	if stats.NodeCount == 0 {
		t.Error("expected nodes, got 0")
	}
	if stats.EdgeCount == 0 {
		t.Error("expected edges, got 0")
	}
	t.Logf("Loaded graph: %d nodes, %d edges, %d modules", stats.NodeCount, stats.EdgeCount, stats.Modules)

	for k, v := range stats.NodesByKind {
		t.Logf("  %s: %d", k, v)
	}
	for k, v := range stats.EdgesByKind {
		t.Logf("  %s: %d", k, v)
	}
}

func TestLoadFromDirectoryNotFound(t *testing.T) {
	_, err := LoadFromDirectory("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestLoadFromDirectoryNoAidFiles(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadFromDirectory(dir)
	if err == nil {
		t.Error("expected error for directory with no .aid files")
	}
}

func TestExtractNodes(t *testing.T) {
	// Create a minimal AID file to test node extraction.
	dir := t.TempDir()
	content := `@module test/pkg
@lang go
@version 0.1.0
@purpose Test package

---

@fn DoSomething
@purpose Does something
@sig (input: str) -> Result? ! error

---

@type Result
@kind struct
@purpose A result type
@fields
  Value: str — The result value
  Code: int — Status code
`
	err := os.WriteFile(filepath.Join(dir, "test.aid"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	g, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory failed: %v", err)
	}

	stats := g.Stats()
	// Expect: 1 module + 1 function + 1 type + 2 fields = 5 nodes
	if stats.NodeCount < 4 {
		t.Errorf("expected at least 4 nodes, got %d", stats.NodeCount)
	}

	// Check function node exists.
	fns := g.NodesByName("DoSomething")
	if len(fns) == 0 {
		t.Error("expected DoSomething function node")
	}

	// Check type node exists.
	types := g.NodesByName("Result")
	if len(types) == 0 {
		t.Error("expected Result type node")
	}

	// Check field nodes exist.
	fields := g.NodesByName("Result.Value")
	if len(fields) == 0 {
		t.Error("expected Result.Value field node")
	}
}

func TestParseList(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"[a, b, c]", 3},
		{"a, b", 2},
		{"[single]", 1},
		{"", 0},
	}
	for _, tt := range tests {
		result := parseList(tt.input)
		if len(result) != tt.expected {
			t.Errorf("parseList(%q) = %v (len %d), want len %d", tt.input, result, len(result), tt.expected)
		}
	}
}

func findAidocs(t *testing.T) string {
	t.Helper()
	// Walk up from test directory to find .aidocs/.
	wd, err := os.Getwd()
	if err != nil {
		t.Skip("cannot get working directory")
	}
	for d := wd; ; d = filepath.Dir(d) {
		candidate := filepath.Join(d, ".aidocs")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
	}
	t.Skip(".aidocs directory not found")
	return ""
}
