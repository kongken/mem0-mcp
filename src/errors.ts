export class Mem0McpError extends Error {
  readonly code: string;
  readonly status?: number;

  constructor(message: string, code: string, status?: number) {
    super(message);
    this.name = "Mem0McpError";
    this.code = code;
    this.status = status;
  }
}

export class AuthenticationError extends Mem0McpError {
  constructor(message = "Authentication required: provide X-API-Key header") {
    super(message, "AUTHENTICATION_REQUIRED", 401);
    this.name = "AuthenticationError";
  }
}

export class UpstreamError extends Mem0McpError {
  constructor(message: string, status?: number) {
    super(message, "UPSTREAM_ERROR", status);
    this.name = "UpstreamError";
  }
}

export function mapUpstreamStatus(status: number): string {
  switch (status) {
    case 401:
      return "Mem0 rejected the API key (401 Unauthorized)";
    case 403:
      return "Mem0 denied access to this resource (403 Forbidden)";
    case 404:
      return "Mem0 resource was not found (404 Not Found)";
    case 429:
      return "Mem0 rate limit exceeded (429 Too Many Requests)";
    default:
      if (status >= 500) {
        return `Mem0 server error (${status})`;
      }
      return `Mem0 request failed (${status})`;
  }
}

export function toToolError(error: unknown): {
  content: Array<{ type: "text"; text: string }>;
  isError: true;
} {
  if (error instanceof Mem0McpError) {
    return {
      content: [{ type: "text", text: error.message }],
      isError: true,
    };
  }

  if (error instanceof Error) {
    return {
      content: [{ type: "text", text: error.message }],
      isError: true,
    };
  }

  return {
    content: [{ type: "text", text: "Unexpected error" }],
    isError: true,
  };
}

export function jsonResponse(data: unknown): {
  content: Array<{ type: "text"; text: string }>;
} {
  return {
    content: [{ type: "text", text: JSON.stringify(data, null, 2) }],
  };
}
