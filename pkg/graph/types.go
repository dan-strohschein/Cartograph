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

	// Tier 4: agentic and dataflow constructs.
	KindGraph     NodeKind = "Graph"     // @graph — a dataflow topology
	KindGraphNode NodeKind = "GraphNode" // a single node within a @graph's @nodes
	KindTool      NodeKind = "Tool"      // @tool — a model-invocable function
	KindAgent     NodeKind = "Agent"     // @agent — an autonomous actor
	KindPrompt    NodeKind = "Prompt"    // @prompt — a templated model invocation
	KindModel     NodeKind = "Model"     // @model — a configured language model
	KindFrame     NodeKind = "Frame"     // a frame type in a push-based (Pipecat) graph
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

	// Tier 4: agentic and dataflow relationships.
	EdgeContainsNode    EdgeKind = "ContainsNode"    // Graph → GraphNode it owns
	EdgeEntryNode       EdgeKind = "EntryNode"       // Graph → its entry GraphNode(s)
	EdgeFlowsTo         EdgeKind = "FlowsTo"         // GraphNode → GraphNode (unconditional @edges)
	EdgeConditionalFlow EdgeKind = "ConditionalFlow" // GraphNode → GraphNode (routed @conditional_edges)
	EdgeImplementedBy   EdgeKind = "ImplementedBy"   // GraphNode → fn/tool/agent that implements it
	EdgeRoutedBy        EdgeKind = "RoutedBy"        // GraphNode → router fn for conditional routing
	EdgeUsesState       EdgeKind = "UsesState"       // Graph → its @state type
	EdgeUsesTool        EdgeKind = "UsesTool"        // Agent → Tool it may call
	EdgeUsesModel       EdgeKind = "UsesModel"       // Agent/Prompt → Model it targets
	EdgeHandsOffTo      EdgeKind = "HandsOffTo"      // Agent → Agent it can transfer control to
	EdgeProducesOutput  EdgeKind = "ProducesOutput"  // Agent/Prompt/Model → output/structured type
	EdgeCarriesFrame    EdgeKind = "CarriesFrame"    // GraphNode → Frame flowing along an edge
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
