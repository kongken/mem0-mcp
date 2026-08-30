import { describe, expect, it } from "vitest";
import { createLogger } from "../../src/logger.js";

describe("createLogger", () => {
  it("redacts api keys from log output", () => {
    const lines: string[] = [];
    const original = console.log;
    console.log = (line?: unknown) => {
      lines.push(String(line));
    };

    try {
      const logger = createLogger("info");
      logger.info("request", { apiKey: "m0sk_secret123", method: "GET" });
    } finally {
      console.log = original;
    }

    expect(lines).toHaveLength(1);
    const payload = JSON.parse(lines[0]!);
    expect(payload.fields.apiKey).toBe("[REDACTED]");
    expect(JSON.stringify(payload)).not.toContain("m0sk_secret123");
  });
});
