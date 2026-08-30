import type { LogLevel } from "./config.js";

const LEVELS: Record<LogLevel, number> = {
  debug: 10,
  info: 20,
  warn: 30,
  error: 40,
};

const SENSITIVE_PATTERNS = [
  /m0sk_[a-zA-Z0-9_-]+/g,
  /Bearer\s+[A-Za-z0-9._-]+/gi,
  /("x-api-key"\s*:\s*")[^"]+/gi,
];

function redact(value: string): string {
  let result = value;
  for (const pattern of SENSITIVE_PATTERNS) {
    result = result.replace(pattern, "[REDACTED]");
  }
  return result;
}

function serialize(value: unknown): string {
  if (typeof value === "string") {
    return redact(value);
  }
  try {
    return redact(JSON.stringify(value));
  } catch {
    return "[unserializable]";
  }
}

export interface Logger {
  debug(message: string, fields?: Record<string, unknown>): void;
  info(message: string, fields?: Record<string, unknown>): void;
  warn(message: string, fields?: Record<string, unknown>): void;
  error(message: string, fields?: Record<string, unknown>): void;
}

export function createLogger(level: LogLevel): Logger {
  const threshold = LEVELS[level];

  function write(
    logLevel: LogLevel,
    message: string,
    fields?: Record<string, unknown>,
  ): void {
    if (LEVELS[logLevel] < threshold) {
      return;
    }

    const payload = {
      ts: new Date().toISOString(),
      level: logLevel,
      msg: message,
      ...(fields ? { fields: sanitizeFields(fields) } : {}),
    };

    const line = JSON.stringify(payload);
    if (logLevel === "error") {
      console.error(line);
    } else if (logLevel === "warn") {
      console.warn(line);
    } else {
      console.log(line);
    }
  }

  return {
    debug: (message, fields) => write("debug", message, fields),
    info: (message, fields) => write("info", message, fields),
    warn: (message, fields) => write("warn", message, fields),
    error: (message, fields) => write("error", message, fields),
  };
}

function sanitizeFields(fields: Record<string, unknown>): Record<string, unknown> {
  const blocked = new Set([
    "apiKey",
    "api_key",
    "authorization",
    "x-api-key",
    "memory",
    "memories",
    "messages",
    "content",
    "text",
  ]);

  const sanitized: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(fields)) {
    if (blocked.has(key.toLowerCase())) {
      sanitized[key] = "[REDACTED]";
      continue;
    }
    if (typeof value === "string") {
      sanitized[key] = redact(value);
    } else if (typeof value === "number" || typeof value === "boolean") {
      sanitized[key] = value;
    } else {
      sanitized[key] = "[REDACTED]";
    }
  }
  return sanitized;
}
