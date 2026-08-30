import { describe, expect, it, vi, afterEach } from "vitest";
import { Mem0Client } from "../../src/mem0/client.js";
import { loadConfig } from "../../src/config.js";
import { createLogger } from "../../src/logger.js";

const baseConfig = loadConfig({
  MEM0_API_URL: "http://mem0.test:8888",
  MEM0_DEFAULT_USER_ID: "alice",
});

describe("Mem0Client", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("sends top_k for search requests", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ results: [] }), { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new Mem0Client({
      config: baseConfig,
      logger: createLogger("error"),
    });

    await client.searchMemories("m0sk_secret", {
      query: "typescript",
      user_id: "alice",
      top_k: 5,
    });

    const [, init] = fetchMock.mock.calls[0]!;
    expect(JSON.parse(String(init?.body))).toEqual({
      query: "typescript",
      user_id: "alice",
      top_k: 5,
    });
  });

  it("sends top_k query param for getMemories", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ results: [] }), { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new Mem0Client({
      config: baseConfig,
      logger: createLogger("error"),
    });

    await client.getMemories("m0sk_secret", {
      user_id: "alice",
      top_k: 10,
    });

    const [url] = fetchMock.mock.calls[0]!;
    expect(String(url)).toContain("user_id=alice");
    expect(String(url)).toContain("top_k=10");
  });

  it("forwards X-API-Key and blocks redirects", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ ok: true }), {
        status: 302,
        headers: { Location: "http://evil.example/steal" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new Mem0Client({
      config: baseConfig,
      logger: createLogger("error"),
    });

    await expect(client.listEntities("m0sk_secret")).rejects.toThrow(
      "redirect",
    );

    expect(fetchMock).toHaveBeenCalledWith(
      new URL("/entities", baseConfig.mem0ApiUrl),
      expect.objectContaining({
        headers: expect.objectContaining({
          "X-API-Key": "m0sk_secret",
        }),
        redirect: "manual",
      }),
    );
  });

  it("maps upstream 401 errors", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ detail: "Invalid API key" }), {
          status: 401,
        }),
      ),
    );

    const client = new Mem0Client({
      config: baseConfig,
      logger: createLogger("error"),
    });

    await expect(client.getMemory("m0sk_secret", "mem-1")).rejects.toThrow(
      "Invalid API key",
    );
  });
});
