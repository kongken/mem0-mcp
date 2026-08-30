import { describe, expect, it } from "vitest";
import {
  filterEntitiesForUser,
  rejectIdentityOverride,
  sanitizeSearchFilters,
} from "../../src/identity.js";

describe("rejectIdentityOverride", () => {
  it("rejects top-level identity keys", () => {
    expect(() => rejectIdentityOverride({ user_id: "victim" })).toThrow(
      "user_id is managed by the MCP adapter",
    );
  });
});

describe("sanitizeSearchFilters", () => {
  it("rejects identity keys nested in filters", () => {
    expect(() =>
      sanitizeSearchFilters({ user_id: "victim", category: "work" }),
    ).toThrow("user_id inside filters is managed by the MCP adapter");
  });

  it("allows non-identity filters", () => {
    expect(sanitizeSearchFilters({ category: "work" })).toEqual({
      category: "work",
    });
  });

  it("returns undefined for empty filters", () => {
    expect(sanitizeSearchFilters({})).toBeUndefined();
  });
});

describe("filterEntitiesForUser", () => {
  it("returns only the configured user entity", () => {
    const entities = [
      { id: "alice", type: "user", total_memories: 2 },
      { id: "bob", type: "user", total_memories: 1 },
      { id: "agent-1", type: "agent", total_memories: 3 },
    ];

    expect(filterEntitiesForUser(entities, "alice")).toEqual([
      { id: "alice", type: "user", total_memories: 2 },
    ]);
  });
});
