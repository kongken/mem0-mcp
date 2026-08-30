import { randomUUID } from "node:crypto";
import { createServer } from "node:http";
import { describe, expect, it, afterEach } from "vitest";
import { loadConfig } from "../../src/config.js";
import { createHttpServer } from "../../src/http/server.js";
import { createLogger } from "../../src/logger.js";

function parseJsonRpcResponse(body: string): unknown {
  const trimmed = body.trim();
  if (trimmed.startsWith("event:")) {
    const dataLine = trimmed
      .split("\n")
      .find((line) => line.startsWith("data: "));
    if (!dataLine) {
      throw new Error("SSE response missing data line");
    }
    return JSON.parse(dataLine.slice("data: ".length));
  }
  return JSON.parse(trimmed);
}

const config = loadConfig({
  MEM0_API_URL: "http://127.0.0.1:59999",
  MEM0_DEFAULT_USER_ID: "alice",
  PORT: "8080",
  MCP_STATELESS: "true",
});

describe("streamable HTTP integration", () => {
  let httpServer: ReturnType<typeof createServer> | undefined;
  let baseUrl = "";

  const mcpHeaders = {
    "Content-Type": "application/json",
    Accept: "application/json, text/event-stream",
    "X-API-Key": "m0sk_test_key",
  } as const;
  afterEach(async () => {
    if (httpServer) {
      await new Promise<void>((resolve) => httpServer!.close(() => resolve()));
      httpServer = undefined;
    }
  });

  async function startServer() {
    const appServer = createHttpServer(config, createLogger("error"));
    httpServer = createServer(appServer.app);
    await new Promise<void>((resolve) => {
      httpServer!.listen(0, "127.0.0.1", () => resolve());
    });
    const address = httpServer!.address();
    if (!address || typeof address === "string") {
      throw new Error("failed to bind test server");
    }
    baseUrl = `http://127.0.0.1:${address.port}`;
  }

  it("rejects requests without X-API-Key", async () => {
    await startServer();
    const response = await fetch(`${baseUrl}/mcp`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        jsonrpc: "2.0",
        id: 1,
        method: "initialize",
        params: {
          protocolVersion: "2025-03-26",
          capabilities: {},
          clientInfo: { name: "test", version: "1.0.0" },
        },
      }),
    });

    expect(response.status).toBe(401);
    const body = await response.json();
    expect(body.error.message).toContain("X-API-Key");
    expect(JSON.stringify(body)).not.toContain("m0sk_");
  });

  it("initializes and lists tools with a valid key header", async () => {
    await startServer();

    const initResponse = await fetch(`${baseUrl}/mcp`, {
      method: "POST",
      headers: {
        ...mcpHeaders,
      },
      body: JSON.stringify({
        jsonrpc: "2.0",
        id: 1,
        method: "initialize",
        params: {
          protocolVersion: "2025-03-26",
          capabilities: {},
          clientInfo: { name: "test", version: "1.0.0" },
        },
      }),
    });

    expect(initResponse.status).toBe(200);

    const initializedNotification = await fetch(`${baseUrl}/mcp`, {
      method: "POST",
      headers: mcpHeaders,
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "notifications/initialized",
      }),
    });
    expect([200, 202, 204]).toContain(initializedNotification.status);

    const toolsResponse = await fetch(`${baseUrl}/mcp`, {
      method: "POST",
      headers: mcpHeaders,
      body: JSON.stringify({
        jsonrpc: "2.0",
        id: 2,
        method: "tools/list",
        params: {},
      }),
    });

    expect(toolsResponse.status).toBe(200);
    const toolsBody = parseJsonRpcResponse(await toolsResponse.text()) as {
      result: {
        tools: Array<{
          name: string;
          inputSchema: { properties?: Record<string, unknown> };
        }>;
      };
    };
    const toolNames = toolsBody.result.tools.map((tool) => tool.name);
    expect(toolNames).toEqual([
      "add_memory",
      "search_memories",
      "get_memories",
      "get_memory",
      "update_memory",
      "delete_memory",
      "list_entities",
    ]);

    for (const name of [
      "add_memory",
      "search_memories",
      "get_memories",
      "list_entities",
    ]) {
      const tool = toolsBody.result.tools.find((candidate) => candidate.name === name);
      expect(tool?.inputSchema.properties).toHaveProperty("user_id");
    }
    for (const name of ["get_memory", "update_memory", "delete_memory"]) {
      const tool = toolsBody.result.tools.find((candidate) => candidate.name === name);
      expect(tool?.inputSchema.properties).not.toHaveProperty("user_id");
    }
  });

  it("exposes health endpoint", async () => {
    await startServer();
    const response = await fetch(`${baseUrl}/healthz`);
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ status: "ok" });
  });
});

describe("stateful session", () => {
  let httpServer: ReturnType<typeof createServer> | undefined;
  let baseUrl = "";

  afterEach(async () => {
    if (httpServer) {
      await new Promise<void>((resolve) => httpServer!.close(() => resolve()));
      httpServer = undefined;
    }
  });

  it("requires session id after initialization", async () => {
    const statefulConfig = loadConfig({
      MEM0_API_URL: "http://127.0.0.1:59999",
      MEM0_DEFAULT_USER_ID: "alice",
      PORT: "8080",
      MCP_STATELESS: "false",
    });

    const appServer = createHttpServer(statefulConfig, createLogger("error"));
    httpServer = createServer(appServer.app);
    await new Promise<void>((resolve) => {
      httpServer!.listen(0, "127.0.0.1", () => resolve());
    });
    const address = httpServer!.address();
    if (!address || typeof address === "string") {
      throw new Error("failed to bind test server");
    }
    baseUrl = `http://127.0.0.1:${address.port}`;

    const initResponse = await fetch(`${baseUrl}/mcp`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
        "X-API-Key": "m0sk_test_key",
      },
      body: JSON.stringify({
        jsonrpc: "2.0",
        id: randomUUID(),
        method: "initialize",
        params: {
          protocolVersion: "2025-03-26",
          capabilities: {},
          clientInfo: { name: "test", version: "1.0.0" },
        },
      }),
    });

    expect(initResponse.status).toBe(200);
    const sessionId = initResponse.headers.get("mcp-session-id");
    expect(sessionId).toBeTruthy();

    const toolsResponse = await fetch(`${baseUrl}/mcp`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
        "X-API-Key": "m0sk_test_key",
        "mcp-session-id": sessionId!,
      },
      body: JSON.stringify({
        jsonrpc: "2.0",
        id: randomUUID(),
        method: "tools/list",
        params: {},
      }),
    });

    expect(toolsResponse.status).toBe(200);
  });
});
