export type LogLevel = "debug" | "info" | "warn" | "error";

export interface AppConfig {
  host: string;
  port: number;
  mcpPath: string;
  mem0ApiUrl: URL;
  defaultUserId: string;
  requestTimeoutMs: number;
  stateless: boolean;
  logLevel: LogLevel;
}

const DEFAULT_PORT = 8080;
const DEFAULT_PATH = "/mcp";
const DEFAULT_TIMEOUT_MS = 30_000;

function parseBoolean(value: string | undefined, fallback: boolean): boolean {
  if (value === undefined) {
    return fallback;
  }
  return value === "1" || value.toLowerCase() === "true";
}

function parseLogLevel(value: string | undefined): LogLevel {
  switch (value?.toLowerCase()) {
    case "debug":
    case "info":
    case "warn":
    case "error":
      return value.toLowerCase() as LogLevel;
    default:
      return "info";
  }
}

export function loadConfig(env: NodeJS.ProcessEnv = process.env): AppConfig {
  const mem0ApiUrlRaw = env.MEM0_API_URL?.trim();
  if (!mem0ApiUrlRaw) {
    throw new Error("MEM0_API_URL is required");
  }

  let mem0ApiUrl: URL;
  try {
    mem0ApiUrl = new URL(mem0ApiUrlRaw);
  } catch {
    throw new Error("MEM0_API_URL must be a valid absolute URL");
  }

  if (!["http:", "https:"].includes(mem0ApiUrl.protocol)) {
    throw new Error("MEM0_API_URL must use http or https");
  }

  const defaultUserId = env.MEM0_DEFAULT_USER_ID?.trim();
  if (!defaultUserId) {
    throw new Error("MEM0_DEFAULT_USER_ID is required");
  }

  const port = Number.parseInt(env.PORT ?? String(DEFAULT_PORT), 10);
  if (!Number.isFinite(port) || port <= 0 || port > 65535) {
    throw new Error("PORT must be a valid TCP port");
  }

  const requestTimeoutMs = Number.parseInt(
    env.MEM0_REQUEST_TIMEOUT_MS ?? String(DEFAULT_TIMEOUT_MS),
    10,
  );
  if (!Number.isFinite(requestTimeoutMs) || requestTimeoutMs <= 0) {
    throw new Error("MEM0_REQUEST_TIMEOUT_MS must be a positive integer");
  }

  const mcpPath = env.MCP_HTTP_PATH?.trim() || DEFAULT_PATH;
  if (!mcpPath.startsWith("/")) {
    throw new Error("MCP_HTTP_PATH must start with /");
  }

  return {
    host: env.HOST?.trim() || "0.0.0.0",
    port,
    mcpPath,
    mem0ApiUrl,
    defaultUserId,
    requestTimeoutMs,
    stateless: parseBoolean(env.MCP_STATELESS, false),
    logLevel: parseLogLevel(env.LOG_LEVEL),
  };
}

export function assertSameOrigin(requestUrl: URL, configuredOrigin: URL): void {
  if (requestUrl.origin !== configuredOrigin.origin) {
    throw new Error(
      `Refusing to follow request to unexpected origin: ${requestUrl.origin}`,
    );
  }
}
