package query

import (
	"fmt"
	"strings"

	"github.com/dan-strohschein/cartograph/pkg/graph"
)

// FieldTouchers finds every function that reads or writes a given field.
func (qe *QueryEngine) FieldTouchers(typeName, fieldName string) (*QueryResult, error) {
	// Find the field node.
	fullFieldName := typeName + "." + fieldName
	fieldNode, err := qe.resolveNode(fullFieldName, graph.KindField)
	if err != nil {
		return nil, &NotFoundError{Entity: fullFieldName, Kind: "field"}
	}

	var allPaths []TraversalPath

	// Check for ReadsField and WritesField edges pointing to this field.
	for _, e := range qe.g.InEdges(fieldNode.ID) {
		if e.Kind == graph.EdgeReadsField || e.Kind == graph.EdgeWritesField {
			src, err := qe.g.NodeByID(e.Source)
			if err == nil {
				allPaths = append(allPaths, TraversalPath{
					Nodes: []graph.Node{src, fieldNode},
					Edges: []graph.Edge{e},
					Depth: 1,
				})
			}
		}
	}

	// Also find functions that accept the parent type (they likely touch its fields).
	typeNode, _ := qe.resolveNode(typeName, graph.KindType)
	if typeNode.ID != "" {
		for _, e := range qe.g.InEdges(typeNode.ID) {
			if e.Kind == graph.EdgeAccepts || e.Kind == graph.EdgeReturns {
				src, err := qe.g.NodeByID(e.Source)
				if err == nil {
					allPaths = append(allPaths, TraversalPath{
						Nodes: []graph.Node{src, fieldNode},
						Edges: []graph.Edge{e},
						Depth: 1,
					})
				}
			}
		}

		// Find methods of the type (they implicitly touch fields).
		for _, e := range qe.g.OutEdges(typeNode.ID) {
			if e.Kind == graph.EdgeHasMethod {
				method, err := qe.g.NodeByID(e.Target)
				if err == nil {
					// Check if the method's signature mentions the field.
					if strings.Contains(strings.ToLower(method.Signature), strings.ToLower(fieldName)) ||
						strings.Contains(strings.ToLower(method.Purpose), strings.ToLower(fieldName)) {
						allPaths = append(allPaths, TraversalPath{
							Nodes: []graph.Node{method, fieldNode},
							Edges: []graph.Edge{{Source: method.ID, Target: fieldNode.ID, Kind: graph.EdgeReadsField, Label: fieldName}},
							Depth: 1,
						})
					}
				}
			}
		}
	}

	uniqueNodes := make(map[graph.NodeID]bool)
	maxDepth := 0
	for _, p := range allPaths {
		for _, n := range p.Nodes {
			uniqueNodes[n.ID] = true
		}
		if p.Depth > maxDepth {
			maxDepth = p.Depth
		}
	}

	return &QueryResult{
		Query:     fmt.Sprintf("FieldTouchers(%s.%s)", typeName, fieldName),
		Paths:     allPaths,
		Summary:   fmt.Sprintf("Found %d function(s) that touch %s.%s", len(allPaths), typeName, fieldName),
		NodeCount: len(uniqueNodes),
		MaxDepth:  maxDepth,
	}, nil
}
