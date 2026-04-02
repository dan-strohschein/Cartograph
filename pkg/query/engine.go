package query

import (
	"fmt"
	"strings"

	"github.com/dan-strohschein/cartograph/pkg/graph"
)

// TraversalDirection controls which edges to follow.
type TraversalDirection int

const (
	Forward TraversalDirection = iota // Follow outgoing edges.
	Reverse                          // Follow incoming edges.
	Both                             // Follow edges in both directions.
)

// TraversalPath is a single path through the graph found by Traverse.
type TraversalPath struct {
	Nodes []graph.Node
	Edges []graph.Edge
	Depth int
}

// QueryResult is the result of a high-level query.
type QueryResult struct {
	Query     string
	Paths     []TraversalPath
	Summary   string
	NodeCount int
	MaxDepth  int
}

// EffectReport is the result of a SideEffects query.
type EffectReport struct {
	Function     string
	Effects      map[string][]graph.Node
	TotalCallees int
	MaxDepth     int
}

// NotFoundError is returned when a query target doesn't exist.
type NotFoundError struct {
	Entity      string
	Kind        string
	Suggestions []string
}

func (e *NotFoundError) Error() string {
	msg := fmt.Sprintf("%s not found: %s", e.Kind, e.Entity)
	if len(e.Suggestions) > 0 {
		msg += fmt.Sprintf(". Did you mean: %s?", strings.Join(e.Suggestions, ", "))
	}
	return msg
}

// AmbiguousError is returned when a name matches multiple entities.
type AmbiguousError struct {
	Name       string
	Candidates []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("ambiguous name %q matches: %v", e.Name, e.Candidates)
}

// QueryEngine executes queries against a Graph.
type QueryEngine struct {
	g        *graph.Graph
	maxDepth int
}

// NewQueryEngine creates a query engine for the given graph.
func NewQueryEngine(g *graph.Graph, maxDepth int) *QueryEngine {
	if maxDepth <= 0 {
		maxDepth = 10
	}
	if maxDepth > 50 {
		maxDepth = 50
	}
	return &QueryEngine{g: g, maxDepth: maxDepth}
}

// maxTraversalPaths caps the number of paths returned to prevent combinatorial explosion.
const maxTraversalPaths = 1000

// Traverse walks edges from a starting node, filtering by edge kind and direction.
func (qe *QueryEngine) Traverse(startID graph.NodeID, edgeKinds []graph.EdgeKind, direction TraversalDirection, maxDepth int) []TraversalPath {
	if maxDepth <= 0 {
		maxDepth = qe.maxDepth
	}
	if maxDepth > 50 {
		maxDepth = 50
	}

	startNode, err := qe.g.NodeByID(startID)
	if err != nil {
		return nil
	}

	kindFilter := make(map[graph.EdgeKind]bool)
	for _, k := range edgeKinds {
		kindFilter[k] = true
	}
	filterAll := len(edgeKinds) == 0

	var results []TraversalPath

	type frame struct {
		nodes   []graph.Node
		edges   []graph.Edge
		visited map[graph.NodeID]bool
	}

	initial := frame{
		nodes:   []graph.Node{startNode},
		edges:   nil,
		visited: map[graph.NodeID]bool{startID: true},
	}

	queue := []frame{initial}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if len(current.edges) >= maxDepth {
			results = append(results, TraversalPath{
				Nodes: current.nodes,
				Edges: current.edges,
				Depth: len(current.edges),
			})
			if len(results) >= maxTraversalPaths {
				return results
			}
			continue
		}

		lastNode := current.nodes[len(current.nodes)-1]
		var neighbors []graph.Edge

		if direction == Forward || direction == Both {
			for _, e := range qe.g.OutEdges(lastNode.ID) {
				if filterAll || kindFilter[e.Kind] {
					neighbors = append(neighbors, e)
				}
			}
		}
		if direction == Reverse || direction == Both {
			for _, e := range qe.g.InEdges(lastNode.ID) {
				if filterAll || kindFilter[e.Kind] {
					neighbors = append(neighbors, e)
				}
			}
		}

		if len(neighbors) == 0 {
			if len(current.edges) > 0 {
				results = append(results, TraversalPath{
					Nodes: current.nodes,
					Edges: current.edges,
					Depth: len(current.edges),
				})
				if len(results) >= maxTraversalPaths {
					return results
				}
			}
			continue
		}

		expanded := false
		for _, edge := range neighbors {
			// Determine the next node: for outEdges it's Target, for inEdges it's Source.
			nextID := edge.Target
			if edge.Target == lastNode.ID {
				nextID = edge.Source
			}

			if current.visited[nextID] {
				continue
			}

			nextNode, err := qe.g.NodeByID(nextID)
			if err != nil {
				continue
			}

			expanded = true

			newVisited := make(map[graph.NodeID]bool, len(current.visited)+1)
			for k, v := range current.visited {
				newVisited[k] = v
			}
			newVisited[nextID] = true

			newNodes := make([]graph.Node, len(current.nodes)+1, maxDepth+1)
			copy(newNodes, current.nodes)
			newNodes[len(current.nodes)] = nextNode

			newEdges := make([]graph.Edge, len(current.edges)+1, maxDepth)
			copy(newEdges, current.edges)
			newEdges[len(current.edges)] = edge

			queue = append(queue, frame{
				nodes:   newNodes,
				edges:   newEdges,
				visited: newVisited,
			})
		}
		// If we had neighbors but none could be visited (cycle), record this path.
		if !expanded && len(current.edges) > 0 {
			results = append(results, TraversalPath{
				Nodes: current.nodes,
				Edges: current.edges,
				Depth: len(current.edges),
			})
			if len(results) >= maxTraversalPaths {
				return results
			}
		}
	}

	return results
}

// resolveFunction finds a function/method node by name. Returns error if not found or ambiguous.
func (qe *QueryEngine) resolveFunction(name string) (graph.Node, error) {
	nodes := qe.g.NodesByName(name)
	// Filter to functions and methods.
	var matches []graph.Node
	for _, n := range nodes {
		if n.Kind == graph.KindFunction || n.Kind == graph.KindMethod {
			matches = append(matches, n)
		}
	}
	if len(matches) == 0 {
		// Try by qualified name or partial match.
		for _, n := range qe.g.AllNodes() {
			if (n.Kind == graph.KindFunction || n.Kind == graph.KindMethod) && (n.QualifiedName == name || n.Name == name) {
				matches = append(matches, n)
			}
		}
	}
	if len(matches) == 0 {
		nfe := &NotFoundError{Entity: name, Kind: "function"}
		suggestions := qe.suggestSimilar(name, []graph.NodeKind{graph.KindFunction, graph.KindMethod}, 5)
		if len(suggestions) > 0 {
			nfe.Suggestions = suggestions
		}
		return graph.Node{}, nfe
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	var candidates []string
	for _, m := range matches {
		candidates = append(candidates, m.QualifiedName)
	}
	return graph.Node{}, &AmbiguousError{Name: name, Candidates: candidates}
}

// resolveNode finds any node by name.
func (qe *QueryEngine) resolveNode(name string, kinds ...graph.NodeKind) (graph.Node, error) {
	nodes := qe.g.NodesByName(name)
	kindSet := make(map[graph.NodeKind]bool)
	for _, k := range kinds {
		kindSet[k] = true
	}
	filterKinds := len(kinds) > 0

	var matches []graph.Node
	for _, n := range nodes {
		if !filterKinds || kindSet[n.Kind] {
			matches = append(matches, n)
		}
	}
	if len(matches) == 0 {
		kindStr := "entity"
		if len(kinds) > 0 {
			kindStr = string(kinds[0])
		}
		nfe := &NotFoundError{Entity: name, Kind: kindStr}
		suggestions := qe.suggestSimilar(name, kinds, 5)
		if len(suggestions) > 0 {
			nfe.Suggestions = suggestions
		}
		return graph.Node{}, nfe
	}
	return matches[0], nil
}
