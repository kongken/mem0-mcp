import type { AppConfig } from "./config.js";

export const IDENTITY_KEYS = ["user_id", "agent_id", "run_id"] as const;

export type IdentityKey = (typeof IDENTITY_KEYS)[number];

export function rejectIdentityOverride(args: Record<string, unknown>): void {
  for (const key of IDENTITY_KEYS) {
    if (key in args && args[key] !== undefined) {
      throw new Error(
        `${key} is managed by the MCP adapter and cannot be set in tool arguments`,
      );
    }
  }
}

export function sanitizeSearchFilters(
  filters: Record<string, unknown> | undefined,
): Record<string, unknown> | undefined {
  if (!filters) {
    return undefined;
  }

  for (const key of IDENTITY_KEYS) {
    if (key in filters && filters[key] !== undefined) {
      throw new Error(
        `${key} inside filters is managed by the MCP adapter and cannot be set in tool arguments`,
      );
    }
  }

  return Object.keys(filters).length > 0 ? filters : undefined;
}

export function scopedUserId(config: AppConfig): { user_id: string } {
  return { user_id: config.defaultUserId };
}

export function filterEntitiesForUser(
  entities: unknown,
  userId: string,
): unknown {
  if (!Array.isArray(entities)) {
    return entities;
  }

  return entities.filter((entity) => {
    if (typeof entity !== "object" || entity === null) {
      return false;
    }

    const record = entity as { type?: unknown; id?: unknown };
    return record.type === "user" && record.id === userId;
  });
}
