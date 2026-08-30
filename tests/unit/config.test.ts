import { describe, expect, it } from "vitest";
import { loadConfig, assertSameOrigin } from "../../src/config.js";

describe("loadConfig", () => {
  it("loads required settings", () => {
    const config = loadConfig({
      MEM0_API_URL: "http://mem0:8888/",
      MEM0_DEFAULT_USER_ID: "alice",
      PORT: "9090",
      MCP_HTTP_PATH: "/mcp",
      MCP_STATELESS: "true",
      LOG_LEVEL: "debug",
    });

    expect(config.port).toBe(9090);
    expect(config.mcpPath).toBe("/mcp");
    expect(config.defaultUserId).toBe("alice");
    expect(config.mem0ApiUrl.origin).toBe("http://mem0:8888");
    expect(config.stateless).toBe(true);
    expect(config.logLevel).toBe("debug");
  });

  it("throws when required env vars are missing", () => {
    expect(() => loadConfig({})).toThrow("MEM0_API_URL is required");
    expect(() =>
      loadConfig({ MEM0_API_URL: "http://localhost:8888" }),
    ).toThrow("MEM0_DEFAULT_USER_ID is required");
  });
});

describe("assertSameOrigin", () => {
  it("allows requests to configured origin", () => {
    const base = new URL("http://mem0:8888");
    expect(() =>
      assertSameOrigin(new URL("http://mem0:8888/memories", base), base),
    ).not.toThrow();
  });

  it("rejects other origins", () => {
    const base = new URL("http://mem0:8888");
    expect(() =>
      assertSameOrigin(new URL("http://evil.example/memories"), base),
    ).toThrow("unexpected origin");
  });
});
