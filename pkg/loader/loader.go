package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/dan-strohschein/aidkit/pkg/discovery"
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

	// First pass: parse all files concurrently, then extract nodes sequentially.
	type parseResult struct {
		aidFile *parser.AidFile
		path    string
		err     error
	}

	results := make([]parseResult, len(paths))
	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers > len(paths) {
		numWorkers = len(paths)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, numWorkers)
	for i, path := range paths {
		wg.Add(1)
		go func(idx int, p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			af, _, err := parser.ParseFile(p)
			results[idx] = parseResult{aidFile: af, path: p, err: err}
		}(i, path)
	}
	wg.Wait()

	var aidFiles []*parser.AidFile
	for _, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("parsing %s: %w", r.path, r.err)
		}
		if r.aidFile.IsManifest {
			continue
		}
		aidFiles = append(aidFiles, r.aidFile)
		nodes := extractNodes(r.aidFile)
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

// LoadFromDirectoryCached is like LoadFromDirectory but uses a gob cache file
// to skip parsing when AID files haven't changed. The cache is stored as
// cartograph.cache alongside the AID files.
func LoadFromDirectoryCached(dir string) (*graph.Graph, error) {
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

	fingerprint, err := graph.ComputeFingerprint(paths)
	if err != nil {
		return nil, fmt.Errorf("computing fingerprint: %w", err)
	}

	cachePath := filepath.Join(dir, graph.CacheFileName)

	// Try loading from cache.
	if g, err := graph.LoadCache(cachePath, fingerprint); err == nil {
		return g, nil
	}

	// Cache miss or stale — rebuild.
	g, err := LoadFromFiles(paths)
	if err != nil {
		return nil, err
	}

	// Best-effort cache write — don't fail if we can't write.
	_ = graph.SaveCache(cachePath, g, fingerprint)

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

// LoadWithDiscovery uses aidkit's discovery protocol to find .aidocs/ from any
// starting directory, then loads all AID files into a Graph. Returns nil graph
// and nil error if no .aidocs/ is found.
func LoadWithDiscovery(startDir string) (*graph.Graph, *discovery.Result, error) {
	result, err := discovery.Discover(startDir)
	if err != nil {
		return nil, nil, fmt.Errorf("discovery from %s: %w", startDir, err)
	}
	if result == nil {
		return nil, nil, nil
	}

	if len(result.AidFiles) == 0 {
		return nil, result, fmt.Errorf("no .aid files found in %s", result.AidDocsPath)
	}

	g, err := LoadFromDirectoryCached(result.AidDocsPath)
	if err != nil {
		return nil, result, err
	}
	return g, result, nil
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
