package output

import (
	"encoding/json"
	"io"

	"github.com/dan-strohschein/cartograph/internal/query"
)

type jsonNode struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name"`
	Kind          string `json:"kind"`
	Module        string `json:"module"`
	SourceFile    string `json:"source_file,omitempty"`
	SourceLine    int    `json:"source_line,omitempty"`
}

type jsonEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Label  string `json:"label,omitempty"`
}

type jsonPath struct {
	Nodes []jsonNode `json:"nodes"`
	Edges []jsonEdge `json:"edges"`
	Depth int        `json:"depth"`
}

type jsonResult struct {
	Query     string     `json:"query"`
	Summary   string     `json:"summary"`
	NodeCount int        `json:"node_count"`
	MaxDepth  int        `json:"max_depth"`
	Paths     []jsonPath `json:"paths"`
}

type jsonEffectEntry struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name"`
	SourceFile    string `json:"source_file,omitempty"`
	SourceLine    int    `json:"source_line,omitempty"`
}

type jsonEffectReport struct {
	Function     string                       `json:"function"`
	TotalCallees int                          `json:"total_callees"`
	MaxDepth     int                          `json:"max_depth"`
	Effects      map[string][]jsonEffectEntry `json:"effects"`
}

// RenderJSON writes a query result as JSON.
func RenderJSON(w io.Writer, result *query.QueryResult) {
	jr := jsonResult{
		Query:     result.Query,
		Summary:   result.Summary,
		NodeCount: result.NodeCount,
		MaxDepth:  result.MaxDepth,
	}
	for _, p := range result.Paths {
		jp := jsonPath{Depth: p.Depth}
		for _, n := range p.Nodes {
			jp.Nodes = append(jp.Nodes, jsonNode{
				Name:          n.Name,
				QualifiedName: n.QualifiedName,
				Kind:          string(n.Kind),
				Module:        n.Module,
				SourceFile:    n.SourceFile,
				SourceLine:    n.SourceLine,
			})
		}
		for _, e := range p.Edges {
			jp.Edges = append(jp.Edges, jsonEdge{
				Source: string(e.Source),
				Target: string(e.Target),
				Kind:   string(e.Kind),
				Label:  e.Label,
			})
		}
		jr.Paths = append(jr.Paths, jp)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(jr)
}

// RenderEffectJSON writes an effect report as JSON.
func RenderEffectJSON(w io.Writer, report *query.EffectReport) {
	jr := jsonEffectReport{
		Function:     report.Function,
		TotalCallees: report.TotalCallees,
		MaxDepth:     report.MaxDepth,
		Effects:      make(map[string][]jsonEffectEntry),
	}
	for category, nodes := range report.Effects {
		for _, n := range nodes {
			jr.Effects[category] = append(jr.Effects[category], jsonEffectEntry{
				Name:          n.Name,
				QualifiedName: n.QualifiedName,
				SourceFile:    n.SourceFile,
				SourceLine:    n.SourceLine,
			})
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(jr)
}
