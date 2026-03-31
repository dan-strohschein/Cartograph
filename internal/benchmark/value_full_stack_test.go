package benchmark

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dan-strohschein/cartograph/pkg/loader"
	"github.com/dan-strohschein/cartograph/pkg/output"
	"github.com/dan-strohschein/cartograph/pkg/query"
)

type FullStackResult struct {
	Scenario         string
	SourceFiles      int
	SourceTokens     int
	AIDFiles         int
	AIDTokens        int
	CartographTokens int
	CartographNodes  int
	TotalReduction   float64 // source / cartograph
}

func TestFullStack(t *testing.T) {
	if _, err := os.Stat(aidDir); err != nil {
		t.Skipf("AID data not available at %s", aidDir)
	}
	if _, err := os.Stat(sourceDir); err != nil {
		t.Skipf("Source tree not available at %s", sourceDir)
	}

	g, err := loader.LoadFromDirectory(aidDir)
	if err != nil {
		t.Fatalf("Failed to load AID files: %v", err)
	}
	stats := g.Stats()

	// Corpus baselines.
	totalSrc := ComputeSourceCost(sourceDir, []string{""})
	totalAID := ComputeAIDCost(aidDir, []string{""})

	t.Logf("Source tree : %d files, %d lines, ~%d tokens", totalSrc.FilesMatched, totalSrc.TotalLines, totalSrc.TokenEstimate())
	t.Logf("AID corpus : %d files, %d lines, ~%d tokens", totalAID.FilesMatched, totalAID.TotalLines, totalAID.TokenEstimate())
	t.Logf("Cartograph : %d nodes, %d edges, %d modules", stats.NodeCount, stats.EdgeCount, stats.Modules)

	engine := query.NewQueryEngine(g, 10)

	var results []FullStackResult
	var sumSrcTok, sumAIDTok, sumCGTok int

	for _, scenario := range Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			// Cartograph query.
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

			// Source cost.
			srcCost := ComputeSourceCost(sourceDir, scenario.GrepTerms)
			// AID cost.
			aidCost := ComputeAIDCost(aidDir, scenario.GrepTerms)

			var reduction float64
			if cgTokens > 0 {
				reduction = float64(srcCost.TokenEstimate()) / float64(cgTokens)
			}

			r := FullStackResult{
				Scenario:         scenario.Name,
				SourceFiles:      srcCost.FilesMatched,
				SourceTokens:     srcCost.TokenEstimate(),
				AIDFiles:         aidCost.FilesMatched,
				AIDTokens:        aidCost.TokenEstimate(),
				CartographTokens: cgTokens,
				CartographNodes:  cgNodeCount,
				TotalReduction:   reduction,
			}
			results = append(results, r)
			sumSrcTok += r.SourceTokens
			sumAIDTok += r.AIDTokens
			sumCGTok += r.CartographTokens
		})
	}

	// Summary table.
	t.Log("")
	t.Log("=== Full Stack Benchmark: Cartograph+AID vs. No Documentation ===")
	t.Log("")
	t.Logf("%-35s | %8s %5s | %8s %4s | %6s %4s | %9s",
		"Scenario", "Src tok", "files", "AID tok", "file", "CG tok", "nod", "Src→CG")
	t.Log(strings.Repeat("-", 105))
	for _, r := range results {
		t.Logf("%-35s | %8d %5d | %8d %4d | %6d %4d | %8.1fx",
			r.Scenario,
			r.SourceTokens, r.SourceFiles,
			r.AIDTokens, r.AIDFiles,
			r.CartographTokens, r.CartographNodes,
			r.TotalReduction)
	}
	t.Log(strings.Repeat("-", 105))

	aggReduction := float64(sumSrcTok) / float64(sumCGTok)
	t.Logf("%-35s | %8d %5s | %8d %4s | %6d %4s | %8.1fx",
		"TOTAL", sumSrcTok, "", sumAIDTok, "", sumCGTok, "", aggReduction)

	t.Log("")
	t.Logf("Agent with nothing       : %d tokens across 10 questions", sumSrcTok)
	t.Logf("Agent with AID only      : %d tokens (%.1fx less than source)", sumAIDTok, float64(sumSrcTok)/float64(sumAIDTok))
	t.Logf("Agent with AID+Cartograph: %d tokens (%.1fx less than source)", sumCGTok, aggReduction)
}
