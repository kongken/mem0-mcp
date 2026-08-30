import { describe, expect, it } from "vitest";
import { loadConfig } from "../../src/config.js";

describe("loadConfig", () => {
  it("loads required settings", () => {
    const config = loadConfig({
      MEM0_API_URL: "http://mem0:8888/",
      MEM0_DEFAULT_USER_ID: "alice",
      PORT: "9090",
      MCP_HTTP_PATH: "/mcp",
      MCP_STATELESS: "true",
      LOG_LEVEL: "debug",
      MCP_ALLOWED_HOSTS: "localhost,mem0.example",
    });

    expect(config.port).toBe(9090);
    expect(config.mcpPath).toBe("/mcp");
    expect(config.defaultUserId).toBe("alice");
    expect(config.mem0ApiUrl.origin).toBe("http://mem0:8888");
    expect(config.stateless).toBe(true);
    expect(config.logLevel).toBe("debug");
    expect(config.allowedHosts).toEqual(["localhost", "mem0.example"]);
  });

  it("throws when required env vars are missing", () => {
    expect(() => loadConfig({})).toThrow("MEM0_API_URL is required");
    expect(() =>
      loadConfig({ MEM0_API_URL: "http://localhost:8888" }),
    ).toThrow("MEM0_DEFAULT_USER_ID is required");
  });
});
