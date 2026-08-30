# mem0-mcp

Streamable HTTP MCP adapter for **self-hosted Mem0 OSS**. It maps MCP tools to the Mem0 REST API so Codex, Claude Code, Cursor, and other MCP clients can use your own Mem0 stack without Mem0 Platform.

## Features

- Streamable HTTP MCP endpoint via `@modelcontextprotocol/sdk`
- Per-request `X-API-Key` passthrough to Mem0 OSS (no platform API key in server config)
- Server-enforced default `user_id` scope for memory operations
- Tools: `add_memory`, `search_memories`, `get_memories`, `get_memory`, `update_memory`, `delete_memory`, `list_entities`
- Structured logs with API keys and memory bodies redacted
- Docker image published by GitHub Actions to `ghcr.io/<owner>/mem0-mcp`

## Requirements

- Node.js 20+
- A running [Mem0 OSS REST API](https://docs.mem0.ai/open-source/features/rest-api) server
- Per-user or admin API key issued by your Mem0 OSS instance

## Quick start (local)

```bash
cp .env.example .env
# edit MEM0_API_URL and MEM0_DEFAULT_USER_ID

npm install
npm run dev
```

Health check: `http://127.0.0.1:8080/healthz`

MCP endpoint: `http://127.0.0.1:8080/mcp`

## Docker

Build locally:

```bash
docker build -t mem0-mcp .

docker run --rm -p 8080:8080 \
  -e MEM0_API_URL=http://host.docker.internal:8888 \
  -e MEM0_DEFAULT_USER_ID=alice \
  mem0-mcp
```

Pull from GitHub Container Registry (after CI publishes):

```bash
docker pull ghcr.io/kongken/mem0-mcp:main
```

## Environment variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `MEM0_API_URL` | yes | — | Mem0 OSS REST base URL, e.g. `http://localhost:8888` |
| `MEM0_DEFAULT_USER_ID` | yes | — | User identity injected into memory tool calls |
| `HOST` | no | `0.0.0.0` | HTTP bind address |
| `PORT` | no | `8080` | HTTP port |
| `MCP_HTTP_PATH` | no | `/mcp` | MCP HTTP path |
| `MEM0_REQUEST_TIMEOUT_MS` | no | `30000` | Upstream request timeout |
| `MCP_STATELESS` | no | `false` | Use stateless Streamable HTTP mode |
| `MCP_ALLOWED_HOSTS` | no | — | Comma-separated allowed Host headers when binding publicly (recommended with `HOST=0.0.0.0`) |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, or `error` |

**Do not** configure client API keys in server environment variables. Clients must send `X-API-Key` on every MCP HTTP request.

## MCP client configuration

Clients must send the Mem0 OSS API key as the HTTP header `X-API-Key`. The adapter forwards it upstream and never accepts keys via tool arguments.

### Codex (`~/.codex/config.toml`)

```toml
[mcp_servers.mem0]
url = "http://127.0.0.1:8080/mcp"
http_headers = { "X-API-Key" = "${MEM0_API_KEY}" }
```

Export `MEM0_API_KEY` in the shell that launches Codex.

### Cursor (`~/.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "mem0-oss": {
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "X-API-Key": "m0sk_your_key_here"
      }
    }
  }
}
```

### Claude Code / generic HTTP MCP

```json
{
  "mcpServers": {
    "mem0-oss": {
      "type": "http",
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "X-API-Key": "m0sk_your_key_here"
      }
    }
  }
}
```

## Security boundaries

- Missing `X-API-Key` → `401` before any Mem0 call
- `user_id` / `agent_id` / `run_id` cannot be set via tool arguments or inside `search_memories.filters`; the adapter injects `MEM0_DEFAULT_USER_ID`
- `search_memories` uses Mem0 `top_k`; `get_memories` supports `top_k` only (no pagination)
- `list_entities` returns only the configured user entity, not the full instance catalog
- No `delete_all_memories`, entity cascade delete, configure, or reset tools in v1
- `delete_memory` requires an explicit `memory_id` and returns a deletion summary
- API keys, auth headers, and memory bodies are redacted from default logs and error responses
- Upstream requests use `redirect: manual` and refuse cross-origin redirects

## Production deployment

1. **TLS** — terminate HTTPS at a reverse proxy (nginx, Caddy, Traefik). Do not expose plain HTTP publicly.
2. **Network** — bind internally or restrict ingress to trusted clients/VPN.
3. **Mem0 OSS** — keep auth enabled; prefer per-user `m0sk_...` keys over legacy admin keys.
4. **Key rotation** — rotate Mem0 API keys in the dashboard; update client MCP headers; no server restart required.
5. **Limits** — configure proxy request size limits, timeouts, and rate limits in front of both this adapter and Mem0 OSS.
6. **Host allowlist** — when binding to `0.0.0.0`, set `MCP_ALLOWED_HOSTS` to the hostnames clients use (for example `mem0-mcp.example.com,localhost`).
7. **Identity** — one adapter instance should map to one `MEM0_DEFAULT_USER_ID`. For multi-tenant setups, run separate instances or add explicit allowlists (future work).

Example nginx snippet:

```nginx
location /mcp {
  proxy_pass http://127.0.0.1:8080/mcp;
  proxy_http_version 1.1;
  proxy_set_header Host $host;
  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  proxy_set_header X-Forwarded-Proto $scheme;
  proxy_read_timeout 120s;
  client_max_body_size 1m;
}
```

## Differences from official Mem0 MCP

| Topic | Official Mem0 MCP (`mcp.mem0.ai`) | This adapter |
| --- | --- | --- |
| Backend | Mem0 Platform (cloud) | Self-hosted Mem0 OSS REST API |
| Auth | OAuth / platform API key | `X-API-Key` passthrough to OSS |
| Data location | Mem0 cloud account | Your Mem0 stack |
| Tools | Includes bulk delete, entities delete, events | v1 subset focused on safe CRUD + list entities |
| Identity | Platform-managed | Server-configured `MEM0_DEFAULT_USER_ID` |

Community reference: [`@yeyuan98/mem0-mcp`](https://www.npmjs.com/package/@yeyuan98/mem0-mcp) targets similar goals; this repo focuses on Streamable HTTP, explicit security constraints, and container publishing.

## Development

```bash
npm install
npm test
npm run build
npm start
```

## License

MIT
