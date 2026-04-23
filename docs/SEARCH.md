# Search And Optional MCP Integrations

Uniam supports keyword search out of the box, optional semantic search through embeddings, and optional MCP integrations that extend what an installed agent can search or inspect.

## Optional MCP integrations

During `uniam setup`, you can opt into any of these MCP servers. Each prompt defaults to `no`.

| MCP | What it adds |
| --- | --- |
| `ripgrep` | Exact text and regex search across code and config files |
| `code-search` | Broader code discovery, symbol-oriented search, and cross-file navigation |
| `Context7` | Up-to-date library docs, current package versions, and dependency compatibility |
| `Git` | Structured repository status, diff, history, and branch inspection |
| `SearXNG` | Web search through your own SearXNG instance |
| `Brave Search` | Web search for current external information |
| `Firecrawl` | Web scraping, page fetch, crawling, extraction, and live web data access |

### CLI flags

If you want to skip the interactive prompts, opt in explicitly:

```bash
uniam setup claude-code --ripgrep
uniam setup claude-code --code-search
uniam setup claude-code --context7
uniam setup claude-code --git-mcp
uniam setup claude-code --searxng
uniam setup claude-code --brave-search
uniam setup claude-code --firecrawl
```

These flags work with every supported agent setup target.

### Saved state and API keys

Uniam stores optional MCP state in `~/.uniam/config.yaml` under `integrations`.

- `Context7` reuses `integrations.context7_api_key`
- `SearXNG` reuses `integrations.searxng_url`
- `Brave Search` reuses `integrations.brave_search_api_key`
- `Firecrawl` reuses `integrations.firecrawl_api_key`
- `code-search` reuses `integrations.code_search_path` after the first successful local install

If a saved key already exists, setup lets you press Enter to reuse it. If the key is empty and you do not provide one, that MCP is skipped for the current setup run.

If `SearXNG` is enabled, setup reuses `integrations.searxng_url` when present. If no saved URL exists yet, it probes common local instance addresses first and offers the detected URL for reuse. `SearXNG` and `Brave Search` are alternative search-provider choices; `Firecrawl` is a separate scraping and fetch integration that can be enabled alongside either of them.

### ripgrep MCP

`ripgrep` MCP is configured with:

```json
{
  "command": "npx",
  "args": ["-y", "mcp-ripgrep@latest"]
}
```

It is best for:

- exact identifiers
- literals and config keys
- log strings
- regex-based narrowing

It is not meant to replace broader architectural or symbol-oriented discovery.

### code-search MCP

Uniam installs `code-search-mcp` locally and reuses the built entrypoint on later setups. The resulting MCP server is configured with `--allowed-workspace` set to your home directory by default.

It is best for:

- broader code discovery
- symbol relationships
- cross-file navigation
- concept-level exploration when you do not already know the exact string to search for

If you want tighter boundaries, edit your agent MCP config and replace the default `--allowed-workspace` path with specific project roots.

Example JSON shape:

```json
{
  "command": "node",
  "args": [
    "/home/you/.local/share/uniam/code-search-mcp/dist/index.js",
    "--allowed-workspace",
    "/home/you/projects/example"
  ]
}
```

### Context7 MCP

`Context7` is configured with:

```json
{
  "command": "npx",
  "args": ["-y", "@upstash/context7-mcp"],
  "env": {
    "CONTEXT7_API_KEY": "..."
  }
}
```

Use it when an agent needs:

- current library or framework documentation
- the latest package versions
- dependency compatibility details

### Git MCP

Uniam configures the official Git MCP server through `uvx`:

```json
{
  "command": "uvx",
  "args": ["mcp-server-git"]
}
```

This MCP helps when an agent should inspect:

- repository status
- staged or unstaged diffs
- commit history
- branches and tags

Uniam skips Git MCP setup if `uvx` is not available in `PATH`.

### Brave Search MCP

`Brave Search` is configured with:

```json
{
  "command": "npx",
  "args": ["-y", "@brave/brave-search-mcp-server", "--transport", "stdio"],
  "env": {
    "BRAVE_API_KEY": "..."
  }
}
```

Use it when an agent needs current external information from the web.

### SearXNG MCP

`SearXNG` is configured with:

```json
{
  "command": "npx",
  "args": ["-y", "mcp-searxng"],
  "env": {
    "SEARXNG_URL": "http://127.0.0.1:8080"
  }
}
```

Use it when you want web search through your own SearXNG instance instead of a hosted search API.

Uniam reuses an existing `integrations.searxng_url` value when present. If no URL is saved yet, setup probes common local instance addresses and lets you confirm the detected URL or enter one manually. The generated MCP config runs `mcp-searxng` through `npx -y`, so the package is fetched on demand by the agent environment when needed.

### Firecrawl MCP

`Firecrawl` is configured with:

```json
{
  "command": "npx",
  "args": ["-y", "firecrawl-mcp"],
  "env": {
    "FIRECRAWL_API_KEY": "fc-..."
  }
}
```

Use it when an agent needs:

- page fetch and scraping
- structured extraction
- crawling and discovery
- live web content beyond plain search results

## Semantic search

Keyword search (FTS5) works with no extra setup. To also enable semantic vector search, configure an embedding provider in `~/.uniam/config.yaml`:

**Ollama (local, free):**

```yaml
embedding:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:11434
```

Install [Ollama](https://ollama.com), then: `ollama pull nomic-embed-text`

**OpenAI:**

```yaml
embedding:
  provider: openai
  model: text-embedding-3-small
  api_key: sk-...
```

**OpenRouter:**

```yaml
embedding:
  provider: openrouter
  model: openai/text-embedding-3-small
  api_key: sk-or-...
```

**Google (Gemini API):**

```yaml
embedding:
  provider: google
  model: gemini-embedding-001
  api_key: AIzaSy...
```

After changing providers, rebuild the vector index:

```bash
uniam reindex
```
