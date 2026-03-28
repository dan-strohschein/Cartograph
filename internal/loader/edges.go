package loader

import (
	"regexp"
	"strings"

	"github.com/dan-strohschein/aidkit/pkg/parser"
	"github.com/dan-strohschein/cartograph/internal/graph"
)

// extractNodes converts parsed AID entries into graph nodes.
func extractNodes(af *parser.AidFile) []graph.Node {
	module := af.Header.Module
	var nodes []graph.Node

	// Module-level node.
	modNode := graph.Node{
		ID:            graph.MakeNodeID(module, graph.KindModule, module),
		Kind:          graph.KindModule,
		Name:          module,
		QualifiedName: module,
		Module:        module,
		Purpose:       af.Header.Purpose,
		Metadata:      make(map[string]string),
	}
	nodes = append(nodes, modNode)

	for _, entry := range af.Entries {
		n := entryToNode(module, entry)
		nodes = append(nodes, n)

		// If this is a type/trait with @fields, create Field nodes.
		if entry.Kind == "type" || entry.Kind == "trait" {
			if f, ok := entry.Fields["fields"]; ok {
				fieldNodes := extractFieldNodes(module, entry.Name, f)
				nodes = append(nodes, fieldNodes...)
			}
		}
	}

	// Workflow nodes.
	for _, wf := range af.Workflows {
		n := graph.Node{
			ID:            graph.MakeNodeID(module, graph.KindWorkflow, wf.Name),
			Kind:          graph.KindWorkflow,
			Name:          wf.Name,
			QualifiedName: module + "." + wf.Name,
			Module:        module,
			Metadata:      make(map[string]string),
		}
		if f, ok := wf.Fields["purpose"]; ok {
			n.Purpose = f.Value()
		}
		nodes = append(nodes, n)
	}

	return nodes
}

func entryToNode(module string, entry parser.Entry) graph.Node {
	kind := entryKindToNodeKind(entry)
	name := entry.Name

	// For methods like "Type.Method", use full name.
	qualifiedName := module + "." + name

	n := graph.Node{
		ID:            graph.MakeNodeID(module, kind, name),
		Kind:          kind,
		Name:          name,
		QualifiedName: qualifiedName,
		Module:        module,
		Metadata:      make(map[string]string),
	}

	if f, ok := entry.Fields["purpose"]; ok {
		n.Purpose = f.Value()
	}
	if f, ok := entry.Fields["sig"]; ok {
		n.Signature = f.Value()
	}
	if f, ok := entry.Fields["kind"]; ok {
		n.Type = f.Value()
	}

	// Extract source references.
	for _, f := range entry.Fields {
		for _, ref := range f.SourceRefs {
			if n.SourceFile == "" {
				n.SourceFile = ref.File
				n.SourceLine = ref.StartLine
			}
		}
	}

	// Store effects as metadata.
	if f, ok := entry.Fields["effects"]; ok {
		n.Metadata["effects"] = f.Value()
	}
	if f, ok := entry.Fields["thread_safety"]; ok {
		n.Metadata["thread_safety"] = f.Value()
	}
	if f, ok := entry.Fields["complexity"]; ok {
		n.Metadata["complexity"] = f.Value()
	}

	return n
}

func entryKindToNodeKind(entry parser.Entry) graph.NodeKind {
	switch entry.Kind {
	case "fn":
		// If the name contains ".", it's a method.
		if strings.Contains(entry.Name, ".") {
			return graph.KindMethod
		}
		return graph.KindFunction
	case "type":
		return graph.KindType
	case "trait":
		return graph.KindTrait
	case "const":
		return graph.KindConstant
	default:
		return graph.KindType
	}
}

// extractFieldNodes creates Field nodes from a @fields declaration.
func extractFieldNodes(module, typeName string, f parser.Field) []graph.Node {
	var nodes []graph.Node
	lines := append([]string{f.InlineValue}, f.Lines...)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "name: type — description" or "name: type"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		fieldName := strings.TrimSpace(parts[0])
		if fieldName == "" || fieldName == "(inner)" {
			continue
		}
		rest := strings.TrimSpace(parts[1])
		fieldType := rest
		if idx := strings.Index(rest, "—"); idx >= 0 {
			fieldType = strings.TrimSpace(rest[:idx])
		}
		if idx := strings.Index(rest, " — "); idx >= 0 {
			fieldType = strings.TrimSpace(rest[:idx])
		}

		qualName := module + "." + typeName + "." + fieldName
		n := graph.Node{
			ID:            graph.MakeNodeID(module, graph.KindField, typeName+"."+fieldName),
			Kind:          graph.KindField,
			Name:          typeName + "." + fieldName,
			QualifiedName: qualName,
			Module:        module,
			Type:          fieldType,
			Metadata:      make(map[string]string),
		}
		nodes = append(nodes, n)
	}
	return nodes
}

// extractEdges derives graph edges from AID field relationships.
func extractEdges(af *parser.AidFile, nodeIndex map[string]graph.NodeID) []graph.Edge {
	module := af.Header.Module
	var edges []graph.Edge

	// Module-level @depends.
	modID := graph.MakeNodeID(module, graph.KindModule, module)
	for _, dep := range af.Header.Depends {
		if targetID, ok := nodeIndex[dep]; ok {
			edges = append(edges, graph.Edge{Source: modID, Target: targetID, Kind: graph.EdgeDependsOn, Label: dep})
		}
	}

	for _, entry := range af.Entries {
		srcID := graph.MakeNodeID(module, entryKindToNodeKind(entry), entry.Name)

		// @sig → Accepts and Returns edges.
		if f, ok := entry.Fields["sig"]; ok {
			edges = append(edges, extractSigEdges(srcID, f.Value(), nodeIndex)...)
		}

		// @errors → ProducesError edges.
		if f, ok := entry.Fields["errors"]; ok {
			edges = append(edges, extractErrorEdges(srcID, f, nodeIndex)...)
		}

		// @related → References edges.
		if f, ok := entry.Fields["related"]; ok {
			for _, ref := range parseList(f.Value()) {
				if targetID, ok := nodeIndex[ref]; ok {
					edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeReferences, Label: ref})
				}
			}
		}

		// @extends → Extends edges.
		if f, ok := entry.Fields["extends"]; ok {
			for _, ext := range parseList(f.Value()) {
				if targetID, ok := nodeIndex[ext]; ok {
					edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeExtends, Label: ext})
				}
			}
		}

		// @implements → Implements edges.
		if f, ok := entry.Fields["implements"]; ok {
			for _, impl := range parseList(f.Value()) {
				if targetID, ok := nodeIndex[impl]; ok {
					edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeImplements, Label: impl})
				}
			}
		}

		// @fields → HasField edges.
		if entry.Kind == "type" || entry.Kind == "trait" {
			if _, ok := entry.Fields["fields"]; ok {
				fieldPrefix := entry.Name + "."
				for qname, targetID := range nodeIndex {
					if strings.HasPrefix(qname, fieldPrefix) && !strings.Contains(strings.TrimPrefix(qname, fieldPrefix), ".") {
						edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeHasField, Label: strings.TrimPrefix(qname, fieldPrefix)})
					}
				}
			}
		}

		// @methods → HasMethod edges.
		if f, ok := entry.Fields["methods"]; ok {
			for _, method := range parseList(f.Value()) {
				methodName := entry.Name + "." + method
				if targetID, ok := nodeIndex[methodName]; ok {
					edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeHasMethod, Label: method})
				}
			}
		}

		// @depends (entry-level) → DependsOn edges.
		if f, ok := entry.Fields["depends"]; ok {
			for _, dep := range parseList(f.Value()) {
				if targetID, ok := nodeIndex[dep]; ok {
					edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeDependsOn, Label: dep})
				}
			}
		}
	}

	// Workflow @steps → StepOf edges.
	for _, wf := range af.Workflows {
		wfID := graph.MakeNodeID(module, graph.KindWorkflow, wf.Name)
		if f, ok := wf.Fields["steps"]; ok {
			stepFuncs := extractStepFunctions(f)
			for _, fn := range stepFuncs {
				if targetID, ok := nodeIndex[fn]; ok {
					edges = append(edges, graph.Edge{Source: wfID, Target: targetID, Kind: graph.EdgeStepOf, Label: fn})
				}
			}
		}
	}

	return edges
}

// sigParamRe matches type references in signatures.
var sigParamRe = regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]*(?:\.[A-Z][A-Za-z0-9_]*)?)\??`)

// extractSigEdges parses a @sig value and creates Accepts/Returns edges.
func extractSigEdges(srcID graph.NodeID, sig string, nodeIndex map[string]graph.NodeID) []graph.Edge {
	var edges []graph.Edge

	// Split on "->" to separate params from return.
	parts := strings.SplitN(sig, "->", 2)
	if len(parts) == 0 {
		return nil
	}

	// Parse parameter types.
	paramPart := parts[0]
	// Strip "self" and "mut self"
	paramPart = strings.Replace(paramPart, "mut self,", "", 1)
	paramPart = strings.Replace(paramPart, "self,", "", 1)
	paramPart = strings.Replace(paramPart, "mut self", "", 1)
	paramPart = strings.Replace(paramPart, "self", "", 1)

	for _, match := range sigParamRe.FindAllStringSubmatch(paramPart, -1) {
		typeName := match[1]
		if targetID, ok := nodeIndex[typeName]; ok {
			edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeAccepts, Label: typeName})
		}
	}

	// Parse return type and error type.
	if len(parts) == 2 {
		returnPart := strings.TrimSpace(parts[1])
		// Split on "!" to get return type and error type.
		retParts := strings.SplitN(returnPart, "!", 2)
		retType := strings.TrimSpace(retParts[0])
		for _, match := range sigParamRe.FindAllStringSubmatch(retType, -1) {
			typeName := match[1]
			if typeName == "None" || typeName == "Self" {
				continue
			}
			if targetID, ok := nodeIndex[typeName]; ok {
				edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeReturns, Label: typeName})
			}
		}
		if len(retParts) == 2 {
			errType := strings.TrimSpace(retParts[1])
			for _, match := range sigParamRe.FindAllStringSubmatch(errType, -1) {
				typeName := match[1]
				if targetID, ok := nodeIndex[typeName]; ok {
					edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeProducesError, Label: typeName})
				}
			}
		}
	}

	return edges
}

// extractErrorEdges creates ProducesError edges from @errors field.
func extractErrorEdges(srcID graph.NodeID, f parser.Field, nodeIndex map[string]graph.NodeID) []graph.Edge {
	var edges []graph.Edge
	lines := append([]string{f.InlineValue}, f.Lines...)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Error lines typically start with the error type name, possibly followed by " — description".
		errType := line
		if idx := strings.Index(line, " — "); idx >= 0 {
			errType = strings.TrimSpace(line[:idx])
		} else if idx := strings.Index(line, "—"); idx >= 0 {
			errType = strings.TrimSpace(line[:idx])
		}
		// Also try splitting on " - ".
		if idx := strings.Index(errType, " - "); idx >= 0 {
			errType = strings.TrimSpace(errType[:idx])
		}
		errType = strings.TrimSpace(errType)
		if errType == "" {
			continue
		}
		if targetID, ok := nodeIndex[errType]; ok {
			edges = append(edges, graph.Edge{Source: srcID, Target: targetID, Kind: graph.EdgeProducesError, Label: errType})
		} else {
			// Create a ProducesError edge even if the error type isn't a node — use the source as both.
			// This allows ErrorProducers query to find functions that produce errors by label matching.
			edges = append(edges, graph.Edge{Source: srcID, Target: srcID, Kind: graph.EdgeProducesError, Label: errType})
		}
	}
	return edges
}

// extractStepFunctions extracts function names referenced in @steps.
func extractStepFunctions(f parser.Field) []string {
	var funcs []string
	lines := append([]string{f.InlineValue}, f.Lines...)
	// Look for capitalized words that might be function references.
	funcRe := regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]*(?:\.[A-Z][A-Za-z0-9_]*)*)\b`)
	for _, line := range lines {
		for _, match := range funcRe.FindAllStringSubmatch(line, -1) {
			name := match[1]
			// Skip common non-function words.
			if name == "AID" || name == "None" || name == "True" || name == "False" || name == "If" || name == "For" || name == "Each" {
				continue
			}
			funcs = append(funcs, name)
		}
	}
	return funcs
}
