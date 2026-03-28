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

- **graph** — Core data structure. Nodes (code entities) + Edges (relationships) + indexes for fast query.
- **query** — Query engine. 5 core queries: ErrorProducers, FieldTouchers, CallStack, TypeDependents, SideEffects.
- **loader** — Reads AID files, extracts nodes and edges, builds the graph.
- **cli** — CLI interface. `cartograph errors|field|callstack|depends|effects`.

## Build

```bash
go build -o cartograph ./cmd/cartograph
```

## Dependencies

- `github.com/dan-strohschein/aidkit/pkg/parser` — AID file parser
