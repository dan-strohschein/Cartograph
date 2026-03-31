package query

import (
	"fmt"
	"strings"

	"github.com/dan-strohschein/cartograph/pkg/graph"
)

// SideEffects traces through all callees of a function and reports effects.
func (qe *QueryEngine) SideEffects(functionName string) (*EffectReport, error) {
	fn, err := qe.resolveFunction(functionName)
	if err != nil {
		return nil, err
	}

	// Forward-traverse Calls edges to find all callees.
	paths := qe.Traverse(fn.ID, []graph.EdgeKind{graph.EdgeCalls, graph.EdgeHasMethod}, Forward, qe.maxDepth)

	effects := make(map[string][]graph.Node)
	callees := make(map[graph.NodeID]bool)
	maxDepth := 0

	// Collect effects from the function itself.
	collectEffects(fn, effects)

	// Collect effects from all callees.
	for _, path := range paths {
		if path.Depth > maxDepth {
			maxDepth = path.Depth
		}
		for _, n := range path.Nodes {
			if n.ID == fn.ID {
				continue
			}
			callees[n.ID] = true
			collectEffects(n, effects)
		}
	}

	// If no explicit effects found, check for common patterns in purpose/metadata.
	if len(effects) == 0 {
		// Check the function and all reachable nodes for effect hints.
		allNodes := []graph.Node{fn}
		for _, p := range paths {
			allNodes = append(allNodes, p.Nodes...)
		}
		for _, n := range allNodes {
			inferEffects(n, effects)
		}
	}

	return &EffectReport{
		Function:     functionName,
		Effects:      effects,
		TotalCallees: len(callees),
		MaxDepth:     maxDepth,
	}, nil
}

func collectEffects(n graph.Node, effects map[string][]graph.Node) {
	if effectStr, ok := n.Metadata["effects"]; ok && effectStr != "" {
		// Parse effects like "Net, Fs" or "[Net, Db]".
		effectStr = strings.TrimPrefix(effectStr, "[")
		effectStr = strings.TrimSuffix(effectStr, "]")
		for _, e := range strings.Split(effectStr, ",") {
			e = strings.TrimSpace(e)
			if e != "" {
				effects[e] = append(effects[e], n)
			}
		}
	}
}

func inferEffects(n graph.Node, effects map[string][]graph.Node) {
	text := strings.ToLower(n.Purpose + " " + n.Name)
	if strings.Contains(text, "network") || strings.Contains(text, "http") || strings.Contains(text, "tcp") || strings.Contains(text, "socket") || strings.Contains(text, "dns") {
		effects["Net"] = append(effects["Net"], n)
	}
	if strings.Contains(text, "file") || strings.Contains(text, "disk") || strings.Contains(text, "write") || strings.Contains(text, "read") {
		effects["Fs"] = append(effects["Fs"], n)
	}
	if strings.Contains(text, "database") || strings.Contains(text, "query") || strings.Contains(text, "sql") || strings.Contains(text, "insert") || strings.Contains(text, "delete") {
		effects["Db"] = append(effects["Db"], n)
	}
	if strings.Contains(text, "log") || strings.Contains(text, "metric") || strings.Contains(text, "trace") {
		effects["Log"] = append(effects["Log"], n)
	}

	return
}

// FormatEffectReport returns a human-readable summary of an EffectReport.
func FormatEffectReport(r *EffectReport) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Side effects of %s:\n", r.Function)
	fmt.Fprintf(&sb, "  Call tree: %d callees, max depth %d\n", r.TotalCallees, r.MaxDepth)
	if len(r.Effects) == 0 {
		sb.WriteString("  No effects detected\n")
	}
	for category, nodes := range r.Effects {
		fmt.Fprintf(&sb, "  [%s]\n", category)
		seen := make(map[graph.NodeID]bool)
		for _, n := range nodes {
			if seen[n.ID] {
				continue
			}
			seen[n.ID] = true
			loc := ""
			if n.SourceFile != "" {
				loc = fmt.Sprintf(" (%s:%d)", n.SourceFile, n.SourceLine)
			}
			fmt.Fprintf(&sb, "    %s%s\n", n.QualifiedName, loc)
		}
	}
	return sb.String()
}
