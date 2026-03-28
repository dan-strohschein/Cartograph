# BM3: Full Stack — Cartograph+AID vs. No Documentation

**Date:** 2026-03-27
**Codebase:** SyndrDB

| Layer | Files | Lines | Tokens |
|-------|-------|-------|--------|
| Source code | 496 Go files | 214,410 | ~1,800,576 |
| AID documentation | 75 .aid files | 59,389 | ~378,995 |
| Cartograph graph | — | — | 11,015 nodes / 10,552 edges |

## Question

How much does the full AID+Cartograph stack reduce the cost of answering structural code questions, compared to an agent with raw source code and no documentation?

## Methodology

Three measurements per question:

- **Source (no docs):** Grep the Go source tree for the query target. Count matching files, sum their total bytes. This is the minimum an undocumented agent must read.
- **AID (docs only):** Grep the AID files for the same target. Count matching files, sum their total bytes. This is what a documented agent without Cartograph must read.
- **Cartograph (AID + graph):** Run the query, measure output size. This is what an agent with the full stack reads.

**Token estimation:** bytes / 4.

## Results

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

## The Compression Chain

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

## Key Findings

- **450.6x total reduction** — the full stack compresses 7.25M tokens of source reading into 16K tokens of structured answers.
- **AID alone gives 3.6x** — documentation compresses source by nearly 4x, but an agent still reads 23 files and 200K tokens per question on average.
- **Cartograph multiplies AID's value by 125x** — the graph turns 23-file cross-referencing into a single query.
- **Range: 204x to 2,231x** per question — every scenario shows at least 200x improvement.
- **Widest gap on narrow queries** — WALConfig.MaxFileSize achieves 2,231x because the answer is 2 nodes / 34 tokens, but an agent must read 8 source files (75K tokens) to discover that.

## What This Means

Without documentation, an AI agent answering "what depends on Bundle?" must:
1. Grep 496 Go files → 278 match
2. Read 278 files (1.37M tokens)
3. Parse Go syntax to distinguish type usage from string matches
4. Mentally assemble the dependency picture

With AID only, the same agent:
1. Grep 75 AID files → 49 match
2. Read 49 files (360K tokens)
3. Scan @sig and @fields for type references
4. Cross-reference across files manually

With AID + Cartograph:
1. Run `cartograph depends Bundle`
2. Read 2,303 tokens of structured output listing 130 dependents with relationship types

## Limitations

- **No Calls edges in L1 AID:** CallStack and ErrorProducers queries are not benchmarked because SyndrDB's L1 AID files lack @calls and typed @errors data. L2+ AID would enable these and increase Cartograph's value.
- **Grep is a generous lower bound:** Real agents read more than grep-matching files (imports, context, exploration). Actual reduction is likely higher.
- **Single codebase:** Results from SyndrDB (medium-sized Go project, ~290K LOC including tests). Larger codebases would show even higher reduction ratios.

## Reproducibility

```bash
cd /Users/danstrohschein/Documents/CodeProjects/AI/cartograph
go test ./internal/benchmark/ -v -run TestFullStack
```

Requires:
- AID files at `/tmp/syndr-aid/`
- SyndrDB source at `/Users/danstrohschein/Documents/CodeProjects/golang/SyndrDB/src`
