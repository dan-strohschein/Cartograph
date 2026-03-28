package query

import (
	"fmt"
	"strings"

	"github.com/dan-strohschein/cartograph/internal/graph"
)

// ErrorProducers finds every function that can return a given error type,
// including transitive propagation chains.
func (qe *QueryEngine) ErrorProducers(errorType string) (*QueryResult, error) {
	// Find all edges with ProducesError that match the error type.
	var producers []graph.Node
	for _, e := range qe.g.AllEdges() {
		if e.Kind == graph.EdgeProducesError && matchesName(e.Label, errorType) {
			n, err := qe.g.NodeByID(e.Source)
			if err == nil {
				producers = append(producers, n)
			}
		}
	}

	if len(producers) == 0 {
		return nil, &NotFoundError{Entity: errorType, Kind: "error"}
	}

	// For each producer, reverse-traverse Calls edges to find callers.
	var allPaths []TraversalPath
	seen := make(map[graph.NodeID]bool)
	for _, producer := range producers {
		if seen[producer.ID] {
			continue
		}
		seen[producer.ID] = true

		paths := qe.Traverse(producer.ID, []graph.EdgeKind{graph.EdgeCalls, graph.EdgePropagatesError}, Reverse, qe.maxDepth)
		if len(paths) == 0 {
			// The producer itself is a result even with no callers.
			allPaths = append(allPaths, TraversalPath{
				Nodes: []graph.Node{producer},
				Edges: nil,
				Depth: 0,
			})
		} else {
			allPaths = append(allPaths, paths...)
		}
	}

	// Count unique nodes.
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
		Query:     fmt.Sprintf("ErrorProducers(%s)", errorType),
		Paths:     allPaths,
		Summary:   fmt.Sprintf("Found %d producer(s) of %s across %d path(s)", len(producers), errorType, len(allPaths)),
		NodeCount: len(uniqueNodes),
		MaxDepth:  maxDepth,
	}, nil
}

// matchesName checks if a label matches the target error type (case-insensitive, supports partial match).
func matchesName(label, target string) bool {
	return strings.EqualFold(label, target) || strings.EqualFold(label, target+"Error") || strings.HasSuffix(strings.ToLower(label), strings.ToLower(target))
}
