package graph

import "fmt"

// Graph is the central data structure holding all nodes, edges, and indexes.
type Graph struct {
	nodes    map[NodeID]Node
	edges    []Edge
	byKind   map[NodeKind][]NodeID
	byName   map[string][]NodeID
	byType   map[string][]NodeID
	outEdges map[NodeID][]Edge
	inEdges  map[NodeID][]Edge
}

// NewGraph creates an empty graph with initialized indexes.
func NewGraph() *Graph {
	return &Graph{
		nodes:    make(map[NodeID]Node),
		edges:    nil,
		byKind:   make(map[NodeKind][]NodeID),
		byName:   make(map[string][]NodeID),
		byType:   make(map[string][]NodeID),
		outEdges: make(map[NodeID][]Edge),
		inEdges:  make(map[NodeID][]Edge),
	}
}

// AddNode adds a node to the graph and updates all indexes.
func (g *Graph) AddNode(node Node) {
	if _, exists := g.nodes[node.ID]; exists {
		return
	}
	g.nodes[node.ID] = node
	g.byKind[node.Kind] = append(g.byKind[node.Kind], node.ID)
	g.byName[node.Name] = append(g.byName[node.Name], node.ID)
	if node.Type != "" {
		g.byType[node.Type] = append(g.byType[node.Type], node.ID)
	}
}

// AddEdge adds an edge to the graph and updates forward/reverse indexes.
func (g *Graph) AddEdge(edge Edge) {
	if _, ok := g.nodes[edge.Source]; !ok {
		return
	}
	if _, ok := g.nodes[edge.Target]; !ok {
		return
	}
	if edge.Weight == 0 {
		edge.Weight = 1.0
	}
	g.edges = append(g.edges, edge)
	g.outEdges[edge.Source] = append(g.outEdges[edge.Source], edge)
	g.inEdges[edge.Target] = append(g.inEdges[edge.Target], edge)
}

// NodeByID looks up a node by its unique ID.
func (g *Graph) NodeByID(id NodeID) (Node, error) {
	node, ok := g.nodes[id]
	if !ok {
		return Node{}, fmt.Errorf("node not found: %s", id)
	}
	return node, nil
}

// NodesByKind returns all nodes of a specific kind.
func (g *Graph) NodesByKind(kind NodeKind) []Node {
	ids := g.byKind[kind]
	nodes := make([]Node, 0, len(ids))
	for _, id := range ids {
		nodes = append(nodes, g.nodes[id])
	}
	return nodes
}

// NodesByName returns all nodes matching a name.
func (g *Graph) NodesByName(name string) []Node {
	ids := g.byName[name]
	nodes := make([]Node, 0, len(ids))
	for _, id := range ids {
		nodes = append(nodes, g.nodes[id])
	}
	return nodes
}

// OutEdges returns all outgoing edges from a node.
func (g *Graph) OutEdges(id NodeID) []Edge {
	return g.outEdges[id]
}

// InEdges returns all incoming edges to a node.
func (g *Graph) InEdges(id NodeID) []Edge {
	return g.inEdges[id]
}

// AllNodes returns all nodes in the graph.
func (g *Graph) AllNodes() []Node {
	nodes := make([]Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// AllEdges returns all edges in the graph.
func (g *Graph) AllEdges() []Edge {
	return g.edges
}

// Stats returns summary statistics about the graph.
func (g *Graph) Stats() GraphStats {
	nk := make(map[string]int)
	for kind, ids := range g.byKind {
		nk[string(kind)] = len(ids)
	}
	ek := make(map[string]int)
	for _, e := range g.edges {
		ek[string(e.Kind)]++
	}
	modules := 0
	seen := make(map[string]bool)
	for _, n := range g.nodes {
		if !seen[n.Module] {
			seen[n.Module] = true
			modules++
		}
	}
	return GraphStats{
		NodeCount:   len(g.nodes),
		EdgeCount:   len(g.edges),
		NodesByKind: nk,
		EdgesByKind: ek,
		Modules:     modules,
	}
}
