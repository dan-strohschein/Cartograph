package query

import (
	"testing"

	"github.com/dan-strohschein/cartograph/pkg/loader"
)

func tier4Engine(t *testing.T) *QueryEngine {
	t.Helper()
	g, err := loader.LoadFromDirectory("../loader/testdata/tier4")
	if err != nil {
		t.Fatalf("load tier4 fixture: %v", err)
	}
	return NewQueryEngine(g, 10)
}

func TestGraphTopologyQuery(t *testing.T) {
	qe := tier4Engine(t)
	v, err := qe.GraphTopology("agent_loop")
	if err != nil {
		t.Fatalf("GraphTopology: %v", err)
	}
	if v.Engine != "langgraph" {
		t.Errorf("engine = %q, want langgraph", v.Engine)
	}
	if v.State != "AgentState" {
		t.Errorf("state = %q, want AgentState", v.State)
	}
	if v.Reducers["messages"] != "append" {
		t.Errorf("messages reducer = %q, want append", v.Reducers["messages"])
	}
	if v.Reducers["next"] != "last-wins" {
		t.Errorf("next reducer = %q, want last-wins", v.Reducers["next"])
	}
	if len(v.Nodes) != 3 {
		t.Errorf("node count = %d, want 3", len(v.Nodes))
	}
	if len(v.EntryNodes) != 1 || v.EntryNodes[0] != "plan" {
		t.Errorf("entry nodes = %v, want [plan]", v.EntryNodes)
	}
	// plan should route via should_continue to act.
	var planCond *CondEdge
	for i := range v.Conditional {
		if v.Conditional[i].From == "plan" {
			planCond = &v.Conditional[i]
		}
	}
	if planCond == nil {
		t.Fatal("no conditional edge from plan")
	}
	if planCond.Router != "should_continue" {
		t.Errorf("router = %q, want should_continue", planCond.Router)
	}
}

func TestAgentInfoQuery(t *testing.T) {
	qe := tier4Engine(t)
	v, err := qe.AgentInfo("researcher")
	if err != nil {
		t.Fatalf("AgentInfo: %v", err)
	}
	if v.Model != "planner_llm" {
		t.Errorf("model = %q", v.Model)
	}
	if len(v.Tools) != 2 {
		t.Errorf("tools = %v, want 2", v.Tools)
	}
	if v.Output != "ResearchReport" {
		t.Errorf("output = %q", v.Output)
	}
	if v.Autonomy != "supervised" || v.Memory != "thread" {
		t.Errorf("autonomy/memory = %q/%q", v.Autonomy, v.Memory)
	}
}

func TestListToolsQuery(t *testing.T) {
	qe := tier4Engine(t)
	tools := qe.ListTools()
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(tools))
	}
	// Sorted by name: fetch_url before web_search.
	if tools[0].Name != "fetch_url" || tools[1].Name != "web_search" {
		t.Errorf("tool order = %s, %s", tools[0].Name, tools[1].Name)
	}
	if tools[1].InvokedBy != "llm" {
		t.Errorf("web_search invoked_by = %q", tools[1].InvokedBy)
	}
}
