package graph

import (
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	// CacheVersion is bumped when the serialization format changes.
	CacheVersion uint32 = 1
	// CacheFileName is the default name for the cache file.
	CacheFileName = "cartograph.cache"
)

// CacheHeader contains metadata for staleness detection.
type CacheHeader struct {
	Version     uint32
	Fingerprint [32]byte
}

// CacheData is the gob-serializable representation of a Graph.
// Only nodes and edges are stored; indexes are rebuilt on load.
type CacheData struct {
	Header CacheHeader
	Nodes  []Node
	Edges  []Edge
}

// SaveCache writes the graph to a gob-encoded cache file.
// Uses atomic write (tmp + rename) to prevent corrupt partial writes.
func SaveCache(path string, g *Graph, fingerprint [32]byte) error {
	data := CacheData{
		Header: CacheHeader{
			Version:     CacheVersion,
			Fingerprint: fingerprint,
		},
		Nodes: g.AllNodes(),
		Edges: g.AllEdges(),
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating cache temp file: %w", err)
	}

	enc := gob.NewEncoder(f)
	if err := enc.Encode(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encoding cache: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing cache temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming cache file: %w", err)
	}
	return nil
}

// LoadCache reads a gob-encoded cache file and rebuilds the graph with indexes.
// Returns an error if the file is missing, corrupt, or the fingerprint/version
// doesn't match (indicating staleness).
func LoadCache(path string, fingerprint [32]byte) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening cache: %w", err)
	}
	defer f.Close()

	var data CacheData
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding cache: %w", err)
	}

	if data.Header.Version != CacheVersion {
		return nil, fmt.Errorf("cache version mismatch: got %d, want %d", data.Header.Version, CacheVersion)
	}
	if data.Header.Fingerprint != fingerprint {
		return nil, fmt.Errorf("cache fingerprint mismatch (AID files changed)")
	}

	// Rebuild the graph with all indexes.
	g := NewGraph()
	for _, n := range data.Nodes {
		g.AddNode(n)
	}
	for _, e := range data.Edges {
		g.AddEdge(e)
	}

	return g, nil
}

// ComputeFingerprint creates a SHA-256 hash from the sorted list of
// (filename, size, mtime) tuples for the given file paths. This detects
// additions, deletions, renames, and content modifications.
func ComputeFingerprint(paths []string) ([32]byte, error) {
	type fileEntry struct {
		name  string
		size  int64
		mtime int64
	}

	entries := make([]fileEntry, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return [32]byte{}, fmt.Errorf("stat %s: %w", p, err)
		}
		entries = append(entries, fileEntry{
			name:  filepath.Base(p),
			size:  info.Size(),
			mtime: info.ModTime().UnixNano(),
		})
	}

	// Sort by filename for deterministic fingerprint.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	h := sha256.New()
	for _, e := range entries {
		fmt.Fprintf(h, "%s:%d:%d\n", e.name, e.size, e.mtime)
	}

	var fp [32]byte
	copy(fp[:], h.Sum(nil))
	return fp, nil
}
