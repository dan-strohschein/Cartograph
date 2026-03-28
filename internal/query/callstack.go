package query

import (
	"fmt"

	"github.com/dan-strohschein/cartograph/internal/graph"
)

// CallStack traces all callers (upward) and/or callees (downward) of a function.
func (qe *QueryEngine) CallStack(functionName string, direction TraversalDirection) (*QueryResult, error) {
	fn, err := qe.resolveFunction(functionName)
	if err != nil {
		return nil, err
	}

	paths := qe.Traverse(fn.ID, []graph.EdgeKind{graph.EdgeCalls}, direction, qe.maxDepth)

	uniqueNodes := make(map[graph.NodeID]bool)
	maxDepth := 0
	for _, p := range paths {
		for _, n := range p.Nodes {
			uniqueNodes[n.ID] = true
		}
		if p.Depth > maxDepth {
			maxDepth = p.Depth
		}
	}

	dirStr := "callers and callees"
	if direction == Forward {
		dirStr = "callees"
	} else if direction == Reverse {
		dirStr = "callers"
	}

	return &QueryResult{
		Query:     fmt.Sprintf("CallStack(%s, %s)", functionName, dirStr),
		Paths:     paths,
		Summary:   fmt.Sprintf("Found %d %s of %s across %d path(s)", len(uniqueNodes)-1, dirStr, functionName, len(paths)),
		NodeCount: len(uniqueNodes),
		MaxDepth:  maxDepth,
	}, nil
}
