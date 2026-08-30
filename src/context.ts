import { AsyncLocalStorage } from "node:async_hooks";

export interface RequestContext {
  apiKey: string;
}

export const requestContext = new AsyncLocalStorage<RequestContext>();

export function getRequestApiKey(): string {
  const ctx = requestContext.getStore();
  if (!ctx?.apiKey) {
    throw new Error("Missing request authentication context");
  }
  return ctx.apiKey;
}

export const API_KEY_HEADER = "x-api-key";

export function readApiKeyFromHeaders(
  headers: Record<string, string | string[] | undefined>,
): string | undefined {
  const raw = headers[API_KEY_HEADER];
  if (Array.isArray(raw)) {
    return raw[0]?.trim() || undefined;
  }
  return raw?.trim() || undefined;
}
