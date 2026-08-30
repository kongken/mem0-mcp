import { describe, expect, it } from "vitest";
import {
  filterEntitiesForUser,
  rejectUnsupportedIdentityOverride,
  sanitizeSearchFilters,
  scopedUserId,
} from "../../src/identity.js";

describe("rejectUnsupportedIdentityOverride", () => {
  it("allows an explicit user_id", () => {
    expect(() =>
      rejectUnsupportedIdentityOverride({ user_id: "bob" }),
    ).not.toThrow();
  });

  it.each(["agent_id", "run_id"])("rejects unsupported %s", (key) => {
    expect(() =>
      rejectUnsupportedIdentityOverride({ [key]: "other-scope" }),
    ).toThrow(`${key} is not supported by the MCP adapter`);
  });
});

describe("scopedUserId", () => {
  const config = { defaultUserId: "alice" } as Parameters<
    typeof scopedUserId
  >[0];

  it("uses the configured user by default", () => {
    expect(scopedUserId(config)).toEqual({ user_id: "alice" });
  });

  it("uses an explicit user_id when provided", () => {
    expect(scopedUserId(config, "bob")).toEqual({ user_id: "bob" });
  });
});

describe("sanitizeSearchFilters", () => {
  it("rejects identity keys at the top level", () => {
    expect(() =>
      sanitizeSearchFilters({ user_id: "victim", category: "work" }),
    ).toThrow("user_id inside filters is not allowed");
  });

  it("rejects identity keys nested inside logical filters", () => {
    expect(() =>
      sanitizeSearchFilters({
        AND: [{ category: "work" }, { OR: [{ user_id: "victim" }] }],
      }),
    ).toThrow("user_id inside filters is not allowed");
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
