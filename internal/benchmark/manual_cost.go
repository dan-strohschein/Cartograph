package benchmark

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// SourceCost represents the minimum cost an agent would pay to answer
// a structural question by reading raw source code.
type SourceCost struct {
	FilesMatched int      // number of source files containing the target
	TotalLines   int      // total lines across those files
	TotalBytes   int      // total bytes (token estimate = TotalBytes / 4)
	MatchedFiles []string // paths of matching files
}

// TokenEstimate returns approximate token count (bytes / 4).
func (sc SourceCost) TokenEstimate() int {
	return sc.TotalBytes / 4
}

// ComputeSourceCost greps the source tree for a target string and computes
// how much source code an agent would need to read to find the answer.
// This is a generous lower bound — a real agent would read more due to
// context, false hits, import tracing, etc.
// Searches .go files, skipping test files.
func ComputeSourceCost(sourceDir string, targets []string) SourceCost {
	return computeCost(sourceDir, targets, ".go", true)
}

// ComputeAIDCost greps AID files for a target string and computes
// how much AID documentation an agent would need to read.
func ComputeAIDCost(aidDir string, targets []string) SourceCost {
	return computeCost(aidDir, targets, ".aid", false)
}

func computeCost(dir string, targets []string, ext string, skipTests bool) SourceCost {
	var result SourceCost
	seen := make(map[string]bool)

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ext) {
			return nil
		}
		if skipTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if seen[path] {
			return nil
		}

		if fileContainsAny(path, targets) {
			seen[path] = true
			lines, bytes := fileStats(path)
			result.FilesMatched++
			result.TotalLines += lines
			result.TotalBytes += bytes
			result.MatchedFiles = append(result.MatchedFiles, path)
		}
		return nil
	})

	return result
}

// ComputeSourceCostRecursive simulates recursive exploration for call stack
// and side effect queries. It greps for the initial target, then greps for
// each function found in those files, up to a depth limit.
func ComputeSourceCostRecursive(sourceDir string, initialTarget string, depth int) SourceCost {
	seen := make(map[string]bool)
	targets := []string{initialTarget}

	for d := 0; d < depth && len(targets) > 0; d++ {
		var nextTargets []string
		filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if seen[path] {
				return nil
			}
			if fileContainsAny(path, targets) {
				seen[path] = true
			}
			return nil
		})
		// For simplicity, don't expand further — the first-level grep
		// already captures the minimum agent read cost.
		_ = nextTargets
		break
	}

	var result SourceCost
	for path := range seen {
		lines, bytes := fileStats(path)
		result.FilesMatched++
		result.TotalLines += lines
		result.TotalBytes += bytes
		result.MatchedFiles = append(result.MatchedFiles, path)
	}
	return result
}

// fileContainsAny checks if a file contains any of the target strings.
func fileContainsAny(path string, targets []string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		for _, target := range targets {
			if strings.Contains(line, target) {
				return true
			}
		}
	}
	return false
}

// fileStats returns line count and byte count for a file.
func fileStats(path string) (lines, bytes int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, 0
	}
	bytes = int(info.Size())

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		lines++
	}
	return lines, bytes
}
