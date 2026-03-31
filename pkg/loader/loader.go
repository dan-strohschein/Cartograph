package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dan-strohschein/aidkit/pkg/parser"
	"github.com/dan-strohschein/cartograph/pkg/graph"
)

// LoadFromDirectory parses all .aid files in a directory and builds a Graph.
func LoadFromDirectory(dir string) (*graph.Graph, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".aid") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .aid files found in %s", dir)
	}
	return LoadFromFiles(paths)
}

// LoadFromFiles parses specific .aid files and builds a Graph.
func LoadFromFiles(paths []string) (*graph.Graph, error) {
	g := graph.NewGraph()
	// nodeIndex maps qualified names to NodeIDs for edge resolution.
	nodeIndex := make(map[string]graph.NodeID)

	// First pass: parse all files and extract nodes.
	var aidFiles []*parser.AidFile
	for _, path := range paths {
		af, _, err := parser.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
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

	// Second pass: extract edges.
	for _, af := range aidFiles {
		edges := extractEdges(af, nodeIndex)
		for _, e := range edges {
			g.AddEdge(e)
		}
	}

	return g, nil
}

// LoadWithManifest uses a manifest.aid to selectively load relevant packages.
func LoadWithManifest(manifestPath string, relevantPackages []string) (*graph.Graph, error) {
	af, _, err := parser.ParseFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest %s: %w", manifestPath, err)
	}
	if !af.IsManifest {
		return nil, fmt.Errorf("%s is not a manifest file", manifestPath)
	}

	// Build package → aid_file mapping and dependency graph.
	type pkgInfo struct {
		aidFile string
		depends []string
	}
	packages := make(map[string]pkgInfo)
	for _, entry := range af.Entries {
		name := entry.Name
		info := pkgInfo{}
		if f, ok := entry.Fields["aid_file"]; ok {
			info.aidFile = f.Value()
		}
		if f, ok := entry.Fields["depends"]; ok {
			info.depends = parseList(f.Value())
		}
		packages[name] = info
	}

	// Resolve transitive dependencies.
	toLoad := make(map[string]bool)
	var resolve func(pkg string)
	resolve = func(pkg string) {
		if toLoad[pkg] {
			return
		}
		toLoad[pkg] = true
		if info, ok := packages[pkg]; ok {
			for _, dep := range info.depends {
				resolve(dep)
			}
		}
	}
	for _, pkg := range relevantPackages {
		resolve(pkg)
	}

	// Collect file paths.
	dir := filepath.Dir(manifestPath)
	var paths []string
	for pkg := range toLoad {
		if info, ok := packages[pkg]; ok && info.aidFile != "" {
			paths = append(paths, filepath.Join(dir, info.aidFile))
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no AID files found for packages: %v", relevantPackages)
	}

	return LoadFromFiles(paths)
}

// parseList parses a bracketed or comma-separated list like "[a, b, c]".
func parseList(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
