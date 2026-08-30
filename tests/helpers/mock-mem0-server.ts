import { createServer, type RequestListener, type Server } from "node:http";
import { URL } from "node:url";

interface MockMem0State {
  memories: Map<string, Record<string, unknown>>;
  nextId: number;
}

export interface MockMem0Server {
  url: string;
  state: MockMem0State;
  close(): Promise<void>;
}

export function startMockMem0Server(): Promise<MockMem0Server> {
  const state: MockMem0State = {
    memories: new Map(),
    nextId: 1,
  };

  const handler: RequestListener = (req, res) => {
    if (!req.url || !req.headers["x-api-key"]) {
      res.writeHead(401, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ detail: "Unauthorized" }));
      return;
    }

    const url = new URL(req.url, "http://127.0.0.1");
    const method = req.method ?? "GET";
    const chunks: Buffer[] = [];

    req.on("data", (chunk) => chunks.push(chunk));
    req.on("end", () => {
      const bodyText = Buffer.concat(chunks).toString("utf8");
      const body = bodyText ? JSON.parse(bodyText) : undefined;
      routeMockRequest(method, url, body, state, res);
    });
  };

  const server = createServer(handler);

  return new Promise((resolve, reject) => {
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (!address || typeof address === "string") {
        reject(new Error("failed to bind mock mem0 server"));
        return;
      }

      resolve({
        url: `http://127.0.0.1:${address.port}`,
        state,
        close: () => closeServer(server),
      });
    });
  });
}

function routeMockRequest(
  method: string,
  url: URL,
  body: unknown,
  state: MockMem0State,
  res: import("node:http").ServerResponse,
): void {
  if (method === "POST" && url.pathname === "/memories") {
    const record = body as {
      messages: unknown[];
      user_id: string;
    };
    const id = `mem-${state.nextId++}`;
    const memory = {
      id,
      memory: record.messages.map((m: { content?: string }) => m.content).join(" "),
      user_id: record.user_id,
    };
    state.memories.set(id, memory);
    json(res, 200, { results: [{ id, event: "ADD" }] });
    return;
  }

  if (method === "POST" && url.pathname === "/search") {
    const record = body as {
      query: string;
      user_id: string;
      top_k?: number;
      filters?: Record<string, unknown>;
    };
    const matches = [...state.memories.values()].filter(
      (memory) =>
        memory.user_id === record.user_id &&
        String(memory.memory).includes(record.query),
    );
    const limited = record.top_k ? matches.slice(0, record.top_k) : matches;
    json(res, 200, { results: limited });
    return;
  }

  if (method === "GET" && url.pathname === "/memories") {
    const userId = url.searchParams.get("user_id");
    const topK = url.searchParams.get("top_k");
    const matches = [...state.memories.values()].filter(
      (memory) => memory.user_id === userId,
    );
    const limited = topK ? matches.slice(0, Number(topK)) : matches;
    json(res, 200, { results: limited });
    return;
  }

  const memoryMatch = url.pathname.match(/^\/memories\/([^/]+)$/);
  if (memoryMatch) {
    const memoryId = decodeURIComponent(memoryMatch[1]!);

    if (method === "GET") {
      const memory = state.memories.get(memoryId);
      if (!memory) {
        json(res, 404, { detail: "Memory not found" });
        return;
      }
      json(res, 200, memory);
      return;
    }

    if (method === "PUT") {
      const existing = state.memories.get(memoryId);
      if (!existing) {
        json(res, 404, { detail: "Memory not found" });
        return;
      }
      const update = body as { text?: string };
      if (update.text) {
        existing.memory = update.text;
      }
      json(res, 200, existing);
      return;
    }

    if (method === "DELETE") {
      state.memories.delete(memoryId);
      json(res, 200, { message: "Memory deleted" });
      return;
    }
  }

  if (method === "GET" && url.pathname === "/entities") {
    json(res, 200, [
      { id: "alice", type: "user", total_memories: state.memories.size },
      { id: "bob", type: "user", total_memories: 0 },
    ]);
    return;
  }

  json(res, 404, { detail: "Not found" });
}

function json(
  res: import("node:http").ServerResponse,
  status: number,
  payload: unknown,
): void {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(JSON.stringify(payload));
}

function closeServer(server: Server): Promise<void> {
  return new Promise((resolve, reject) => {
    server.close((error) => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
  });
}
