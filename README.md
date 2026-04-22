# Uniam (Universal Agent Memory)

Local note storage for coding agents. Your agent keeps notes on decisions, bugs, and context across sessions — no cloud, no API keys required, no cost.

**License & Credits:**

- This project is released under the **GNU General Public License v3.0 (GPLv3)**. Anyone may freely copy, distribute, modify, and use this software, including for commercial purposes, provided that any modified versions or derivative works are also distributed openly under the same GPLv3 license.
- The source code of this project was adapted from <https://github.com/mobydeck/pantry>.

## Features

- **Works with multiple agents** — Claude Code, Cursor, Windsurf, Antigravity, Codex, OpenCode, RooCode, GitHub Copilot, and Gemini CLI. One command sets up MCP config for your agent.
- **MCP native** — Runs as an MCP server exposing project-scoped memory tools such as `uniam_context`, `uniam_search`, `uniam_retrieve`, `uniam_store`, `uniam_archive`, `uniam_supersede`, `uniam_update_note`, `uniam_compact`, and `uniam_explain_search`.
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

1. Go to the [Releases](https://github.com/pdasilem/uniam/releases) page and download the binary for your platform:

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
uniam setup claude-code   # or: cursor, windsurf, antigravity, codex, opencode, roocode, copilot, gemini-cli
```

By default, `setup` asks whether to also install optional MCP servers for `ripgrep`, `code-search`, `Context7`, `Git`, and `Brave Search`.

To skip the prompt and opt in explicitly, use:

```bash
uniam setup claude-code --ripgrep
uniam setup claude-code --code-search
uniam setup claude-code --context7
uniam setup claude-code --git-mcp
uniam setup claude-code --brave-search
```

These flags work with every supported agent setup target.

When you enable `Context7` or `Brave Search`, Uniam reads the saved API key from `~/.uniam/config.yaml` if one is already present there. During setup, you can press Enter to reuse the saved key or enter a new one; new values are written back to the same Uniam config file so you do not need to re-enter the key for each agent.

This writes the MCP server entry into your agent's config file. Restart the agent and uniam will be available as a tool.

If you want Uniam only for a specific repository, use project scope and run setup from the repository root:

```bash
cd /path/to/repo
uniam setup claude-code --project
```

Project scope is always relative to the current working directory. Do not run `--project` from your home directory unless that is the actual project root you want to target.

### Agent scope support

| Agent | Scope support | Notes |
| --- | --- | --- |
| Claude Code | Global and project | Run `uniam setup claude-code --project` from the repo root to write `.mcp.json` and `./.claude/` there |
| Cursor | Global and project | Run `uniam setup cursor --project` from the repo root to write `./.cursor/` there |
| Windsurf | Global only | `--project` is not supported |
| Antigravity | Global and project | Run `uniam setup antigravity --project` from the repo root to write `./.gemini/antigravity/` there |
| Codex | Global and project | Run `uniam setup codex --project` from the repo root to write `./.codex/` there |
| OpenCode | Global and project | Run `uniam setup opencode --project` from the repo root to write `opencode.json` and `./.opencode/` there |
| RooCode | Project only | Run `uniam setup roocode --project` from the repo root; it writes only to `./.roo/mcp.json` |
| GitHub Copilot | Global and project | Run `uniam setup copilot --project` from the repo root to write `.mcp.json` and `./.github/` there |
| Gemini CLI | Global and project | Run `uniam setup gemini-cli --project` from the repo root to write `./.gemini/` there |

Run `uniam setup` again at any time to re-apply the config — it is **idempotent** and will not overwrite other entries in your agent's config. This will also update installed agent skill files to their latest version.

Run `uniam doctor` to verify everything is working.

> **Note**: Optional MCP setup, search-oriented integrations, API key reuse, and code-search workspace guidance are documented in the [Search Configuration Guide](docs/SEARCH.md).

### Updating

Check whether a newer release is available:

```bash
uniam check-update
```

Apply the latest compatible release to the current binary path:

```bash
uniam update
```

Platform notes:

- Linux: if `uniam` is installed in a system-owned path such as `/usr/local/bin/uniam`, run `sudo uniam update`. If it is installed in a user-writable path such as `~/.local/bin/uniam`, plain `uniam update` is enough.
- macOS: use `uniam update` when the binary path is user-writable. If you installed it into a protected system path, run `sudo uniam update`. If Gatekeeper prompts after a manual binary replacement, allow the binary again in **System Settings → Privacy & Security**.
- Windows: use `uniam check-update` to see whether a newer release exists, then replace `uniam-windows-amd64.exe` manually with the latest release asset. In-place self-update of the running `.exe` is not the recommended path on Windows.

Check only without applying:

```bash
uniam update --check-only
```

No config changes are needed — the agent always runs whatever `uniam` binary is in PATH when it starts the MCP server. Restart the agent after updating the binary.

### Upgrading from 1.x to 2.x

Existing notes and indexes are expected to carry forward when you upgrade in place and keep the same `UNIAM_HOME`. Uniam still uses the same home directory and migrates the SQLite schema forward on startup.

Recommended upgrade flow:

1. Replace the old `1.x` binary with the `2.x` binary from the latest release:

   ```bash
   chmod +x uniam-linux-amd64
   sudo mv uniam-linux-amd64 /usr/local/bin/uniam
   ```

   On `1.x`, the `uniam update` command does not exist yet, so the upgrade to `2.x` must start with replacing the binary.

2. Re-run setup for each installed agent with the new `2.x` binary:

   ```bash
   uniam setup claude-code
   uniam setup cursor
   uniam setup opencode
   ```

3. Restart your agent applications.

What happens during re-setup:

- MCP config entries are updated in place
- installed skill files are overwritten in place
- managed OpenCode instructions and plugin files are overwritten in place in the selected scope
- project Copilot instructions are overwritten in place when using project setup

What does **not** get automatically rewritten everywhere:

- manually maintained project rules such as `AGENTS.md`, `CLAUDE.md`, or `.rules`
- the Codex `AGENTS.md` snippet if you already have an older Uniam section there

That means you usually do **not** need to uninstall first just to avoid duplicate skill files. Skill files are replaced at the same path.

The main place that may need manual refresh is old rules text that lives in repo files. If you want a clean refresh of those older rule blocks, remove or update them manually before re-running setup.

For Codex specifically, `uniam uninstall codex` does not fully clean `.codex/AGENTS.md`; it only removes installed skills and tells you to remove Uniam entries from config and `AGENTS.md` manually.

### Removing Uniam

There are two different removal cases:

#### 1. Remove Uniam from an agent

Use:

```bash
uniam uninstall <agent>
```

Examples:

```bash
uniam uninstall claude-code
uniam uninstall cursor
uniam uninstall opencode
```

This removes the Uniam MCP integration for that agent and, where supported, removes installed skill files and related managed assets.

Notes:

- OpenCode uninstall also removes the managed instructions file and plugin in the selected scope
- Codex uninstall is only partial and requires manual cleanup of `.codex/config.toml` and `.codex/AGENTS.md`
- RooCode, Copilot, and some other integrations have platform-specific limits described by the CLI

#### 2. Remove Uniam completely from your machine

To remove Uniam fully:

1. Remove it from the agents you configured:

   ```bash
   uniam uninstall <agent>
   ```

2. Delete the `uniam` binary from your `PATH`
3. Delete `~/.uniam` if you also want to remove all stored notes, indexes, and config

Deleting `~/.uniam` is destructive and removes your local memory store.

### Tell your agent to use Uniam

MCP registration makes the tools available, but your agent also needs instructions to actually use them. The `setup` command installs a skill file automatically for agents that support it (Claude Code, Cursor, Windsurf, Antigravity, Codex, OpenCode, Copilot, Gemini CLI). For other agents — or if you prefer to use a project-level rules file — add the following to your [AGENTS.md](AGENTS.md), `.rules`, `CLAUDE.md`, or equivalent:

> **Note for VS Code Copilot users**: project setup installs repository-local instructions in `.github/copilot-instructions.md`. Global setup still requires you to manually copy the instructions from the installed `skills` folder into your Copilot Chat Rules settings.

```markdown
## Uniam

Use Uniam for cross-session memory.

Required workflow:
- Before meaningful work, retrieve with `uniam_context`, `uniam_search`, or `uniam_retrieve`.
- During long or decision-heavy work, checkpoint with `uniam_store`.
- Before finishing meaningful work, store a final note with `uniam_store`.
- Curate stale or repetitive memory with `uniam_archive`, `uniam_supersede`, `uniam_update_note`, and `uniam_compact`.
- If Context7 is installed, use it to fetch up-to-date library and framework documentation, current package versions, and dependency compatibility details.

Use `uniam_explain_search` when retrieval behavior needs debugging.

Current scope is only the current project or folder. Cross-project access is not allowed.

Store decisions, bugs, root causes, constraints, and non-obvious project context.
Do not store trivial edits, obvious code facts, secrets, or duplicates.
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
uniam stats                 Show note counters and lifecycle stats
uniam explain-search <q>    Explain retrieval behavior for a query
uniam search <query>        Search notes
uniam retrieve <id>         Show full note details
uniam list                  List recent notes
uniam update-note <id>      Update a note explicitly
uniam compact               Create a canonical summary note
uniam remove [id]           Delete one note, or all notes in the current project with confirmation
uniam notes                 List daily note files (alias: log)
uniam config                Show current configuration
uniam config init           Generate a starter config.yaml
uniam check-update          Check for a newer release
uniam update                Update the current binary
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
| `--all` | `-a` | List across all projects instead of the current one |
| `--limit` | `-n` | Maximum results |
| `--source` | `-s` | Filter by source agent |
| `--query` | `-q` | Text filter (list only) |
| `--mode` |  | Retrieval mode (`list`, `search`) |

`uniam reindex`:

| Flag | Short | Description |
|------|-------|-------------|
| `--all` | `-a` | Reindex all projects instead of only the current one |

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
  index.db             # SQLite database (WAL mode)
  shelves/
    project/
      YYYY-MM-DD-notes.md # daily Markdown files — human-readable, Obsidian-compatible
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

GPL-3.0