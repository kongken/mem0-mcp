# mem0-mcp

Streamable HTTP MCP adapter for **self-hosted Mem0 OSS**. It sits in front of your own Mem0 REST API and exposes memory tools to Codex, Claude Code, Cursor, and other MCP clients — no Mem0 Platform account required.

Official Mem0 MCP (`mcp.mem0.ai`) talks to Mem0 Cloud. **This project talks to the Mem0 you run yourself.**

This repo also ships a **Go CLI** for the same OSS API (`mem0 add` / `search` / `list` / `update` / `delete` / … with an agent-JSON mode). See [`cli/`](./cli/README.md).

## Architecture

```
MCP client (Codex / Cursor / Claude Code)
    │  HTTP + X-API-Key: m0sk_...
    ▼
mem0-mcp  :8080/mcp                 ← this repo
    │  REST + X-API-Key (passthrough)
    ▼
Mem0 OSS  :8888                     ← github.com/mem0ai/mem0/server
    │  POST /memories, POST /search, …
    ▼
Postgres + pgvector (your data)
```

| Component | Role | You configure |
| --- | --- | --- |
| **Mem0 OSS** | Stores and searches memories | `OPENAI_API_KEY`, Postgres, admin/API keys |
| **mem0-mcp** | MCP ↔ REST bridge | `MEM0_API_URL`, `MEM0_DEFAULT_USER_ID` |
| **mem0 CLI** | Go CLI against the OSS API (`cli/`) | `MEM0_BASE_URL`, `MEM0_API_KEY` |
| **MCP client** | Calls tools from the agent | MCP URL + `X-API-Key` header |

**Important:** Mem0 API keys (`m0sk_...`) go in the **MCP client**, not in mem0-mcp's environment. mem0-mcp forwards the key from each HTTP request to your Mem0 instance.

---

## 1. Deploy self-hosted Mem0 OSS

Follow the [Mem0 self-hosted server docs](https://docs.mem0.ai/open-source/features/rest-api). Minimal path:

```bash
git clone https://github.com/mem0ai/mem0.git
cd mem0/server
cp .env.example .env
# Edit .env — set at least POSTGRES_PASSWORD and OPENAI_API_KEY

make bootstrap
```

This starts Postgres, the Mem0 API, and the dashboard. After bootstrap you should see:

| Service | URL |
| --- | --- |
| Mem0 REST API | `http://localhost:8888` |
| OpenAPI docs | `http://localhost:8888/docs` |
| Dashboard | `http://localhost:3000` |

Save the **admin password** and **API key** (`m0sk_...`) printed in the `=== Ready ===` block — the key is shown only once.

Verify Mem0 is up:

```bash
curl -s http://localhost:8888/docs | head -1   # HTML from OpenAPI page

curl -X POST http://localhost:8888/memories \
  -H "Content-Type: application/json" \
  -H "X-API-Key: m0sk_YOUR_KEY" \
  -d '{"messages":[{"role":"user","content":"I prefer TypeScript"}],"user_id":"alice"}'
```

### Mem0 OSS API notes

- Paths have **no** `/v1/` prefix: use `POST /memories`, `POST /search`, not `/v1/memories`.
- Auth is on by default. Send `X-API-Key: m0sk_...` on protected endpoints.
- Docker Compose maps host **8888** → container **8000**. From another container on the same network, use `http://mem0:8000`.

More: [Mem0 server README](https://github.com/mem0ai/mem0/tree/main/server), [REST API reference](https://docs.mem0.ai/open-source/features/rest-api).

---

## 2. Deploy mem0-mcp

Point mem0-mcp at your running Mem0 OSS instance and choose the fallback `user_id`. Memory tools may override it explicitly per call.

### Option A — Docker (recommended)

```bash
docker pull ghcr.io/kongken/mem0-mcp:main

docker run -d --name mem0-mcp \
  -p 8080:8080 \
  -e MEM0_API_URL=http://host.docker.internal:8888 \
  -e MEM0_DEFAULT_USER_ID=alice \
  mem0-mcp
```

On Linux without `host.docker.internal`, use your host IP or join the Mem0 Docker network (see Option C).

### Option B — Local Node.js

```bash
cp .env.example .env
```

```env
MEM0_API_URL=http://localhost:8888
MEM0_DEFAULT_USER_ID=alice
HOST=127.0.0.1
PORT=8080
```

```bash
npm install
npm run dev
```

### Option C — Same Docker network as Mem0

Add mem0-mcp to Mem0's compose stack or attach to its network:

```yaml
# compose.yaml (add alongside Mem0 services)
services:
  mem0-mcp:
    image: ghcr.io/kongken/mem0-mcp:main
    ports:
      - "8080:8080"
    environment:
      MEM0_API_URL: http://mem0:8000      # service name + internal port
      MEM0_DEFAULT_USER_ID: alice
      MCP_ALLOWED_HOSTS: localhost,mem0-mcp.example.com
    networks:
      - mem0_network                       # same network as Mem0 OSS
    restart: unless-stopped

networks:
  mem0_network:
    external: true                          # if using Mem0's existing network
```

Check mem0-mcp:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

MCP endpoint: `http://localhost:8080/mcp`

---

## 3. Configure MCP clients

Clients connect to **mem0-mcp**, not directly to Mem0 OSS. Pass the Mem0 API key as `X-API-Key`.

### Codex (`~/.codex/config.toml`)

```toml
[mcp_servers.mem0]
url = "http://127.0.0.1:8080/mcp"
http_headers = { "X-API-Key" = "${MEM0_API_KEY}" }
```

```bash
export MEM0_API_KEY=m0sk_your_key_here
codex
```

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

### End-to-end smoke test

Ask your agent to remember something, then in a new turn ask it to recall:

```
You:   Remember that I prefer TypeScript for new projects.
Agent: Saved.

You:   What language do I prefer for new projects?
Agent: TypeScript.
```

Confirm the memory also appears in the Mem0 dashboard at `http://localhost:3000`.

---

## mem0-mcp environment variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `MEM0_API_URL` | yes | — | Self-hosted Mem0 REST base URL, e.g. `http://localhost:8888` |
| `MEM0_DEFAULT_USER_ID` | yes | — | Fallback `user_id` used when a tool call omits `user_id` |
| `HOST` | no | `0.0.0.0` | HTTP bind address |
| `PORT` | no | `8080` | HTTP port |
| `MCP_HTTP_PATH` | no | `/mcp` | MCP HTTP path |
| `MEM0_REQUEST_TIMEOUT_MS` | no | `30000` | Upstream timeout to Mem0 OSS |
| `MCP_STATELESS` | no | `false` | Stateless Streamable HTTP mode |
| `MCP_ALLOWED_HOSTS` | no | — | Allowed Host headers when binding publicly (recommended with `HOST=0.0.0.0`) |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error` |

**Do not** put `m0sk_...` keys in these variables. Keys belong in MCP client headers only.

---

## MCP tools

| Tool | Mem0 OSS endpoint | Scope input |
| --- | --- | --- |
| `add_memory` | `POST /memories` | optional `user_id` |
| `search_memories` | `POST /search` | optional `user_id`; metadata filters |
| `get_memories` | `GET /memories?user_id=…&top_k=…` | optional `user_id` |
| `get_memory` | `GET /memories/{id}` | exact memory ID |
| `update_memory` | `PUT /memories/{id}` | exact memory ID |
| `delete_memory` | `DELETE /memories/{id}` | exact memory ID |
| `list_entities` | `GET /entities` | optional `user_id`; result filtered to that user |

---

## Security

- Missing `X-API-Key` on MCP requests → `401` before Mem0 is contacted
- `add_memory`, `search_memories`, `get_memories`, and `list_entities` accept an optional top-level `user_id`; otherwise the adapter uses `MEM0_DEFAULT_USER_ID`
- Identity keys are rejected anywhere inside search `filters`; pass `user_id` through the dedicated top-level argument
- `agent_id` and `run_id` are not supported by this adapter
- A Mem0 OSS API key authenticates the caller but does not authorize a specific memory `user_id`; any trusted client with a valid key can select another `user_id`
- No bulk delete, entity cascade delete, configure, or reset in v1
- API keys and memory bodies are redacted from logs
- Keep Mem0 OSS auth enabled; do not use `AUTH_DISABLED=true` in production

## Production checklist

1. **TLS** — terminate HTTPS at nginx/Caddy/Traefik in front of mem0-mcp (and Mem0 dashboard if exposed).
2. **Network** — Mem0 Postgres and API on a private network; only expose mem0-mcp (and dashboard if needed).
3. **Keys** — use separate `m0sk_...` keys per client and rotate by updating client headers; do not treat a key as bound to a memory `user_id`.
4. **Access** — expose the endpoint only to clients trusted to choose any `user_id`, or add an authorization proxy that maps credentials to allowed identities.
5. **Host allowlist** — set `MCP_ALLOWED_HOSTS` when binding `0.0.0.0`.
6. **Identity** — set `MEM0_DEFAULT_USER_ID` to the normal fallback scope used when callers omit `user_id`.

Example nginx for mem0-mcp:

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

## vs official Mem0 MCP

| | Official (`mcp.mem0.ai`) | mem0-mcp (this repo) |
| --- | --- | --- |
| Backend | Mem0 Platform | Your Mem0 OSS |
| Data | Mem0 Cloud | Your Postgres |
| Auth | OAuth / platform key | `X-API-Key` → your Mem0 |
| User scope | Tool-selected user/agent filters | Optional tool `user_id`, then server default |
| Setup | `npx mcp-add` | Deploy Mem0 OSS + this adapter |

## Development

```bash
npm install
npm test
npm run build
npm start
```

## License

MIT
