package benchmark

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dan-strohschein/cartograph/internal/loader"
	"github.com/dan-strohschein/cartograph/internal/output"
	"github.com/dan-strohschein/cartograph/internal/query"
)

func TestCartographVsAID(t *testing.T) {
	if _, err := os.Stat(aidDir); err != nil {
		t.Skipf("AID data not available at %s", aidDir)
	}

	// Load graph once.
	g, err := loader.LoadFromDirectory(aidDir)
	if err != nil {
		t.Fatalf("Failed to load AID files: %v", err)
	}
	stats := g.Stats()
	t.Logf("Graph: %d nodes, %d edges, %d modules", stats.NodeCount, stats.EdgeCount, stats.Modules)

	// Total AID corpus size.
	totalAIDCost := ComputeAIDCost(aidDir, []string{""})
	t.Logf("AID corpus: %d files, %d lines, ~%d tokens",
		totalAIDCost.FilesMatched, totalAIDCost.TotalLines, totalAIDCost.TokenEstimate())

	engine := query.NewQueryEngine(g, 10)

	var results []BenchmarkResult
	var totalCGTokens, totalAIDTokens int
	var totalAIDFiles int

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
				cgNodeCount = report.TotalCallees + 1
				for _, nodes := range report.Effects {
					cgNodeCount += len(nodes)
				}
			}

			cgTokens := buf.Len() / 4

			// Compute AID cost — grep AID files for the same terms.
			aidCost := ComputeAIDCost(aidDir, scenario.GrepTerms)
			aidTokens := aidCost.TokenEstimate()

			var reduction float64
			if cgTokens > 0 {
				reduction = float64(aidTokens) / float64(cgTokens)
			}

			br := BenchmarkResult{
				Scenario:         scenario.Name,
				CartographTokens: cgTokens,
				CartographNodes:  cgNodeCount,
				SourceFiles:      aidCost.FilesMatched,
				SourceLines:      aidCost.TotalLines,
				SourceTokens:     aidTokens,
				TokenReduction:   reduction,
				FileReduction:    aidCost.FilesMatched,
			}
			results = append(results, br)
			totalCGTokens += cgTokens
			totalAIDTokens += aidTokens
			totalAIDFiles += aidCost.FilesMatched

			t.Logf("CG: %d tokens, %d nodes | AID: %d files, %d lines, %d tokens | Reduction: %.1fx",
				cgTokens, cgNodeCount, aidCost.FilesMatched, aidCost.TotalLines, aidTokens, reduction)
		})
	}

	// Summary table.
	t.Log("")
	t.Log("=== Cartograph Value Benchmark (vs. raw AID files) ===")
	t.Log("")
	t.Logf("%-40s | %6s | %6s | %8s | %8s | %9s",
		"Scenario", "CG tok", "CG nod", "AID file", "AID tok", "Reduction")
	t.Log(strings.Repeat("-", 95))
	for _, r := range results {
		t.Logf("%-40s | %6d | %6d | %8d | %8d | %8.1fx",
			r.Scenario, r.CartographTokens, r.CartographNodes,
			r.SourceFiles, r.SourceTokens, r.TokenReduction)
	}
	t.Log(strings.Repeat("-", 95))

	avgReduction := float64(totalAIDTokens) / float64(totalCGTokens)
	avgFiles := float64(totalAIDFiles) / float64(len(results))
	t.Logf("%-40s | %6d | %6s | %8.0f | %8d | %8.1fx",
		"AGGREGATE", totalCGTokens, "—", avgFiles, totalAIDTokens, avgReduction)

	t.Log("")
	t.Logf("Total: Cartograph uses %d tokens to answer %d questions.", totalCGTokens, len(results))
	t.Logf("Agent reading raw AID files would need %d tokens (%.1fx more).", totalAIDTokens, avgReduction)
	t.Logf("Agent would need to open %.0f AID files on average per question.", avgFiles)
}

// TestAIDBaseline shows what an agent reading raw AID files faces per scenario.
func TestAIDBaseline(t *testing.T) {
	if _, err := os.Stat(aidDir); err != nil {
		t.Skipf("AID data not available at %s", aidDir)
	}

	total := ComputeAIDCost(aidDir, []string{""})
	t.Logf("Total AID corpus: %d files, %d lines, %d bytes (~%d tokens)",
		total.FilesMatched, total.TotalLines, total.TotalBytes, total.TokenEstimate())

	for _, scenario := range Scenarios {
		cost := ComputeAIDCost(aidDir, scenario.GrepTerms)
		t.Logf("%-40s: %4d files, %6d lines, ~%6d tokens",
			scenario.Name, cost.FilesMatched, cost.TotalLines, cost.TokenEstimate())
	}
}
