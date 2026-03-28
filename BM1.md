# BM1: Cartograph vs. Raw Source Code (No Documentation)

**Date:** 2026-03-27
**Dataset:** SyndrDB — 496 Go source files, 214,410 lines, ~1.8M tokens
**Graph:** 11,015 nodes, 10,552 edges, 72 modules (from 75 AID files)

## Question

Does Cartograph help an AI agent answer structural code questions more efficiently than an agent working with raw source code and no documentation?

## Methodology

**Control (agent with nothing):** For each question, grep the entire Go source tree for the query target. Count matching files and sum their sizes. This is the *minimum* read cost — a real agent would read even more (imports, context, false hits, call tracing).

**Experiment (agent with Cartograph):** Run the Cartograph query, measure output token count.

**Token estimation:** bytes / 4 (standard approximation for English/code).

## Results

```
Scenario                                 | CG tok | CG nod | Src file |  Src tok | Reduction
-----------------------------------------------------------------------------------------------
depends-Bundle                           |   2303 |    130 |      278 |  1375690 |    597.3x
depends-Document                         |   4018 |    236 |      261 |  1339414 |    333.4x
depends-BundleFieldSchema                |   1401 |     81 |       55 |   417622 |    298.1x
depends-FieldValue                       |    415 |     28 |       98 |   644509 |   1553.0x
depends-DocumentCommand                  |    239 |     13 |       18 |   123570 |    517.0x
field-Bundle.Name                        |   2490 |    131 |      296 |  1428243 |    573.6x
field-Document.DocumentID                |   4736 |    236 |      147 |   973618 |    205.6x
field-FieldValue.Type                    |    440 |     27 |      158 |   872209 |   1982.3x
field-WALConfig.MaxFileSize              |     34 |      2 |        8 |    75841 |   2230.6x
effects-AsyncWALAdapter.WriteEntries     |     30 |      2 |        4 |     6130 |    204.3x
-----------------------------------------------------------------------------------------------
AGGREGATE                                |  16106 |      — |      132 |  7256846 |    450.6x
```

## Key Findings

- **450.6x aggregate token reduction** — Cartograph answers 10 structural questions in 16,106 tokens. An agent reading source code would need 7,256,846 tokens.
- **132 files per question** — on average, an agent would need to open and read 132 Go source files to answer each structural question.
- **Range: 204x to 2,231x** — reduction varies by query specificity. Narrow queries (WALConfig.MaxFileSize) save the most per-token because the agent still has to read entire files to find the one relevant line.
- **Even the cheapest question costs 204x more without Cartograph** — the effects query for AsyncWALAdapter.WriteEntries only touches 4 source files (834 lines), but Cartograph answers in 30 tokens.

## What This Means

An AI agent using Cartograph can answer "what depends on Bundle?" by reading **2,303 tokens** of structured output listing 130 dependent functions with their relationship types.

The same agent without any documentation would need to:
1. Grep 496 source files for "Bundle"
2. Open and read the 278 matching files (158,479 lines / 1,375,690 tokens)
3. Mentally parse Go code to distinguish type references from string matches
4. Assemble the dependency picture from scattered function signatures

That's **597x more context** consumed, assuming perfect grep with zero false exploration — a generous lower bound.

## Limitations

- **No Calls edges:** SyndrDB AID files are L1 mechanical extraction and lack `@calls` data. CallStack and ErrorProducers queries return minimal results. L2+ AID files would enable these queries and increase Cartograph's value further.
- **Grep is generous:** The control measures minimum grep cost. A real agent would read more files (exploring imports, tracing types, reading surrounding context). Actual reduction is likely higher.
- **No accuracy scoring:** This benchmark measures token efficiency only. F1 accuracy scoring requires hand-verified golden answers (future work).
- **Single codebase:** Results are from SyndrDB (a medium-sized Go project). Reduction ratios would differ for smaller or larger codebases.

## Reproducibility

```bash
cd /Users/danstrohschein/Documents/CodeProjects/AI/cartograph
go test ./internal/benchmark/ -v -run TestCartographValue
go test ./internal/benchmark/ -v -run TestSourceTreeBaseline
```

Requires:
- AID files at `/tmp/syndr-aid/`
- SyndrDB source at `/Users/danstrohschein/Documents/CodeProjects/golang/SyndrDB/src`
