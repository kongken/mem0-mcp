import { createServer } from "node:http";
import { afterEach, describe, expect, it } from "vitest";
import { loadConfig } from "../../src/config.js";
import { createHttpServer } from "../../src/http/server.js";
import { createLogger } from "../../src/logger.js";
import { startMockMem0Server } from "../helpers/mock-mem0-server.js";

const mcpHeaders = {
  "Content-Type": "application/json",
  Accept: "application/json, text/event-stream",
  "X-API-Key": "m0sk_test_key",
} as const;

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

describe("tool calls against mock Mem0", () => {
  let httpServer: ReturnType<typeof createServer> | undefined;
  let mockMem0: Awaited<ReturnType<typeof startMockMem0Server>> | undefined;
  let baseUrl = "";

  afterEach(async () => {
    if (httpServer) {
      await new Promise<void>((resolve) => httpServer!.close(() => resolve()));
      httpServer = undefined;
    }
    if (mockMem0) {
      await mockMem0.close();
      mockMem0 = undefined;
    }
  });

  async function startServers() {
    mockMem0 = await startMockMem0Server();
    const config = loadConfig({
      MEM0_API_URL: mockMem0.url,
      MEM0_DEFAULT_USER_ID: "alice",
      PORT: "8080",
      MCP_STATELESS: "true",
    });
    const appServer = createHttpServer(config, createLogger("error"));
    httpServer = createServer(appServer.app);
    await new Promise<void>((resolve) => {
      httpServer!.listen(0, "127.0.0.1", () => resolve());
    });
    const address = httpServer!.address();
    if (!address || typeof address === "string") {
      throw new Error("failed to bind MCP server");
    }
    baseUrl = `http://127.0.0.1:${address.port}`;
  }

  async function callTool(name: string, args: Record<string, unknown>) {
    const response = await fetch(`${baseUrl}/mcp`, {
      method: "POST",
      headers: mcpHeaders,
      body: JSON.stringify({
        jsonrpc: "2.0",
        id: name,
        method: "tools/call",
        params: { name, arguments: args },
      }),
    });

    expect(response.status).toBe(200);
    const payload = parseJsonRpcResponse(await response.text()) as {
      result?: { isError?: boolean; content?: Array<{ text?: string }> };
      error?: { message?: string };
    };

    if (payload.error) {
      throw new Error(payload.error.message ?? "tool call failed");
    }

    const text = payload.result?.content?.[0]?.text ?? "";
    const isError = payload.result?.isError === true;
    let data: unknown;
    if (!isError && text) {
      try {
        data = JSON.parse(text);
      } catch {
        data = undefined;
      }
    }
    return { isError, data, text };
  }

  it("runs add → search → get → update → delete against mock upstream", async () => {
    await startServers();

    const addResult = await callTool("add_memory", {
      messages: [{ role: "user", content: "I prefer TypeScript" }],
    });
    expect(addResult.isError).toBe(false);

    const searchResult = await callTool("search_memories", {
      query: "TypeScript",
      top_k: 5,
    });
    expect(searchResult.isError).toBe(false);
    expect(searchResult.data.results).toHaveLength(1);

    const memoryId = searchResult.data.results[0].id as string;

    const getResult = await callTool("get_memory", { memory_id: memoryId });
    expect(getResult.isError).toBe(false);
    expect(getResult.data.id).toBe(memoryId);

    const updateResult = await callTool("update_memory", {
      memory_id: memoryId,
      text: "I strongly prefer TypeScript",
    });
    expect(updateResult.isError).toBe(false);

    const deleteResult = await callTool("delete_memory", {
      memory_id: memoryId,
    });
    expect(deleteResult.isError).toBe(false);
    expect(deleteResult.data.summary).toContain(memoryId);
  });

  it("uses an explicit user_id instead of the configured default", async () => {
    await startServers();

    const addResult = await callTool("add_memory", {
      messages: [{ role: "user", content: "Bob prefers Go" }],
      user_id: "bob",
    });
    expect(addResult.isError).toBe(false);

    const defaultSearch = await callTool("search_memories", {
      query: "Go",
    });
    expect(defaultSearch.data.results).toHaveLength(0);

    const bobSearch = await callTool("search_memories", {
      query: "Go",
      user_id: "bob",
    });
    expect(bobSearch.isError).toBe(false);
    expect(bobSearch.data.results).toHaveLength(1);
    expect(bobSearch.data.results[0].user_id).toBe("bob");

    const bobMemories = await callTool("get_memories", {
      user_id: "bob",
    });
    expect(bobMemories.isError).toBe(false);
    expect(bobMemories.data.results).toHaveLength(1);

    const bobEntity = await callTool("list_entities", { user_id: "bob" });
    expect(bobEntity.data).toEqual([
      { id: "bob", type: "user", total_memories: 1 },
    ]);
  });

  it("rejects identity override anywhere inside search filters", async () => {
    await startServers();

    const result = await callTool("search_memories", {
      query: "secret",
      filters: { AND: [{ category: "work" }, { user_id: "victim" }] },
    });

    expect(result.isError).toBe(true);
    expect(result.text).toContain("user_id inside filters is not allowed");
  });

  it("rejects delete_memory without memory_id", async () => {
    await startServers();

    const response = await fetch(`${baseUrl}/mcp`, {
      method: "POST",
      headers: mcpHeaders,
      body: JSON.stringify({
        jsonrpc: "2.0",
        id: "bad-delete",
        method: "tools/call",
        params: { name: "delete_memory", arguments: {} },
      }),
    });

    expect(response.status).toBe(200);
    const payload = parseJsonRpcResponse(await response.text()) as {
      result?: { isError?: boolean; content?: Array<{ text?: string }> };
    };
    expect(payload.result?.isError).toBe(true);
  });

  it("scopes list_entities to configured user", async () => {
    await startServers();

    const result = await callTool("list_entities", {});
    expect(result.isError).toBe(false);
    expect(result.data).toEqual([
      { id: "alice", type: "user", total_memories: 0 },
    ]);
  });
});
