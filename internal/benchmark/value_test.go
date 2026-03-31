package benchmark

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/dan-strohschein/cartograph/pkg/loader"
	"github.com/dan-strohschein/cartograph/pkg/output"
	"github.com/dan-strohschein/cartograph/pkg/query"
)

const (
	aidDir    = "/tmp/syndr-aid/"
	sourceDir = "/Users/danstrohschein/Documents/CodeProjects/golang/SyndrDB/src"
)

// BenchmarkResult holds the comparison data for one scenario.
type BenchmarkResult struct {
	Scenario          string
	CartographTokens  int
	CartographNodes   int
	SourceFiles       int
	SourceLines       int
	SourceTokens      int
	TokenReduction    float64
	FileReduction     int
}

func TestCartographValue(t *testing.T) {
	if _, err := os.Stat(aidDir); err != nil {
		t.Skipf("AID data not available at %s", aidDir)
	}
	if _, err := os.Stat(sourceDir); err != nil {
		t.Skipf("Source tree not available at %s", sourceDir)
	}

	// Load graph once.
	g, err := loader.LoadFromDirectory(aidDir)
	if err != nil {
		t.Fatalf("Failed to load AID files: %v", err)
	}
	stats := g.Stats()
	t.Logf("Graph: %d nodes, %d edges, %d modules", stats.NodeCount, stats.EdgeCount, stats.Modules)

	// Count total source tree size.
	totalSourceCost := ComputeSourceCost(sourceDir, []string{""})
	t.Logf("Source tree: %d files, %d lines, ~%d tokens",
		totalSourceCost.FilesMatched, totalSourceCost.TotalLines, totalSourceCost.TokenEstimate())

	engine := query.NewQueryEngine(g, 10)

	var results []BenchmarkResult
	var totalCGTokens, totalSrcTokens int
	var totalSrcFiles int

	for _, scenario := range Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			// Run Cartograph query.
			var buf bytes.Buffer
			var cgNodeCount int

			switch scenario.QueryType {
			case "depends":
				result, err := engine.TypeDependents(scenario.Target)
				if err != nil {
					t.Fatalf("Query failed: %v", err)
				}
				output.RenderTree(&buf, result)
				cgNodeCount = result.NodeCount

			case "field":
				result, err := engine.FieldTouchers(scenario.Target, scenario.Target2)
				if err != nil {
					t.Fatalf("Query failed: %v", err)
				}
				output.RenderTree(&buf, result)
				cgNodeCount = result.NodeCount

			case "effects":
				report, err := engine.SideEffects(scenario.Target)
				if err != nil {
					t.Fatalf("Query failed: %v", err)
				}
				output.RenderEffectTree(&buf, report)
				cgNodeCount = report.TotalCallees + 1 // +1 for the function itself
				for _, nodes := range report.Effects {
					cgNodeCount += len(nodes)
				}
			}

			cgOutput := buf.String()
			cgTokens := len(cgOutput) / 4

			// Compute source cost — what an agent would need to read.
			srcCost := ComputeSourceCost(sourceDir, scenario.GrepTerms)
			srcTokens := srcCost.TokenEstimate()

			// Compute reduction.
			var reduction float64
			if cgTokens > 0 {
				reduction = float64(srcTokens) / float64(cgTokens)
			}

			br := BenchmarkResult{
				Scenario:         scenario.Name,
				CartographTokens: cgTokens,
				CartographNodes:  cgNodeCount,
				SourceFiles:      srcCost.FilesMatched,
				SourceLines:      srcCost.TotalLines,
				SourceTokens:     srcTokens,
				TokenReduction:   reduction,
				FileReduction:    srcCost.FilesMatched,
			}
			results = append(results, br)
			totalCGTokens += cgTokens
			totalSrcTokens += srcTokens
			totalSrcFiles += srcCost.FilesMatched

			t.Logf("CG: %d tokens, %d nodes | Source: %d files, %d lines, %d tokens | Reduction: %.1fx",
				cgTokens, cgNodeCount, srcCost.FilesMatched, srcCost.TotalLines, srcTokens, reduction)
		})
	}

	// Print summary table.
	t.Log("")
	t.Log("=== Cartograph Value Benchmark (vs. raw source code) ===")
	t.Log("")
	t.Logf("%-40s | %6s | %6s | %8s | %8s | %9s",
		"Scenario", "CG tok", "CG nod", "Src file", "Src tok", "Reduction")
	t.Log(strings.Repeat("-", 95))
	for _, r := range results {
		t.Logf("%-40s | %6d | %6d | %8d | %8d | %8.1fx",
			r.Scenario, r.CartographTokens, r.CartographNodes,
			r.SourceFiles, r.SourceTokens, r.TokenReduction)
	}
	t.Log(strings.Repeat("-", 95))

	avgReduction := float64(totalSrcTokens) / float64(totalCGTokens)
	avgFiles := float64(totalSrcFiles) / float64(len(results))
	t.Logf("%-40s | %6d | %6s | %8.0f | %8d | %8.1fx",
		"AGGREGATE", totalCGTokens, "—", avgFiles, totalSrcTokens, avgReduction)

	t.Log("")
	t.Logf("Total: Cartograph uses %d tokens to answer %d questions.", totalCGTokens, len(results))
	t.Logf("Agent reading source would need %d tokens (%.1fx more).", totalSrcTokens, avgReduction)
	t.Logf("Agent would need to open %.0f files on average per question.", avgFiles)
}

// TestBootstrapGoldenAnswers runs all scenarios and prints detailed results
// for human verification. Run this to inspect what Cartograph returns.
func TestBootstrapGoldenAnswers(t *testing.T) {
	if _, err := os.Stat(aidDir); err != nil {
		t.Skipf("AID data not available at %s", aidDir)
	}

	g, err := loader.LoadFromDirectory(aidDir)
	if err != nil {
		t.Fatalf("Failed to load AID files: %v", err)
	}

	engine := query.NewQueryEngine(g, 10)

	for _, scenario := range Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			var buf bytes.Buffer

			switch scenario.QueryType {
			case "depends":
				result, err := engine.TypeDependents(scenario.Target)
				if err != nil {
					t.Fatalf("Query failed: %v", err)
				}
				output.RenderTree(&buf, result)
				t.Logf("Paths: %d, Nodes: %d", len(result.Paths), result.NodeCount)

			case "field":
				result, err := engine.FieldTouchers(scenario.Target, scenario.Target2)
				if err != nil {
					t.Fatalf("Query failed: %v", err)
				}
				output.RenderTree(&buf, result)
				t.Logf("Paths: %d, Nodes: %d", len(result.Paths), result.NodeCount)

			case "effects":
				report, err := engine.SideEffects(scenario.Target)
				if err != nil {
					t.Fatalf("Query failed: %v", err)
				}
				output.RenderEffectTree(&buf, report)
				t.Logf("Effects: %d categories, Callees: %d", len(report.Effects), report.TotalCallees)
			}

			// Print first 30 lines of output.
			lines := strings.Split(buf.String(), "\n")
			limit := 30
			if len(lines) < limit {
				limit = len(lines)
			}
			for _, line := range lines[:limit] {
				t.Logf("  %s", line)
			}
			if len(lines) > 30 {
				t.Logf("  ... (%d more lines)", len(lines)-30)
			}
			t.Logf("  Output: %d bytes, ~%d tokens", buf.Len(), buf.Len()/4)
		})
	}
}

// TestSourceTreeBaseline measures the raw source tree stats for context.
func TestSourceTreeBaseline(t *testing.T) {
	if _, err := os.Stat(sourceDir); err != nil {
		t.Skipf("Source tree not available at %s", sourceDir)
	}

	// Total source tree.
	total := ComputeSourceCost(sourceDir, []string{""})
	t.Logf("Total source tree: %d files, %d lines, %d bytes (~%d tokens)",
		total.FilesMatched, total.TotalLines, total.TotalBytes, total.TokenEstimate())

	// Per-scenario grep cost.
	for _, scenario := range Scenarios {
		cost := ComputeSourceCost(sourceDir, scenario.GrepTerms)
		t.Logf("%-40s: %4d files, %6d lines, ~%6d tokens",
			scenario.Name, cost.FilesMatched, cost.TotalLines, cost.TokenEstimate())
	}

	// Show what an agent is up against.
	t.Log("")
	t.Logf("An agent with NO documentation must grep and read from %d files / %d lines.",
		total.FilesMatched, total.TotalLines)
	t.Logf("Even with perfect grep, each structural question requires reading %s.",
		fmt.Sprintf("10-100+ files"))
}
