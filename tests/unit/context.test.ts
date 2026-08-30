import { describe, expect, it } from "vitest";
import { readApiKeyFromHeaders } from "../../src/context.js";

describe("readApiKeyFromHeaders", () => {
  it("reads lowercase header", () => {
    expect(readApiKeyFromHeaders({ "x-api-key": "m0sk_test" })).toBe("m0sk_test");
  });

  it("returns undefined when missing", () => {
    expect(readApiKeyFromHeaders({})).toBeUndefined();
  });
});
