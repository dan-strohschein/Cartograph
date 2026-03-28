package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/dan-strohschein/cartograph/internal/graph"
	"github.com/dan-strohschein/cartograph/internal/query"
)

// RenderTree writes a query result as an indented tree.
func RenderTree(w io.Writer, result *query.QueryResult) {
	fmt.Fprintf(w, "%s\n", result.Summary)
	fmt.Fprintf(w, "(%d nodes, max depth %d)\n\n", result.NodeCount, result.MaxDepth)

	for _, path := range result.Paths {
		for i, node := range path.Nodes {
			indent := strings.Repeat("   ", i)
			prefix := "└─ "
			if i == 0 {
				prefix = ""
			}
			loc := formatLocation(node)
			edgeLabel := ""
			if i > 0 && i-1 < len(path.Edges) {
				edgeLabel = fmt.Sprintf(" [%s]", path.Edges[i-1].Kind)
			}
			fmt.Fprintf(w, "%s%s%s%s%s\n", indent, prefix, node.Name, loc, edgeLabel)
		}
		fmt.Fprintln(w)
	}
}

// RenderEffectTree writes an effect report as an indented tree.
func RenderEffectTree(w io.Writer, report *query.EffectReport) {
	fmt.Fprintf(w, "Side effects of %s\n", report.Function)
	fmt.Fprintf(w, "(%d callees, max depth %d)\n\n", report.TotalCallees, report.MaxDepth)

	if len(report.Effects) == 0 {
		fmt.Fprintln(w, "  No effects detected")
		return
	}

	for category, nodes := range report.Effects {
		fmt.Fprintf(w, "[%s]\n", category)
		seen := make(map[graph.NodeID]bool)
		for _, n := range nodes {
			if seen[n.ID] {
				continue
			}
			seen[n.ID] = true
			loc := formatLocation(n)
			fmt.Fprintf(w, "  └─ %s%s\n", n.QualifiedName, loc)
		}
		fmt.Fprintln(w)
	}
}

func formatLocation(n graph.Node) string {
	if n.SourceFile != "" {
		return fmt.Sprintf(" (%s:%d)", n.SourceFile, n.SourceLine)
	}
	return ""
}
