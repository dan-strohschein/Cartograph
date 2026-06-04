# Cartograph

A semantic code index that turns AID files into a queryable graph of relationships between types, functions, errors, and data flows.

## AID Documentation

This project uses AID skeleton files in `.aidocs/` as the design spec.

- **Read `.aidocs/manifest.aid` first** to see all packages and their `@key_risks`
- **Implement code to match the AID contracts** — signatures, types, and workflows are the spec
- After implementing a package, run `aid-gen-go` to replace skeleton with extracted L1 data
- Check `@antipatterns` before making architectural decisions
- Check `@decision` blocks to understand WHY things are designed a certain way

## Architecture

4 packages:

- **graph** — Core data structure. Nodes (code entities) + Edges (relationships) + indexes for fast query. Includes Tier 4 kinds (Graph, GraphNode, Tool, Agent, Prompt, Model, Frame) and agentic edges (FlowsTo, ConditionalFlow, UsesTool, UsesModel, HandsOffTo, etc.).
- **query** — Query engine. Core queries: ErrorProducers, FieldTouchers, CallStack, TypeDependents, SideEffects. Tier 4 queries: GraphTopology, AgentInfo, ListTools.
- **loader** — Reads AID files, extracts nodes and edges, builds the graph. Tier 4 extraction (graph topology, state channels/reducers, tools/agents/prompts/models) lives in `tier4.go`.
- **cli** — CLI interface. `cartograph errors|field|callstack|depends|effects|search|list|stats` plus Tier 4 `graph|agent|tools`.

## AID Tiers

Cartograph models all four AID spec tiers (see `AID/spec/format.md`): Tier 1/2 entries (`@fn`/`@type`/`@trait`/`@const`), Tier 2.5 annotations (`@lock` etc.), Tier 3 workflows (`@workflow`, incl. `@errors_at`/`@variants`), and Tier 4 agentic/dataflow constructs (`@graph`/`@tool`/`@agent`/`@prompt`/`@model`, state channels, `@engine`/`@checkpointer`). Tier 4 parsing depends on aidkit recognizing those entry keywords — it requires aidkit `v0.3.0` or later.

## Build

```bash
go build -o cartograph ./cmd/cartograph
```

## Dependencies

- `github.com/dan-strohschein/aidkit/pkg/parser` — AID file parser

## Testing

- Default: `go test ./...` — hermetic tests (including `internal/loader/testdata` AID fixtures).
- Integration (aid-gen-go + external trees such as chisel): set `CARTOGRAPH_AID_GEN_GO` to the `aid-gen-go` main package directory, and optionally `CARTOGRAPH_CHISEL_RESOLVE` / `CARTOGRAPH_CHISEL_EDIT` to those package roots, then run `go test -tags=integration ./internal/loader -v`. Tests skip if variables are unset or paths are missing.
