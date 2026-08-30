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
