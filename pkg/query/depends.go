package query

import (
	"fmt"

	"github.com/dan-strohschein/cartograph/pkg/graph"
)

// TypeDependents finds every function, field, and method that references a given type.
func (qe *QueryEngine) TypeDependents(typeName string) (*QueryResult, error) {
	typeNode, err := qe.resolveNode(typeName, graph.KindType, graph.KindTrait)
	if err != nil {
		return nil, err
	}

	var allPaths []TraversalPath

	// Find all edges where the target is this type.
	for _, e := range qe.g.InEdges(typeNode.ID) {
		src, err := qe.g.NodeByID(e.Source)
		if err != nil {
			continue
		}
		allPaths = append(allPaths, TraversalPath{
			Nodes: []graph.Node{src, typeNode},
			Edges: []graph.Edge{e},
			Depth: 1,
		})
	}

	// Also find nodes that reference this type by name in their Type field.
	for _, n := range qe.g.AllNodes() {
		if n.Type == typeName || n.Type == typeName+"?" {
			if n.ID == typeNode.ID {
				continue
			}
			allPaths = append(allPaths, TraversalPath{
				Nodes: []graph.Node{n, typeNode},
				Edges: []graph.Edge{{Source: n.ID, Target: typeNode.ID, Kind: graph.EdgeReferences, Label: typeName}},
				Depth: 1,
			})
		}
	}

	uniqueNodes := make(map[graph.NodeID]bool)
	for _, p := range allPaths {
		for _, n := range p.Nodes {
			uniqueNodes[n.ID] = true
		}
	}

	return &QueryResult{
		Query:     fmt.Sprintf("TypeDependents(%s)", typeName),
		Paths:     allPaths,
		Summary:   fmt.Sprintf("Found %d dependent(s) of type %s", len(allPaths), typeName),
		NodeCount: len(uniqueNodes),
		MaxDepth:  1,
	}, nil
}
