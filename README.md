# Cartograph

A semantic code index that turns [AID](https://github.com/dan-strohschein/AID-Docs) files into a queryable graph of relationships between types, functions, errors, and data flows. Built to help AI coding agents answer structural questions about codebases using a fraction of the tokens.

## How It Works

Cartograph parses AID documentation files, builds an in-memory graph of code entities (types, functions, fields, methods, constants) and their relationships (calls, returns, accepts, produces-error, has-field, has-method, extends, implements), then answers structural queries against that graph.

```
Source code (290K lines) → AID files (59K lines) → Cartograph graph (11K nodes) → Query answer (30-5000 tokens)
```

## Quick Start

```bash
go build -o cartograph ./cmd/cartograph

# Point at a directory of .aid files
cartograph stats --dir /path/to/.aidocs/

# What depends on this type?
cartograph depends Bundle --dir /path/to/.aidocs/

# What touches this field?
cartograph field Bundle.Name --dir /path/to/.aidocs/

# What produces this error?
cartograph errors ConnectionError --dir /path/to/.aidocs/

# What's the call stack?
cartograph callstack HttpClient.Get --up --dir /path/to/.aidocs/

# What are the side effects?
cartograph effects SaveDocument --dir /path/to/.aidocs/
```

Auto-discovers `.aidocs/` by walking up from the current directory if `--dir` is not specified.

## Graph Cache

On first run, cartograph creates a `cartograph.cache` file alongside the AID files. Subsequent runs load the cached graph instead of re-parsing. The cache auto-invalidates when any AID file changes (tracked via file size + mtime fingerprint). Use `--no-cache` to bypass.

## Commands

| Command | Question it answers |
|---------|-------------------|
| `cartograph depends <Type>` | What depends on this type? |
| `cartograph field <Type.Field>` | What touches this field? |
| `cartograph errors <ErrorType>` | What produces this error? |
| `cartograph callstack <fn> [--up\|--down]` | What's the call stack? |
| `cartograph effects <fn>` | What are the side effects? |
| `cartograph search <pattern> [--kind <kind>]` | Find nodes by name (glob/regex) |
| `cartograph list <module>` | List all nodes in a module |
| `cartograph stats` | Graph statistics |

**Flags:** `--dir <path>`, `--format tree|json`, `--depth 1-50`, `--no-cache`

## Architecture

```
cmd/cartograph/       CLI entry point
pkg/
  graph/              Core data structure — nodes, edges, indexes, gob cache
  loader/             AID parser → graph construction (parallel, cached)
  query/              Query engine — BFS traversal + 5 typed queries + search
  output/             Tree and JSON renderers
internal/
  benchmark/          Token-reduction value benchmarks
```

Depends on [`aidkit/pkg/parser`](https://github.com/dan-strohschein/aidkit) for AID file parsing and [`aidkit/pkg/discovery`](https://github.com/dan-strohschein/aidkit) for `.aidocs/` auto-discovery.

---

## Benchmarks

All benchmarks run against SyndrDB, a medium-sized Go project (496 source files, 214K lines, 75 AID files). Cartograph builds a graph of 11,015 nodes and 10,552 edges from the AID files.

10 structural questions were tested across three query types: type dependents, field touchers, and side effects.

### BM1: Cartograph vs. Raw Source Code (No Documentation)

An agent with no documentation must grep and read raw Go source files.

```
Scenario                                 | CG tok | Src file |  Src tok | Reduction
---------------------------------------------------------------------------------------
depends-Bundle                           |   2303 |      278 |  1375690 |    597.3x
depends-Document                         |   4018 |      261 |  1339414 |    333.4x
depends-BundleFieldSchema                |   1401 |       55 |   417622 |    298.1x
depends-FieldValue                       |    415 |       98 |   644509 |   1553.0x
depends-DocumentCommand                  |    239 |       18 |   123570 |    517.0x
field-Bundle.Name                        |   2490 |      296 |  1428243 |    573.6x
field-Document.DocumentID                |   4736 |      147 |   973618 |    205.6x
field-FieldValue.Type                    |    440 |      158 |   872209 |   1982.3x
field-WALConfig.MaxFileSize              |     34 |        8 |    75841 |   2230.6x
effects-AsyncWALAdapter.WriteEntries     |     30 |        4 |     6130 |    204.3x
---------------------------------------------------------------------------------------
AGGREGATE                                |  16106 |      132 |  7256846 |    450.6x
```

**450.6x aggregate token reduction.** An agent would need 7.25M tokens reading source. Cartograph answers in 16K tokens.

### BM2: Cartograph vs. Raw AID Files

An agent has AID documentation but no Cartograph — it must grep and read AID files manually.

```
Scenario                                 | CG tok | AID file |  AID tok | Reduction
---------------------------------------------------------------------------------------
depends-Bundle                           |   2303 |       49 |   359514 |    156.1x
depends-Document                         |   4018 |       45 |   346059 |     86.1x
depends-BundleFieldSchema                |   1401 |       15 |   195122 |    139.3x
depends-FieldValue                       |    415 |       19 |   193831 |    467.1x
depends-DocumentCommand                  |    239 |        6 |    58644 |    245.4x
field-Bundle.Name                        |   2489 |       49 |   359514 |    144.4x
field-Document.DocumentID                |   4736 |       21 |   268315 |     56.7x
field-FieldValue.Type                    |    440 |       19 |   193831 |    440.5x
field-WALConfig.MaxFileSize              |     34 |        3 |    22088 |    649.6x
effects-AsyncWALAdapter.WriteEntries     |     30 |        1 |    10659 |    355.3x
---------------------------------------------------------------------------------------
AGGREGATE                                |  16105 |       23 |  2007577 |    124.7x
```

**124.7x reduction on top of AID.** Even with structured documentation, cross-cutting queries still require reading 23 files on average. Cartograph collapses that to one query.

### BM3: Full Stack — All Three Layers

Side-by-side comparison showing the compression chain from source to AID to Cartograph.

```
Scenario                            |  Src tok files |  AID tok file | CG tok  nod |    Src→CG
---------------------------------------------------------------------------------------------------------
depends-Bundle                      |  1375690   278 |   359514   49 |   2303  130 |    597.3x
depends-Document                    |  1339414   261 |   346059   45 |   4018  236 |    333.4x
depends-BundleFieldSchema           |   417622    55 |   195122   15 |   1401   81 |    298.1x
depends-FieldValue                  |   644509    98 |   193831   19 |    415   28 |   1553.0x
depends-DocumentCommand             |   123570    18 |    58644    6 |    239   13 |    517.0x
field-Bundle.Name                   |  1428243   296 |   359514   49 |   2490  131 |    573.6x
field-Document.DocumentID           |   973618   147 |   268315   21 |   4736  236 |    205.6x
field-FieldValue.Type               |   872209   158 |   193831   19 |    440   27 |   1982.3x
field-WALConfig.MaxFileSize         |    75841     8 |    22088    3 |     34    2 |   2230.6x
effects-AsyncWALAdapter.WriteEntries |     6130     4 |    10659    1 |     30    2 |    204.3x
---------------------------------------------------------------------------------------------------------
TOTAL                               |  7256846       |  2007577      |  16106      |    450.6x
```

### The Compression Chain

```
Agent with nothing        : 7,256,846 tokens across 10 questions
Agent with AID only       : 2,007,577 tokens (3.6x less than source)
Agent with AID+Cartograph :    16,106 tokens (450.6x less than source)
```

| Transition | Reduction |
|------------|-----------|
| Source → AID | 3.6x |
| AID → Cartograph | 124.7x |
| Source → Cartograph | **450.6x** |

AID makes the data readable. Cartograph makes it queryable.

### Performance Benchmarks

Tested on two real codebases (Apple M3 Pro):

| Metric | Proofgo (73 AID files) | SyndrDB (70 AID files) |
|--------|----------------------|----------------------|
| Nodes / Edges | 1,781 / 2,397 | 10,971 / 20,641 |
| Parse load | 13.4ms | 144ms |
| **Cached load** | **2.3ms** | **15.5ms** |
| Memory footprint | 2.0 MB | 14.8 MB |

**Head-to-head vs vanilla source reading (SyndrDB, 727 .go files, 9.1 MB):**

| Task | Vanilla (grep src) | Cartograph | Speedup |
|------|-------------------|------------|---------|
| Type dependents | 118ms | 412μs | 288x |
| Error producers | 124ms | 16μs | 7,840x |
| Callers of function | 118ms | 15μs | 7,953x |
| Field touchers | 119ms | 9μs | 12,760x |
| **Total (6 queries)** | **721ms** | **12.2ms** | **59x** |

Cartograph cached start (cache load + 6 queries): **27.9ms** vs vanilla 721ms.

### Limitations

- **L1 AID only:** SyndrDB's AID files are L1 mechanical extraction — no `@calls` or typed `@errors` data. CallStack and ErrorProducers queries would improve significantly with L2+ AID.
- **Grep is a generous lower bound:** Real agents read more than the minimum grep-matching set. Actual reduction is likely higher than reported.
- **Single codebase:** Benchmarked on SyndrDB (~290K LOC). Larger codebases would likely show even higher reduction ratios.

### Reproducibility

```bash
go test ./internal/benchmark/ -v -run TestCartographValue    # BM1: Token reduction vs source
go test ./internal/benchmark/ -v -run TestCartographVsAID    # BM2: Token reduction vs AID
go test ./internal/benchmark/ -v -run TestFullStack          # BM3: Full compression chain
go test ./pkg/loader/ -v -run TestHeadToHead                 # Speed: Vanilla vs Cartograph
go test ./pkg/loader/ -bench=. -benchmem                     # Load benchmarks
go test ./pkg/query/ -bench=. -benchmem                      # Query benchmarks
```

Full benchmark details: [BM1.md](BM1.md) | [BM2.md](BM2.md) | [BM3.md](BM3.md)

---

## Dependencies

- [aidkit/pkg/parser](https://github.com/dan-strohschein/aidkit) — AID file parser (referenced via Go module replace directive for local development)

## License

MIT
