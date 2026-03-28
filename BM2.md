# BM2: Cartograph vs. Raw AID Files

**Date:** 2026-03-27
**Dataset:** SyndrDB — 75 AID files, 59,389 lines, ~379K tokens
**Graph:** 11,015 nodes, 10,552 edges, 72 modules

## Question

Does Cartograph add value on top of AID files? An agent already has structured documentation — does a queryable semantic graph still help?

## Methodology

**Control (agent reading raw AID files):** For each question, grep the 75 AID files for the query target. Count matching files and sum their sizes. This is the minimum read cost — AID files are already more structured than source code, but the agent still has to open, read, and mentally cross-reference multiple files.

**Experiment (agent with Cartograph):** Run the Cartograph query, measure output token count.

**Token estimation:** bytes / 4.

## Results

```
Scenario                                 | CG tok | CG nod | AID file |  AID tok | Reduction
-----------------------------------------------------------------------------------------------
depends-Bundle                           |   2303 |    130 |       49 |   359514 |    156.1x
depends-Document                         |   4018 |    236 |       45 |   346059 |     86.1x
depends-BundleFieldSchema                |   1401 |     81 |       15 |   195122 |    139.3x
depends-FieldValue                       |    415 |     28 |       19 |   193831 |    467.1x
depends-DocumentCommand                  |    239 |     13 |        6 |    58644 |    245.4x
field-Bundle.Name                        |   2489 |    131 |       49 |   359514 |    144.4x
field-Document.DocumentID                |   4736 |    236 |       21 |   268315 |     56.7x
field-FieldValue.Type                    |    440 |     27 |       19 |   193831 |    440.5x
field-WALConfig.MaxFileSize              |     34 |      2 |        3 |    22088 |    649.6x
effects-AsyncWALAdapter.WriteEntries     |     30 |      2 |        1 |    10659 |    355.3x
-----------------------------------------------------------------------------------------------
AGGREGATE                                |  16105 |      — |       23 |  2007577 |    124.7x
```

## Key Findings

- **124.7x aggregate token reduction** — even compared to AID files (which are already 3.6x more compact than source code), Cartograph delivers another 125x reduction.
- **23 AID files per question** — an agent would need to open and read 23 AID files on average to answer each structural question.
- **Range: 57x to 650x** — reduction varies by query. Narrow queries save more because the agent has to read entire AID files even when only one entry matters.
- **AID files are necessary but not sufficient for cross-cutting queries** — to answer "what depends on Bundle?", an agent must read 49 of the 75 AID files (359K tokens). Cartograph answers in 2,303 tokens.

## Comparison with BM1

| Metric | BM1 (vs. source) | BM2 (vs. AID) |
|--------|-------------------|----------------|
| Control corpus | 496 Go files, 214K lines | 75 AID files, 59K lines |
| Control tokens per question | ~726K avg | ~201K avg |
| Cartograph tokens per question | ~1,611 avg | ~1,611 avg |
| Aggregate reduction | 450.6x | 124.7x |
| Files per question | 132 avg | 23 avg |

AID files provide a **3.6x reduction** over source code (450.6 / 124.7). Cartograph provides an additional **124.7x** on top of that. The combined stack (AID + Cartograph) delivers **450.6x** total reduction vs. raw source.

## What This Means

AID files compress a codebase from 214K lines to 59K lines — a significant improvement. But for cross-cutting structural questions, an agent still needs to read and cross-reference dozens of files. Cartograph collapses that to a single query with a targeted answer.

The value compounds: AID makes the data readable, Cartograph makes it queryable.

## Limitations

- **Same as BM1:** No Calls edges (L1 AID), no accuracy scoring, single codebase.
- **Grep is generous for AID:** An agent reading AID files would actually be more efficient than reading source code (AID is structured and scannable). The grep simulation counts entire file sizes, but a skilled agent might skip irrelevant sections. Even so, the reduction is substantial.

## Reproducibility

```bash
cd /Users/danstrohschein/Documents/CodeProjects/AI/cartograph
go test ./internal/benchmark/ -v -run TestCartographVsAID
go test ./internal/benchmark/ -v -run TestAIDBaseline
```

Requires:
- AID files at `/tmp/syndr-aid/`
