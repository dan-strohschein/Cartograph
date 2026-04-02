package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dan-strohschein/cartograph/pkg/query"
)

const (
	proofgoBackend = "/Users/danstrohschein/Documents/CodeProjects/proofgo/backend"
	proofgoAidocs  = proofgoBackend + "/.aidocs"
)

// TestHeadToHead_VanillaVsCartograph compares two approaches to answering
// code comprehension questions:
//
//   1. "Vanilla Claude" — read all .go source files, grep for the target.
//      Every query re-reads source because Claude doesn't cache file contents
//      across tool calls in a real session.
//   2. "Cartograph" — load AID graph once, then run typed queries.
//      Cold start includes graph build. Warm queries are near-instant.
//
// We measure: wall-clock time, bytes read, files touched, and results found.
func TestHeadToHead_VanillaVsCartograph(t *testing.T) {
	if _, err := os.Stat(proofgoAidocs); err != nil {
		t.Skip("proofgo not available")
	}

	// Count source files and bytes for reporting.
	var srcFiles int
	var srcBytes int
	filepath.Walk(proofgoBackend, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".go") {
			srcFiles++
			srcBytes += int(info.Size())
		}
		return nil
	})

	// Count AID files and bytes.
	paths := proofgoPaths(t)
	var aidBytes int
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil {
			aidBytes += int(info.Size())
		}
	}

	t.Logf("=== Head-to-Head: Vanilla Source Reading vs Cartograph ===")
	t.Logf("Codebase: proofgo backend")
	t.Logf("  Source: %d .go files, %s", srcFiles, formatBytesH2H(srcBytes))
	t.Logf("  AID:    %d .aid files, %s", len(paths), formatBytesH2H(aidBytes))
	t.Logf("  Compression: %.1fx fewer files, %.1fx less data",
		float64(srcFiles)/float64(len(paths)), float64(srcBytes)/float64(aidBytes))
	t.Logf("")

	// Warm filesystem cache for both.
	vanillaReadAllSource(t)
	LoadFromFiles(paths)

	// --- Cartograph: load graph (parse from AID files) ---
	graphStart := time.Now()
	g, err := LoadFromFiles(paths)
	if err != nil {
		t.Fatal(err)
	}
	graphLoadTime := time.Since(graphStart)
	stats := g.Stats()
	engine := query.NewQueryEngine(g, 10)

	// --- Cartograph: load graph from cache ---
	// Ensure cache exists first.
	LoadFromDirectoryCached(proofgoAidocs)
	cachedStart := time.Now()
	gCached, err := LoadFromDirectoryCached(proofgoAidocs)
	if err != nil {
		t.Fatal(err)
	}
	cachedLoadTime := time.Since(cachedStart)
	cachedStats := gCached.Stats()

	t.Logf("Cartograph parse load:  %s (%d nodes, %d edges)",
		graphLoadTime.Round(time.Microsecond), stats.NodeCount, stats.EdgeCount)
	t.Logf("Cartograph cached load: %s (%d nodes, %d edges)",
		cachedLoadTime.Round(time.Microsecond), cachedStats.NodeCount, cachedStats.EdgeCount)
	t.Logf("Cache speedup: %.1fx faster load", float64(graphLoadTime)/float64(cachedLoadTime))
	t.Logf("")

	// --- Run 4 tasks, comparing vanilla (cold each time) vs cartograph (warm) ---

	type task struct {
		name    string
		vanilla func() (int, int)
		carto   func() (int, time.Duration)
	}

	tasks := []task{
		{
			name: "What depends on AssignmentDetailResponse?",
			vanilla: func() (int, int) {
				return vanillaGrep(t, "AssignmentDetailResponse")
			},
			carto: func() (int, time.Duration) {
				start := time.Now()
				r, err := engine.TypeDependents("AssignmentDetailResponse")
				dur := time.Since(start)
				if err != nil {
					return 0, dur
				}
				return len(r.Paths), dur
			},
		},
		{
			name: "What produces errors?",
			vanilla: func() (int, int) {
				return vanillaGrepError(t)
			},
			carto: func() (int, time.Duration) {
				start := time.Now()
				r, err := engine.ErrorProducers("error")
				dur := time.Since(start)
				if err != nil {
					return 0, dur
				}
				return len(r.Paths), dur
			},
		},
		{
			name: "Who calls GetAssignmentDetail?",
			vanilla: func() (int, int) {
				return vanillaGrep(t, "GetAssignmentDetail(")
			},
			carto: func() (int, time.Duration) {
				start := time.Now()
				r, err := engine.CallStack("GetAssignmentDetail", query.Reverse)
				dur := time.Since(start)
				if err != nil {
					return 0, dur
				}
				return len(r.Paths), dur
			},
		},
		{
			name: "Find all Handler types/functions",
			vanilla: func() (int, int) {
				return vanillaGrepDecl(t, "Handler")
			},
			carto: func() (int, time.Duration) {
				start := time.Now()
				r, err := engine.Search("*Handler*", "")
				dur := time.Since(start)
				if err != nil {
					return 0, dur
				}
				return r.Total, dur
			},
		},
	}

	t.Logf("  %-45s  %12s  %12s  %10s  %8s  %8s", "Task", "Vanilla", "Cartograph", "Speedup", "V.hits", "C.hits")
	t.Logf("  %-45s  %12s  %12s  %10s  %8s  %8s",
		strings.Repeat("-", 45), strings.Repeat("-", 12), strings.Repeat("-", 12),
		strings.Repeat("-", 10), strings.Repeat("-", 8), strings.Repeat("-", 8))

	var totalVanilla, totalCarto time.Duration

	for _, task := range tasks {
		// Vanilla: reads all source files every time.
		vStart := time.Now()
		vResults, _ := task.vanilla()
		vDur := time.Since(vStart)

		// Cartograph: query only (graph already loaded).
		cResults, cDur := task.carto()

		speedup := float64(vDur) / float64(cDur)
		totalVanilla += vDur
		totalCarto += cDur

		t.Logf("  %-45s  %12s  %12s  %9.0fx  %8d  %8d",
			task.name,
			vDur.Round(time.Microsecond),
			cDur.Round(time.Microsecond),
			speedup,
			vResults,
			cResults,
		)
	}

	t.Logf("")
	t.Logf("  %-45s  %12s  %12s  %9.0fx", "TOTAL (4 queries)",
		totalVanilla.Round(time.Microsecond),
		totalCarto.Round(time.Microsecond),
		float64(totalVanilla)/float64(totalCarto))
	t.Logf("")
	t.Logf("  Cartograph cold start (parse + 4 queries):  %s",
		(graphLoadTime + totalCarto).Round(time.Microsecond))
	t.Logf("  Cartograph cached start (cache + 4 queries): %s",
		(cachedLoadTime + totalCarto).Round(time.Microsecond))
	t.Logf("  Vanilla (4 queries, re-reads source each time): %s",
		totalVanilla.Round(time.Microsecond))
	t.Logf("  Break-even (parse): cartograph wins after ~%.0f queries",
		float64(graphLoadTime)/float64(totalVanilla/4-totalCarto/4))
	t.Logf("  Break-even (cached): cartograph wins after ~%.0f queries",
		float64(cachedLoadTime)/float64(totalVanilla/4-totalCarto/4))
	t.Logf("")
	t.Logf("  Data read per query:")
	t.Logf("    Vanilla:     %s (all source)", formatBytesH2H(srcBytes))
	t.Logf("    Cartograph:  0 bytes (graph in memory)")
	t.Logf("    AID one-time load: %s", formatBytesH2H(aidBytes))
}

// --- Vanilla helpers ---

func vanillaReadAllSource(t *testing.T) {
	t.Helper()
	filepath.Walk(proofgoBackend, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".go") {
			os.ReadFile(path)
		}
		return nil
	})
}

// vanillaGrep reads all .go source and counts lines matching a pattern.
func vanillaGrep(t *testing.T, pattern string) (results int, bytesRead int) {
	t.Helper()
	filepath.Walk(proofgoBackend, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		bytesRead += len(data)
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, pattern) {
				results++
			}
		}
		return nil
	})
	return
}

// vanillaGrepError counts lines with error return patterns.
func vanillaGrepError(t *testing.T) (results int, bytesRead int) {
	t.Helper()
	filepath.Walk(proofgoBackend, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		bytesRead += len(data)
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "return") && (strings.Contains(line, "err") || strings.Contains(line, "fmt.Errorf")) {
				results++
			}
		}
		return nil
	})
	return
}

// vanillaGrepDecl counts type/func declarations matching a pattern.
func vanillaGrepDecl(t *testing.T, pattern string) (results int, bytesRead int) {
	t.Helper()
	filepath.Walk(proofgoBackend, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		bytesRead += len(data)
		for _, line := range strings.Split(string(data), "\n") {
			if (strings.Contains(line, "type ") || strings.Contains(line, "func ")) && strings.Contains(line, pattern) {
				results++
			}
		}
		return nil
	})
	return
}

func formatBytesH2H(b int) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
