# mem0 CLI (Go)

A Go command-line client for the **self-hosted Mem0 OSS** REST API
(`github.com/mem0ai/mem0`, the `server/` directory). It is a sibling of the
TypeScript `mem0-mcp` adapter in this repo: the MCP server gives agents memory
tools, this CLI gives humans (and agent tool loops `--agent`) the same memory
layer from the terminal.

Unlike the official `@mem0/cli` / `mem0-cli`, which talk to the Mem0 **Platform**
API, this CLI talks directly to the OSS server you run yourself:

- paths have **no `/v1/` prefix** (`POST /memories`, `POST /search`, …)
- auth is per-request via the `X-API-Key: m0sk_...` header
- `GET /memories` returns `{"results": [...]}` unscoped and a bare array when
  scoped — the client normalizes both

## Build

```bash
cd cli
go build -o mem0 .
```

Needs Go ≥ 1.22. Zero runtime dependencies beyond cobra and go-pretty.

## Quick start

```bash
# point the CLI at your OSS server
mem0 init --api-key m0sk_xxx --base-url http://localhost:8888 --user-id alice
# or interactively:
mem0 init

# use it
mem0 add "Alice prefers dark mode and vim" --user-id alice
mem0 search "what does alice prefer?" --user-id alice
mem0 list --user-id alice --output table
mem0 update <id> "Alice switched to light mode"
mem0 delete <id>
```

## Configuration

Precedence: **flags > environment > `~/.mem0/config.json`** (0600).

| Source | Example |
| --- | --- |
| flag | `mem0 list --user-id alice --base-url http://x` |
| env | `MEM0_API_KEY`, `MEM0_BASE_URL`, `MEM0_USER_ID`, `MEM0_AGENT_ID`, `MEM0_RUN_ID` (`MEM0_DEFAULT_USER_ID` also accepted) |
| file | `mem0 config set user_id alice`, `mem0 config set base_url http://localhost:8888` |

The default base URL is `http://localhost:8888` (the OSS compose mapping).

## Commands

| Command | Description |
| --- | --- |
| `mem0 init` | Setup wizard, or non-interactive via `--api-key` / `--email`. Email+password path can register the first admin, log in and mint an API key |
| `mem0 add [text]` | Add a memory — text arg, `--file`, `--json-messages`, or piped stdin |
| `mem0 search <query>` | Semantic search (`--top-k`, `--threshold`, `--filter k=v`) |
| `mem0 list` | List memories (`--limit`, `--show-expired`; scoped or admin-wide) |
| `mem0 get <id>` | Get one memory (`--history` for its edit trail) |
| `mem0 update <id> [text]` | Update text / `--metadata` / `--expiration-date` |
| `mem0 delete <id>...` | Delete by ID, or `--all --user-id X` to wipe a scope |
| `mem0 import <file.json>` | Bulk import (one `POST /memories` per entry, per-entry error isolation) |
| `mem0 entity` | `entity list`, `entity delete user\|agent\|run <id>` (admin key) |
| `mem0 config` | `show` / `set <key> <value>` / `unset <key>` / `path` |
| `mem0 status` | Verify connection: server config (LLM/embedder/vector store) + your settings |
| `mem0 version` | Print version |

## Agent mode

Pass `--agent` (alias `--json`) on **any** command for one envelope on stdout,
built for tool loops — no colors, sanitized fields, errors as JSON with a
non-zero exit:

```bash
$ mem0 --json search "vim" --user-id alice
{
  "status": "success",
  "command": "search",
  "duration_ms": 187,
  "scope": { "user_id": "alice" },
  "count": 2,
  "data": [ { "id": "…", "memory": "Alice prefers vim", "score": 0.92, "created_at": "2026-01-15 10:00" } ]
}

$ mem0 --json get 00000000-0000-0000-0000-000000000000 ; echo $?
{
  "status": "error",
  "command": "get",
  "duration_ms": 12,
  "data": { "error": "GET memories/…: Memory not found (HTTP 404 Not Found)" }
}
1
```

## Output formats

`--output text` (default) · `json` (sanitized structured JSON) · `table`
(default for `list`) · `quiet` (IDs only). `--output` is ignored in agent mode
(the envelope wins).

## Import file format

```json
[
  { "messages": [{"role":"user","content":"likes Rust"}], "user_id": "dave", "metadata": {"source": "chat"} },
  { "text": "favors PostgreSQL", "user_id": "dave", "expiration_date": "2026-12-31" }
]
```

## Development

```bash
cd cli
go test ./...      # unit tests (client is exercised against httptest)
go vet ./...
```

Layout: `cmd/` (cobra commands), `internal/client/` (typed OSS API client),
`internal/config/` (config file + env resolution).
