import type { AppConfig } from "./config.js";

export const IDENTITY_KEYS = ["user_id", "agent_id", "run_id"] as const;
const UNSUPPORTED_IDENTITY_KEYS = ["agent_id", "run_id"] as const;

export type IdentityKey = (typeof IDENTITY_KEYS)[number];

export function rejectUnsupportedIdentityOverride(
  args: Record<string, unknown>,
): void {
  for (const key of UNSUPPORTED_IDENTITY_KEYS) {
    if (key in args && args[key] !== undefined) {
      throw new Error(`${key} is not supported by the MCP adapter`);
    }
  }
}

export function sanitizeSearchFilters(
  filters: Record<string, unknown> | undefined,
): Record<string, unknown> | undefined {
  if (!filters) {
    return undefined;
  }

  const identityKey = findIdentityKey(filters);
  if (identityKey) {
    throw new Error(
      `${identityKey} inside filters is not allowed; use the top-level user_id argument instead`,
    );
  }

  return Object.keys(filters).length > 0 ? filters : undefined;
}

export function scopedUserId(
  config: AppConfig,
  userId?: string,
): { user_id: string } {
  return { user_id: userId ?? config.defaultUserId };
}

function findIdentityKey(value: unknown): IdentityKey | undefined {
  if (Array.isArray(value)) {
    for (const item of value) {
      const key = findIdentityKey(item);
      if (key) {
        return key;
      }
    }
    return undefined;
  }

  if (typeof value !== "object" || value === null) {
    return undefined;
  }

  const record = value as Record<string, unknown>;
  for (const key of IDENTITY_KEYS) {
    if (key in record && record[key] !== undefined) {
      return key;
    }
  }

  for (const nested of Object.values(record)) {
    const key = findIdentityKey(nested);
    if (key) {
      return key;
    }
  }

  return undefined;
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
