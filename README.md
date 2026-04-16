# Uniam (Universal Agent Memory)

Local note storage for coding agents. Your agent keeps notes on decisions, bugs, and context across sessions — no cloud, no API keys required, no cost.

**License & Credits:**

- This project is released under the **GNU General Public License v3.0 (GPLv3)**. Anyone may freely copy, distribute, modify, and use this software, including for commercial purposes, provided that any modified versions or derivative works are also distributed openly under the same GPLv3 license.
- The source code of this project was adapted from <https://github.com/mobydeck/pantry>.

## Features

- **Works with multiple agents** — Claude Code, Cursor, Windsurf, Antigravity, Codex, OpenCode, RooCode. One command sets up MCP config for your agent.
- **MCP native** — Runs as an MCP server exposing `uniam_store`, `uniam_search`, and `uniam_context` as tools.
- **Local-first** — Everything stays on your machine. Notes are stored as Markdown in `~/.uniam/shelves/`, readable in Obsidian or any editor.
- **Zero idle cost** — No background processes, no daemon, no RAM overhead. The MCP server only runs when the agent starts it.
- **Hybrid search** — FTS5 keyword search works out of the box. Add Ollama, OpenAI, or OpenRouter for semantic vector search.
- **Secret redaction** — 3-layer redaction strips API keys, passwords, and credentials before anything hits disk.
- **Cross-agent** — Notes stored by one agent are searchable by all agents. One uniam, many agents.

## Install

### Automatic install (recommended)

**macOS / Linux:**

```bash
curl -sSL https://github.com/pdasilem/uniam/releases/latest/download/install.sh | sh
```

The script detects your OS and architecture, downloads the right binary, and installs it to `/usr/local/bin` (will prompt for `sudo` if needed). If you decline sudo, it falls back to `~/.local/bin` and updates your shell profile automatically.

**Windows (PowerShell):**

```powershell
irm https://github.com/pdasilem/uniam/releases/latest/download/install.ps1 | iex
```

Installs to `%LOCALAPPDATA%\Programs\uniam\` and adds it to your user `PATH` via `SETX`. **Restart your terminal** after installation for PATH changes to take effect.

### Manual download

1. Go to the [Releases](../../releases) page and download the binary for your platform:

   | Platform | File |
   |----------|------|
   | macOS (Apple Silicon) | `uniam-darwin-arm64` |
   | macOS (Intel) | `uniam-darwin-amd64` |
   | Linux x86-64 | `uniam-linux-amd64` |
   | Linux ARM64 | `uniam-linux-arm64` |
   | Windows x86-64 | `uniam-windows-amd64.exe` |

2. Make it executable and move it to your PATH (macOS/Linux):

   ```bash
   chmod +x uniam-darwin-arm64
   mv uniam-darwin-arm64 /usr/local/bin/uniam
   ```

3. On macOS you may need to allow the binary in **System Settings → Privacy & Security** the first time you run it.

### Initialize

```bash
uniam init
```

### Connect your agent

```bash
uniam setup claude-code   # or: cursor, windsurf, antigravity, codex, codex-cli, opencode, roocode, copilot, gemini-cli
```

During setup (except for Windsurf), you will be prompted to install **fast context MCP servers** (`ripgrep` and `code-search-mcp`). Answering "yes" will also add these powerful context retrieval plugins to your agent's configuration.

This writes the MCP server entry into your agent's config file. Restart the agent and uniam will be available as a tool.

Run `uniam setup` again at any time to re-apply the config — it is **idempotent** and will not overwrite other entries in your agent's config. This will also update installed agent skill files to their latest version.

Run `uniam doctor` to verify everything is working.

> **Note**: If you want to configure your search parameters, set up custom search directories, or manage your GitHub PAT constraints, refer to the [Search Configuration Guide](docs/SEARCH.md).

### Updating

To update the binary, download the new release from the [Releases](../../releases) page and replace the old file in your PATH:

```bash
chmod +x uniam-linux-amd64
mv uniam-linux-amd64 /usr/local/bin/uniam
```

No config changes are needed — the agent always runs whatever `uniam` binary is in PATH when it starts the MCP server. Restart the agent after replacing the binary.

### Tell your agent to use Uniam

MCP registration makes the tools available, but your agent also needs instructions to actually use them. The `setup` command installs a skill file automatically for agents that support it (Claude Code, Cursor, Windsurf, Antigravity, Codex, Codex CLI, Copilot, Gemini CLI). For other agents — or if you prefer to use a project-level rules file — add the following to your [AGENTS.md](AGENTS.md), `.rules`, `CLAUDE.md`, or equivalent:

> **Note for VS Code Copilot users**: Copilot does not automatically read global skill files. You must manually copy the instructions from the installed `skills` folder (e.g., `~/.uniam/skills/uniam.md`) directly into your VS Code Copilot Chat Rules settings to ensure proper agent behavior.

```markdown
## Uniam — persistent notes

You have access to a persistent note storage system via the `uniam` MCP tools.

**Session start — MANDATORY**: Before doing any work, retrieve notes from previous sessions:
- Call `uniam_context` to get recent notes for this project
- If the request relates to a specific topic, also call `uniam_search` with relevant terms

**During session — MANDATORY**: While working, periodically call `uniam_store` at meaningful checkpoints:
- After architectural or design decisions
- After identifying a root cause or important debugging finding
- After discovering non-obvious constraints, patterns, or gotchas
- After the user clarifies or changes a requirement
- During long sessions where context could be lost if interrupted

**Session end — MANDATORY**: After any task that involved changes, decisions, bugs, or learnings, call `uniam_store` with:
- `title`: short descriptive title
- `what`: what happened or was decided
- `why`: reasoning behind it
- `impact`: what changed
- `category`: one of `decision`, `pattern`, `bug`, `context`, `learning`
- `details`: full context for a future agent with no prior knowledge

Do not skip any step. Retrieve at the start, store during meaningful checkpoints, and store again at the end. Notes are how context survives across sessions.
```

## Semantic search (optional)

Keyword search (FTS5) works with no extra setup. To enable AI-powered semantic vector search using models like Ollama, OpenAI, or Gemini, please see the [Semantic Search Setup Guide](docs/SEARCH.md#semantic-search-optional).

## Environment variables

All config file values can be overridden with environment variables. They take precedence over `~/.uniam/config.yaml` and are useful when the MCP host injects secrets into the environment instead of writing them to disk.

| Variable | Description | Example |
|----------|-------------|---------|
| `UNIAM_HOME` | Override uniam home directory | `/data/uniam` |
| `UNIAM_EMBEDDING_PROVIDER` | Embedding provider | `ollama`, `openai`, `openrouter`, `google` |
| `UNIAM_EMBEDDING_MODEL` | Embedding model name | `text-embedding-3-small`, `gemini-embedding-001` |
| `UNIAM_EMBEDDING_API_KEY` | API key for the embedding provider | `sk-...`, `AIzaSy...` |
| `UNIAM_EMBEDDING_BASE_URL` | Base URL for the embedding API | `http://localhost:11434` |
| `UNIAM_CONTEXT_SEMANTIC` | Semantic search mode | `auto`, `always`, `never` |

### Examples

Use OpenAI embeddings without putting the key in the config file:

```bash
UNIAM_EMBEDDING_PROVIDER=openai \
UNIAM_EMBEDDING_MODEL=text-embedding-3-small \
UNIAM_EMBEDDING_API_KEY=sk-... \
uniam search "rate limiting"
```

Point a second uniam instance at a different directory (useful for testing or per-workspace isolation):

```bash
UNIAM_HOME=/tmp/uniam-test uniam init
UNIAM_HOME=/tmp/uniam-test uniam store -t "test note" -w "testing" -y "because"
```

Pass the API key through the MCP server config so it is injected at launch time rather than stored on disk. Example for Claude Code (`~/.claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "uniam": {
      "command": "uniam",
      "args": ["mcp"],
      "env": {
        "UNIAM_EMBEDDING_PROVIDER": "openai",
        "UNIAM_EMBEDDING_MODEL": "text-embedding-3-small",
        "UNIAM_EMBEDDING_API_KEY": "sk-..."
      }
    }
  }
}
```

Disable semantic search entirely for a single invocation (falls back to FTS5 keyword search):

```bash
UNIAM_CONTEXT_SEMANTIC=never uniam search "connection pool"
```

## Commands

```
uniam init                  Initialize uniam (~/.uniam)
uniam doctor                Check health and capabilities
uniam store                 Store a note
uniam search <query>        Search notes
uniam retrieve <id>         Show full note details
uniam list                  List recent notes
uniam remove <id>           Delete a note
uniam notes                 List daily note files (alias: log)
uniam config                Show current configuration
uniam config init           Generate a starter config.yaml
uniam setup <agent>         Configure MCP for an agent
uniam uninstall <agent>     Remove agent MCP config
uniam reindex               Rebuild vector search index
uniam version               Print version
```

## Storing notes manually

```bash
uniam store \
  -t "Switched to JWT auth" \
  -w "Replaced session cookies with JWT" \
  -y "Needed stateless auth for API" \
  -i "All endpoints now require Bearer token" \
  -g "auth,jwt" \
  -c "decision"
```

## Flag reference

`uniam store`:

| Flag | Short | Description |
|------|-------|-------------|
| `--title` | `-t` | Title (required) |
| `--what` | `-w` | What happened or was learned (required) |
| `--why` | `-y` | Why it matters |
| `--impact` | `-i` | Impact or consequences |
| `--tags` | `-g` | Comma-separated tags |
| `--category` | `-c` | `decision`, `pattern`, `bug`, `context`, `learning` |
| `--details` | `-d` | Extended details |
| `--source` | `-s` | Source agent identifier |
| `--project` | `-p` | Project name (defaults to current directory) |

`uniam list` / `uniam search` / `uniam notes`:

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Filter to current project |
| `--limit` | `-n` | Maximum results |
| `--source` | `-s` | Filter by source agent |
| `--query` | `-q` | Text filter (list only) |

## Under the hood

### CGO-free, pure Go

Uniam is built without CGO. SQLite runs as a WebAssembly module inside the process via [wazero](https://github.com/tetratelabs/wazero) — a zero-dependency, pure-Go WASM runtime. This means:

- **No C compiler needed** — `go build` just works, no `gcc`, `musl`, or `zig` required
- **True static binaries** — the distributed binaries have no shared library dependencies (`ldd` shows nothing)
- **Cross-compilation is trivial** — all five platform targets (`GOOS`/`GOARCH`) build from a single `go build` invocation with `CGO_ENABLED=0`
- **Reproducible builds** — no C toolchain version drift

The tradeoff: first query of a session pays a one-time ~10 ms WASM compilation cost. Subsequent queries are fast.

### SQLite extensions

Two SQLite extensions are compiled into the binary as embedded WASM blobs:

**[sqlite-vec](https://github.com/asg017/sqlite-vec)** — vector similarity search. Uniam uses it to store note embeddings as 768- or 1536-dimensional `float32` vectors in a `vec0` virtual table, then retrieves the nearest neighbours with a single SQL query:

```sql
SELECT note_id, distance
FROM vec_notes
WHERE embedding MATCH ?
ORDER BY distance
LIMIT 20
```

The extension is loaded at connection open time via `sqlite3_load_extension` equivalent in the WASM host.

**FTS5** — SQLite's built-in full-text search virtual table. Notes are indexed in an `fts_notes` shadow table using the `porter` tokenizer (English stemming). FTS5 handles keyword search when no embedding provider is configured, and also runs alongside vector search as a hybrid fallback.

### Storage layout

Notes live in `~/.uniam/`:

```
~/.uniam/
  config.yaml          # embedding provider, model, API key
  uniam.db            # SQLite database (WAL mode)
  shelves/
    project/
      YYYY-MM-DD.md    # daily Markdown files — human-readable, Obsidian-compatible
```

The SQLite database holds structured note data and search indexes. The Markdown files in `shelves/` are append-only daily logs — they're the canonical human-readable view and survive even if the database is deleted (run `uniam reindex` to rebuild from them).

### GORM + vendored gormlite

The ORM layer uses [GORM](https://gorm.io) with a vendored copy of [gormlite](https://github.com/ncruces/go-sqlite3/tree/main/gormlite) — the SQLite GORM dialector from the same `ncruces/go-sqlite3` ecosystem. It's vendored (at `internal/gormlite/`) rather than imported as a module because the gormlite sub-module is versioned independently from the parent `go-sqlite3` package, and the sqlite-vec WASM binary constrains the host runtime to `go-sqlite3 v0.23.x`. Vendoring decouples dialector quality from host runtime version.

### Dependency count

The binary embeds everything it needs. Runtime dependencies: zero. The full `go.mod` direct dependency list:

| Package | Role |
|---------|------|
| `ncruces/go-sqlite3` | SQLite via WASM/wazero |
| `asg017/sqlite-vec-go-bindings` | Vector search extension (WASM blob) |
| `tetratelabs/wazero` | Pure-Go WebAssembly runtime |
| `gorm.io/gorm` | ORM |
| `modelcontextprotocol/go-sdk` | MCP server |
| `openai/openai-go` | OpenAI/OpenRouter embedding API |
| `spf13/cobra` | CLI |
| `google/uuid` | Note IDs |
| `go.yaml.in/yaml/v3` | Config parsing |

## License

MIT
