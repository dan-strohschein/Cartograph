package loader

import (
	"regexp"
	"strings"

	"github.com/dan-strohschein/aidkit/pkg/parser"
	"github.com/dan-strohschein/cartograph/pkg/graph"
)

// Tier 4 (agentic / dataflow) extraction. See AID spec §7.
//
// A @graph entry owns internal GraphNode and Frame nodes; @tool/@agent/@prompt/
// @model entries become first-class nodes (created via the generic entryToNode
// path) and gain relationship edges here.

// reducerRe captures a "reducer: <value>" constraint in a @channels field line.
var reducerRe = regexp.MustCompile(`(?i)reducer:\s*([A-Za-z][A-Za-z0-9_:-]*)`)

// effectsRe captures a trailing "[Effects]" annotation on a node/edge line.
var effectsRe = regexp.MustCompile(`\[([^\]]*)\]\s*$`)

// frameDirRe finds a frame direction keyword anywhere in a frame description.
var frameDirRe = regexp.MustCompile(`(?i)\b(downstream|upstream|bidirectional)\b`)

// parseReducer extracts the reducer name from a @channels field description.
func parseReducer(desc string) string {
	if m := reducerRe.FindStringSubmatch(desc); m != nil {
		return m[1]
	}
	return ""
}

// graphNodeSpec is one parsed line of a @graph's @nodes block.
type graphNodeSpec struct {
	name    string // node name within the graph
	impl    string // fn/tool/agent that implements it (may be empty)
	effects string // raw effect tags, e.g. "Llm, Net"
}

// parseGraphNodes parses a @nodes block: "name: fn — description [Effects]".
func parseGraphNodes(f parser.Field) []graphNodeSpec {
	var specs []graphNodeSpec
	for _, line := range fieldLines(f) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		rest := strings.TrimSpace(line[colon+1:])
		spec := graphNodeSpec{name: name}
		if m := effectsRe.FindStringSubmatch(rest); m != nil {
			spec.effects = strings.TrimSpace(m[1])
			rest = strings.TrimSpace(rest[:strings.LastIndex(rest, "[")])
		}
		// fn is the token before the em-dash / hyphen description separator.
		implPart := rest
		if idx := indexSep(rest); idx >= 0 {
			implPart = strings.TrimSpace(rest[:idx])
		}
		if fields := strings.Fields(implPart); len(fields) > 0 {
			spec.impl = fields[0]
		}
		if name != "" {
			specs = append(specs, spec)
		}
	}
	return specs
}

// extractGraphInternalNodes builds the GraphNode and Frame nodes owned by a @graph.
func extractGraphInternalNodes(module string, entry parser.Entry) []graph.Node {
	var nodes []graph.Node
	graphName := entry.Name

	if f, ok := entry.Fields["nodes"]; ok {
		for _, spec := range parseGraphNodes(f) {
			local := graphName + "." + spec.name
			n := graph.Node{
				ID:            graph.MakeNodeID(module, graph.KindGraphNode, local),
				Kind:          graph.KindGraphNode,
				Name:          local,
				QualifiedName: module + "." + local,
				Module:        module,
				Metadata:      make(map[string]string),
			}
			if spec.impl != "" {
				n.Metadata["implements"] = spec.impl
			}
			if spec.effects != "" {
				n.Metadata["effects"] = spec.effects
			}
			nodes = append(nodes, n)
		}
	}

	if f, ok := entry.Fields["frames"]; ok {
		for _, line := range fieldLines(f) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			name := line
			if idx := indexSep(line); idx >= 0 {
				name = strings.TrimSpace(line[:idx])
			}
			name = strings.Fields(name + " ")[0]
			if name == "" {
				continue
			}
			n := graph.Node{
				ID:            graph.MakeNodeID(module, graph.KindFrame, name),
				Kind:          graph.KindFrame,
				Name:          name,
				QualifiedName: module + "." + name,
				Module:        module,
				Metadata:      make(map[string]string),
			}
			if m := frameDirRe.FindStringSubmatch(line); m != nil {
				n.Metadata["direction"] = strings.ToLower(m[1])
			}
			nodes = append(nodes, n)
		}
	}

	return nodes
}

// extractGraphEdges builds the topology edges for a @graph entry: containment,
// entry, flow, conditional routing, node→impl, and state.
func extractGraphEdges(module string, entry parser.Entry, nodeIndex map[string]graph.NodeID) []graph.Edge {
	var edges []graph.Edge
	graphName := entry.Name
	graphID := graph.MakeNodeID(module, graph.KindGraph, graphName)

	// Resolve a bare node name to its GraphNode ID within this graph.
	gnodeID := func(name string) (graph.NodeID, bool) {
		name = strings.TrimSpace(name)
		if name == "" || name == "END" || name == "START" {
			return "", false
		}
		return graph.MakeNodeID(module, graph.KindGraphNode, graphName+"."+name), true
	}

	// @state → UsesState edge to the shared-state type.
	if f, ok := entry.Fields["state"]; ok {
		if targetID, ok := resolveName(strings.TrimSpace(f.Value()), nodeIndex); ok {
			edges = append(edges, graph.Edge{Source: graphID, Target: targetID, Kind: graph.EdgeUsesState, Label: f.Value()})
		}
	}

	// @nodes → ContainsNode (graph → node) and ImplementedBy (node → fn/tool/agent).
	if f, ok := entry.Fields["nodes"]; ok {
		for _, spec := range parseGraphNodes(f) {
			nid, ok := gnodeID(spec.name)
			if !ok {
				continue
			}
			edges = append(edges, graph.Edge{Source: graphID, Target: nid, Kind: graph.EdgeContainsNode, Label: spec.name})
			if spec.impl != "" {
				if targetID, ok := resolveName(spec.impl, nodeIndex); ok {
					edges = append(edges, graph.Edge{Source: nid, Target: targetID, Kind: graph.EdgeImplementedBy, Label: spec.impl})
				}
			}
		}
	}

	// @entry → EntryNode edges.
	if f, ok := entry.Fields["entry"]; ok {
		for _, name := range splitTargets(f.Value()) {
			if nid, ok := gnodeID(name); ok {
				edges = append(edges, graph.Edge{Source: graphID, Target: nid, Kind: graph.EdgeEntryNode, Label: name})
			}
		}
	}

	// @edges → FlowsTo edges (with optional ": Frame [direction]" annotation).
	if f, ok := entry.Fields["edges"]; ok {
		for _, line := range fieldLines(f) {
			line = strings.TrimSpace(line)
			if line == "" || !strings.Contains(line, "->") {
				continue
			}
			parts := strings.SplitN(line, "->", 2)
			src := strings.TrimSpace(parts[0])
			rhs := strings.TrimSpace(parts[1])
			var frame string
			if idx := strings.Index(rhs, ":"); idx >= 0 {
				frame = strings.TrimSpace(rhs[idx+1:])
				rhs = strings.TrimSpace(rhs[:idx])
			}
			srcID, ok1 := gnodeID(src)
			dstID, ok2 := gnodeID(rhs)
			if ok1 && ok2 {
				edges = append(edges, graph.Edge{Source: srcID, Target: dstID, Kind: graph.EdgeFlowsTo, Label: rhs})
			}
			// Frame annotation → CarriesFrame edge from the source node.
			if ok1 && frame != "" {
				frameName := frame
				if m := effectsRe.FindStringSubmatch(frame); m != nil {
					frameName = strings.TrimSpace(frame[:strings.LastIndex(frame, "[")])
				}
				if fid, ok := nodeIndex[frameName]; ok {
					edges = append(edges, graph.Edge{Source: srcID, Target: fid, Kind: graph.EdgeCarriesFrame, Label: frameName})
				}
			}
		}
	}

	// @conditional_edges → ConditionalFlow (src → each target) + RoutedBy (src → router fn).
	if f, ok := entry.Fields["conditional_edges"]; ok {
		for _, line := range fieldLines(f) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			colon := strings.Index(line, ":")
			arrow := strings.Index(line, "->")
			if colon < 0 || arrow < 0 || arrow < colon {
				continue
			}
			src := strings.TrimSpace(line[:colon])
			router := strings.TrimSpace(line[colon+1 : arrow])
			targetsPart := strings.TrimSpace(line[arrow+2:])
			if idx := indexSep(targetsPart); idx >= 0 {
				targetsPart = strings.TrimSpace(targetsPart[:idx])
			}
			srcID, ok := gnodeID(src)
			if !ok {
				continue
			}
			if routerID, ok := resolveName(router, nodeIndex); ok {
				edges = append(edges, graph.Edge{Source: srcID, Target: routerID, Kind: graph.EdgeRoutedBy, Label: router})
			}
			for _, t := range strings.Split(targetsPart, "|") {
				if tid, ok := gnodeID(t); ok {
					edges = append(edges, graph.Edge{Source: srcID, Target: tid, Kind: graph.EdgeConditionalFlow, Label: strings.TrimSpace(t)})
				}
			}
		}
	}

	return edges
}

// extractAgentEdges builds relationship edges for an @agent entry.
func extractAgentEdges(module string, srcID graph.NodeID, entry parser.Entry, nodeIndex map[string]graph.NodeID) []graph.Edge {
	var edges []graph.Edge

	if f, ok := entry.Fields["model"]; ok {
		if targetID, ok := resolveName(strings.TrimSpace(f.Value()), nodeIndex); ok {
			edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeUsesModel, Label: f.Value()})
		}
	}
	if f, ok := entry.Fields["tools"]; ok {
		for _, tool := range parseList(f.Value()) {
			if targetID, ok := resolveName(tool, nodeIndex); ok {
				edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeUsesTool, Label: tool})
			}
		}
	}
	if f, ok := entry.Fields["handoffs"]; ok {
		for _, agent := range parseList(f.Value()) {
			if targetID, ok := resolveName(agent, nodeIndex); ok {
				edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeHandsOffTo, Label: agent})
			}
		}
	}
	if f, ok := entry.Fields["output"]; ok {
		if targetID, ok := resolveName(firstToken(f.Value()), nodeIndex); ok {
			edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeProducesOutput, Label: f.Value()})
		}
	}
	return edges
}

// extractPromptModelEdges builds edges for @prompt and @model entries.
func extractPromptModelEdges(srcID graph.NodeID, entry parser.Entry, nodeIndex map[string]graph.NodeID) []graph.Edge {
	var edges []graph.Edge

	if f, ok := entry.Fields["model"]; ok {
		if targetID, ok := resolveName(strings.TrimSpace(f.Value()), nodeIndex); ok {
			edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeUsesModel, Label: f.Value()})
		}
	}
	// @output (prompt) / @structured_output (model) → ProducesOutput edge.
	for _, key := range []string{"output", "structured_output"} {
		if f, ok := entry.Fields[key]; ok {
			if targetID, ok := resolveName(firstToken(f.Value()), nodeIndex); ok {
				edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeProducesOutput, Label: f.Value()})
			}
		}
	}
	return edges
}

// extractErrorsAtTypes pulls error type names from a workflow @errors_at block:
// "step N: ErrorType | OtherError — condition".
func extractErrorsAtTypes(f parser.Field) []string {
	var types []string
	seen := make(map[string]bool)
	for _, line := range fieldLines(f) {
		line = strings.TrimSpace(line)
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		rhs := strings.TrimSpace(line[colon+1:])
		if idx := indexSep(rhs); idx >= 0 {
			rhs = strings.TrimSpace(rhs[:idx])
		}
		for _, t := range strings.Split(rhs, "|") {
			t = strings.TrimSpace(t)
			if t != "" && !seen[t] {
				seen[t] = true
				types = append(types, t)
			}
		}
	}
	return types
}

// --- small shared helpers ---

// fieldLines returns all value lines of a field (inline value + continuations).
func fieldLines(f parser.Field) []string {
	lines := make([]string, 0, len(f.Lines)+1)
	if strings.TrimSpace(f.InlineValue) != "" {
		lines = append(lines, f.InlineValue)
	}
	return append(lines, f.Lines...)
}

// indexSep returns the index of the first description separator (em-dash or
// space-hyphen-space), or -1 if none.
func indexSep(s string) int {
	if idx := strings.Index(s, "—"); idx >= 0 {
		return idx
	}
	if idx := strings.Index(s, " - "); idx >= 0 {
		return idx
	}
	return -1
}

// splitTargets splits a list that may be bracketed/comma-separated or a single name.
func splitTargets(s string) []string {
	if list := parseList(s); len(list) > 0 {
		return list
	}
	if t := strings.TrimSpace(s); t != "" {
		return []string{t}
	}
	return nil
}

// firstToken returns the first whitespace-delimited token, stripping a leading
// "[" so "[Entity]" resolves to "Entity".
func firstToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if fields := strings.Fields(s); len(fields) > 0 {
		return strings.Trim(fields[0], "[]")
	}
	return ""
}

// resolveName resolves a (possibly qualified) name to a NodeID, falling back to
// the trailing simple name — mirroring the @calls resolution in edges.go.
func resolveName(name string, nodeIndex map[string]graph.NodeID) (graph.NodeID, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	if id, ok := nodeIndex[name]; ok {
		return id, true
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		if id, ok := nodeIndex[name[idx+1:]]; ok {
			return id, true
		}
	}
	return "", false
}
