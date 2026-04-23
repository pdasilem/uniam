# Uniam Architecture

## Purpose

Uniam is a single-binary Go application for persistent agent memory.

It solves three problems at once:

1. Stores session knowledge as human-readable Markdown notes.
2. Indexes the same knowledge in SQLite for retrieval by keyword and vectors.
3. Exposes the memory system both as a CLI and as an MCP server for coding agents.

The repository is therefore not just a CLI tool. It is a local memory platform with four major surfaces:

- `uniam <command>` CLI
- `uniam mcp` stdio MCP server
- disk-backed Markdown shelves in `~/.uniam/shelves/`
- SQLite index in `~/.uniam/index.db`

## High-Level System

```text
                         +----------------------+
                         |  Agent / User        |
                         |  Claude, Codex, etc. |
                         +----------+-----------+
                                    |
                   +----------------+----------------+
                   |                                 |
                   v                                 v
         +-------------------+             +-------------------+
         | CLI (cobra)       |             | MCP stdio server  |
         | pkg/cli/*         |             | internal/mcp      |
         +---------+---------+             +---------+---------+
                   |                                 |
                   +---------------+-----------------+
                                   |
                                   v
                        +----------------------+
                        | core.Service         |
                        | internal/core        |
                        +----------+-----------+
                                   |
              +--------------------+--------------------+
              |                    |                    |
              v                    v                    v
    +----------------+   +--------------------+   +------------------+
    | Redaction      |   | Markdown shelves   |   | SQLite index     |
    | internal/      |   | internal/storage   |   | internal/db      |
    | redaction      |   |                    |   | + FTS5 + vec0    |
    +----------------+   +--------------------+   +--------+---------+
                                                               |
                                                               v
                                                    +--------------------+
                                                    | Embedding provider |
                                                    | internal/          |
                                                    | embeddings         |
                                                    +--------------------+
```

## Architectural Principles

- Local-first: all primary data is stored on the local machine.
- Single-process runtime: no daemon, queue, or background worker.
- Dual persistence: each note is stored as Markdown and indexed in SQLite.
- Graceful degradation: FTS works without embeddings; vectors are optional.
- Agent-native integration: MCP is a first-class runtime, not an afterthought.
- CGO-free distribution: SQLite runs via `go-sqlite3` + `wazero`, enabling static builds.

## Package Map

### Entry point

- `cmd/uniam/main.go`
  - Tiny executable wrapper.
  - Calls `pkg/cli.Execute()`.

### CLI layer

- `pkg/cli/root.go`
  - Registers all subcommands.
- `pkg/cli/init.go`
  - Creates `UNIAM_HOME`, shelves directory, config, and DB.
- `pkg/cli/store.go`
  - Collects note fields and calls `core.Service.Store`.
- `pkg/cli/search.go`
  - Calls `core.Service.Search`.
- `pkg/cli/list.go`
  - Calls `core.Service.GetContext`.
- `pkg/cli/retrieve.go`
  - Calls `core.Service.GetDetails`.
- `pkg/cli/remove.go`
  - Calls `core.Service.Remove`.
- `pkg/cli/reindex.go`
  - Rebuilds vector table using current embedding provider.
- `pkg/cli/doctor.go`
  - End-to-end health check for filesystem, config, DB, vectors, and provider reachability.
- `pkg/cli/config.go`
  - Reads/writes config templates and prints redacted config.
- `pkg/cli/mcp.go`
  - Launches the stdio MCP server.
- `pkg/cli/setup.go`
  - Large agent-integration subsystem.
  - Writes MCP config for multiple agent ecosystems.
- `pkg/cli/setup_skill.go`
  - Embeds and installs skill files.
- `pkg/cli/setup_codesearch.go`
  - Installs and reuses the optional `code-search` MCP server.

### Application layer

- `internal/core/service.go`
  - The orchestration center of the project.
  - Owns config loading, redaction, deduplication, note writing, DB writes, search mode selection, vector initialization, and reindexing.
- `internal/core/errors.go`
  - Local validation error type.

### Domain models

- `internal/models/models.go`
  - `RawItemInput`
  - `Item`
  - `ItemDetail`
  - `SearchResult`
  - category metadata and anchor generation

### Persistence layer

- `internal/storage/shelves.go`
  - Markdown file rendering and mutation.
  - Maintains frontmatter, category sections, and note appends.
- `internal/db/db.go`
  - SQLite-backed structured storage and search.
  - Creates tables, FTS virtual table, triggers, vec table, and meta.
- `internal/db/models.go`
  - GORM models for structured tables.
- `internal/db/interface.go`
  - Store interface used by `core` and tests.

### Search and embeddings

- `internal/search/hybrid.go`
  - FTS/vector merge and tiered search.
- `internal/embeddings/*`
  - Provider abstraction plus adapters for:
  - Ollama
  - OpenAI
  - OpenRouter via OpenAI-compatible client
  - Google Gemini embeddings

### MCP layer

- `internal/mcp/server.go`
  - Registers:
  - `uniam_store`
  - `uniam_search`
  - `uniam_context`
  - Adapts JSON-like MCP input into service calls.

### Support infrastructure

- `internal/config/config.go`
  - Default config values, YAML load/save, env overrides, validation.
- `internal/redaction/redaction.go`
  - Three-layer redaction pipeline.
- `internal/gormlite/*`
  - Vendored GORM SQLite dialector/migrator adapted for the `go-sqlite3` runtime used here.

## Runtime Flows

### 1. Initialization flow

```text
uniam init
  -> config.GetUniamHome()
  -> create ~/.uniam/shelves
  -> create ~/.uniam/config.yaml if missing
  -> core.NewService(home)
       -> load config
       -> validate config
       -> db.NewDB(index.db)
            -> open SQLite through gormlite/go-sqlite3
            -> run migrations
            -> create FTS table + triggers
            -> recreate vec table if meta.embedding_dim exists
       -> load .uniamignore
       -> compile custom redaction regexps
```

### 2. Store flow

This is the most important write path in the system.

```text
CLI or MCP store request
  -> build RawItemInput
  -> core.Service.Store()
       -> resolve project name
       -> redact what/why/impact/details
       -> tryDedup()
            -> FTS search candidate notes in same project
            -> normalize against broad search top score
            -> require exact title match + threshold
            -> if duplicate:
                 update DB row + append details block
                 return action=updated
       -> models.FromRaw()
       -> storage.WriteNoteItem()
            -> create/append YYYY-MM-DD-notes.md
            -> maintain category ordering and frontmatter
       -> db.InsertItem()
       -> optional detail insert
       -> lazy embedding provider init
       -> embed combined text
       -> ensure vec table exists with correct dimension
       -> insert vector row
       -> return id/file_path/action
```

Important consequence: Markdown is written before DB insert. The DB is the query index, but the shelf file is also a primary artifact.

### 3. Search flow

```text
CLI/MCP search
  -> core.Service.Search(query, limit, project, source, useVectors)
       -> GetEmbeddingProvider()
       -> if provider unavailable or vectors disabled or vec table absent:
            -> db.FTSSearch()
       -> else:
            -> search.TieredSearch()
                 -> db.FTSSearch(limit*2)
                 -> if enough keyword hits:
                      return FTS only
                 -> else:
                      embed query
                      db.VectorSearch(limit*2)
                      merge weighted results
```

Design intent: vector search is a fallback or enhancement, not a mandatory dependency.

### 4. Context flow

`GetContext` powers recent note retrieval and startup context injection.

```text
context request
  -> count total notes
  -> if query present:
       choose vector usage based on semantic mode:
       - always
       - auto
       - never
       run Search()
       optionally top up with recent notes
  -> else:
       db.ListRecent()
```

### 5. MCP flow

```text
uniam mcp
  -> internal/mcp.RunServer()
  -> core.NewService("")
  -> mcpsdk.NewServer(...)
  -> registerTools()
  -> stdio transport loop

tool handlers:
  uniam_store   -> HandleUniamStore   -> svc.Store
  uniam_search  -> HandleUniamSearch  -> svc.Search
  uniam_context -> HandleUniamContext -> svc.GetContext
```

The MCP layer is intentionally thin. Business behavior stays in `core.Service`.

## Data Model

### Filesystem layout

```text
UNIAM_HOME/
├── config.yaml
├── index.db
├── .uniamignore
└── shelves/
    └── <project>/
        ├── 2026-04-20-notes.md
        ├── 2026-04-21-notes.md
        └── ...
```

### Markdown note file shape

Each daily file contains:

- YAML frontmatter
  - `project`
  - `sources`
  - `created`
  - `tags`
- body
  - `# <date> Notes`
  - category headings (`## Decisions`, `## Bugs Fixed`, etc.)
  - per-note sections (`### <title>`)
  - required `**What:**`
  - optional `**Why:**`, `**Impact:**`, `**Source:**`
  - optional `<details> ... </details>` block

### SQLite schema

Structured tables:

- `items`
  - canonical searchable metadata
- `item_details`
  - larger body text stored separately
- `meta`
  - currently used for `embedding_dim`

Virtual tables:

- `items_fts`
  - FTS5 virtual table indexing title/what/why/impact/tags/category/project/source
- `items_vec`
  - `vec0` virtual table for embeddings
  - created only after dimension is known

Triggers:

- `items_ai`
  - inserts new row into `items_fts`
- `items_au`
  - deletes old FTS row and reinserts updated row

### Domain object relationships

```text
RawItemInput
   |
   v
Item -------------------------> items table
   |                                |
   +--> details text (optional) --> item_details table
   |
   +--> markdown section ---------> shelves/<project>/<date>-notes.md
   |
   +--> embedding text -----------> items_vec table (optional)
```

## Search Architecture

### Keyword search

- Implemented with SQLite FTS5.
- Queries are split into terms and converted into a prefix query:
  - `"term1"* OR "term2"*`
- Filters can be applied by project and source.
- Returned score is `-fts.rank`, then normalized later when needed.

### Vector search

- Enabled only after embeddings exist and a vec table has been created.
- Embeddings are stored as JSON-marshaled `[]float32`.
- On provider dimension mismatch, writes/searches fail with a guidance error to run `uniam reindex`.

### Hybrid ranking

Default weighting in `internal/search/hybrid.go`:

- FTS weight: `0.3`
- vector weight: `0.7`

The code normalizes each result set to `[0..1]` before merging, deduplicates by ID, sums weighted scores, and sorts descending.

### Tiered optimization

`TieredSearch` avoids calling the embedding provider unless keyword results are sparse.

That matters because:

- local Ollama may be slow or absent
- remote embedding APIs cost time and money
- many practical note lookups are already satisfied by FTS

## Redaction Pipeline

Redaction happens before persistence, inside `core.Service.Store`.

Three layers are applied:

1. Explicit `<redacted>...</redacted>` tags
2. Built-in regexes for common secrets
3. Custom regexes loaded from `.uniamignore`

This means both Markdown shelves and the SQLite index receive redacted text, not raw text.

## Configuration Architecture

Source of truth is `internal/config.Config`:

```yaml
embedding:
  provider: ollama|openai|openrouter|google
  model: ...
  base_url: ...
  api_key: ...

context:
  semantic: auto|always|never
  topup_recent: true|false
```

Load order:

1. defaults in code
2. `config.yaml`
3. environment variable overrides

Supported env overrides:

- `UNIAM_HOME`
- `UNIAM_EMBEDDING_PROVIDER`
- `UNIAM_EMBEDDING_MODEL`
- `UNIAM_EMBEDDING_API_KEY`
- `UNIAM_EMBEDDING_BASE_URL`
- `UNIAM_CONTEXT_SEMANTIC`

This lets MCP hosts inject secrets via environment instead of disk config.

## Embedding Provider Architecture

Provider interface:

```go
type Provider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}
```

Implementations:

- `OllamaProvider`
  - direct HTTP call to `/api/embeddings`
- `OpenAIProvider`
  - OpenAI SDK
  - also used for OpenRouter by overriding `base_url`
- `GoogleProvider`
  - direct Gemini embedContent HTTP call

Factory selection happens in `internal/embeddings/factory.go`.

Providers are lazy-initialized in `core.Service` via `sync.Once`.

## SQLite and GORM Architecture

This project has an important non-obvious dependency arrangement.

### Why `internal/gormlite` exists

The repository vendors a custom `gormlite` dialector from `github.com/ncruces/go-sqlite3/gormlite`.

Reason:

- Uniam uses `github.com/ncruces/go-sqlite3`, a pure-Go SQLite runtime.
- It also imports `sqlite-vec` bindings compatible with that runtime.
- Standard GORM SQLite integration is not used here.
- The vendored dialector keeps GORM compatible with this stack.

### SQLite runtime stack

```text
GORM
  -> internal/gormlite dialector
      -> github.com/ncruces/go-sqlite3
          -> wazero WebAssembly runtime
              -> SQLite + sqlite-vec
```

### Why this matters

- preserves CGO-free builds
- allows static distribution
- keeps vector search embedded in the binary/runtime model
- forces some raw SQL for FTS5 and vec virtual-table operations

## Agent Integration Subsystem

`pkg/cli/setup.go` is effectively a second product surface.

Responsibilities:

- installs Uniam as an MCP server into different agent config formats
- supports global and project-scoped installation depending on agent
- optionally installs additional MCP integrations:
  - `ripgrep`
  - `code-search`
  - `Context7`
  - `Git`
  - `Brave Search`
- installs embedded skill files for supported agents

Supported targets in code:

- Claude Code
- Cursor
- Windsurf
- Antigravity
- Codex / Codex CLI
- OpenCode
- GitHub Copilot
- Gemini CLI

This setup layer is intentionally idempotent for most config writers, but Codex uninstall is manual.

## Testing Strategy

The codebase uses two complementary test styles.

### Package-level Go tests

Covered packages:

- `internal/core`
- `internal/db`
- `internal/search`
- `internal/mcp`
- `internal/storage`
- `internal/models`
- `internal/config`
- `internal/redaction`
- `internal/embeddings`

These tests act as executable specifications for package behavior:

- note writing
- DB round-trips
- search semantics
- handler behavior
- config defaults
- provider adapters

### End-to-end shell test

- `testing/test-uniam.sh`

This script:

- creates a temporary `UNIAM_HOME`
- exercises real CLI commands
- avoids real local config mutation
- disables embedding dependency by pointing Ollama to a dead localhost port
- validates init/store/search/list/retrieve/remove/notes/config/reindex/setup/mcp/help flows

This is the nearest thing to a full system smoke test in the repo.

## Dependency Direction

The dependency graph is mostly clean and downward:

```text
cmd/uniam
  -> pkg/cli
      -> internal/core
          -> internal/config
          -> internal/db
          -> internal/storage
          -> internal/redaction
          -> internal/search
          -> internal/embeddings
          -> internal/models

internal/mcp
  -> internal/core
  -> internal/models

internal/db
  -> internal/models
  -> internal/gormlite
```

Notable property: `internal/mcp` and `pkg/cli` are both adapters over the same `core.Service`, which keeps behavior consistent across transport layers.

## Strengths of the Current Design

- Simple deployment model: one binary, one local home directory.
- Strong degradation path: note storage and FTS still work without embeddings.
- Good separation between transport adapters and core logic.
- Search path is practical rather than overly abstract.
- Markdown shelves remain user-readable and future-proof.
- Setup subsystem gives the tool real ecosystem reach.

## Architectural Tradeoffs and Constraints

- Dual-write model means markdown file and DB must stay conceptually aligned.
- Some writes are not transactional across file + DB boundaries.
- Dedup logic currently depends on FTS heuristics plus exact title match.
- Search ranking is intentionally simple and hard-coded.
- Setup logic is centralized in one very large file, which is functional but difficult to evolve.
- SQLite/GORM integration relies on a vendored adapter, which adds maintenance cost.

## Suggested Mental Model For Contributors

Treat the project as four layers:

1. transport adapters
   - CLI and MCP
2. orchestration
   - `core.Service`
3. persistence and retrieval
   - files, SQLite, FTS, vectors
4. ecosystem integration
   - setup/install/skills/tests

If a change affects user-visible behavior, it usually belongs in `core.Service`.

If a change affects storage format or retrieval semantics, it usually belongs in `internal/storage`, `internal/db`, or `internal/search`.

If a change affects how agents discover or launch Uniam, it belongs in `pkg/cli/setup*.go`.

## File Guide

Start here when navigating the repo:

- `cmd/uniam/main.go`
- `pkg/cli/root.go`
- `internal/core/service.go`
- `internal/db/db.go`
- `internal/storage/shelves.go`
- `internal/search/hybrid.go`
- `internal/mcp/server.go`
- `pkg/cli/setup.go`
- `internal/gormlite/sqlite.go`
