package graph

import (
	"crypto/sha256"
	"fmt"
)

// NodeID is a unique identifier for a node, derived from module + kind + name.
type NodeID string

// MakeNodeID creates a deterministic NodeID from module, kind, and name.
func MakeNodeID(module string, kind NodeKind, name string) NodeID {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", module, kind, name)))
	return NodeID(fmt.Sprintf("%x", h[:8]))
}

// NodeKind classifies code entities in the graph.
type NodeKind string

const (
	KindFunction NodeKind = "Function"
	KindMethod   NodeKind = "Method"
	KindType     NodeKind = "Type"
	KindTrait    NodeKind = "Trait"
	KindField    NodeKind = "Field"
	KindConstant NodeKind = "Constant"
	KindModule   NodeKind = "Module"
	KindWorkflow NodeKind = "Workflow"
	KindLock     NodeKind = "Lock"
)

// EdgeKind classifies relationships between code entities.
type EdgeKind string

const (
	EdgeCalls           EdgeKind = "Calls"
	EdgeReturns         EdgeKind = "Returns"
	EdgeAccepts         EdgeKind = "Accepts"
	EdgeProducesError   EdgeKind = "ProducesError"
	EdgePropagatesError EdgeKind = "PropagatesError"
	EdgeHasField        EdgeKind = "HasField"
	EdgeHasMethod       EdgeKind = "HasMethod"
	EdgeImplements      EdgeKind = "Implements"
	EdgeExtends         EdgeKind = "Extends"
	EdgeReferences      EdgeKind = "References"
	EdgeReadsField      EdgeKind = "ReadsField"
	EdgeWritesField     EdgeKind = "WritesField"
	EdgeDependsOn       EdgeKind = "DependsOn"
	EdgeStepOf          EdgeKind = "StepOf"
	EdgeAcquires        EdgeKind = "Acquires"
	EdgeOrderedBefore   EdgeKind = "OrderedBefore"
)

// Node represents a single code entity: function, type, field, method, constant, or module.
type Node struct {
	ID            NodeID
	Kind          NodeKind
	Name          string
	QualifiedName string
	Module        string
	Type          string
	Signature     string
	Purpose       string
	SourceFile    string
	SourceLine    int
	Metadata      map[string]string
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	Source NodeID
	Target NodeID
	Kind   EdgeKind
	Label  string
	Weight float64
}

// GraphStats contains summary statistics for a loaded graph.
type GraphStats struct {
	NodeCount   int
	EdgeCount   int
	NodesByKind map[string]int
	EdgesByKind map[string]int
	Modules     int
}
