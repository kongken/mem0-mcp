import { assertSameOrigin, type AppConfig } from "../config.js";
import { UpstreamError, mapUpstreamStatus } from "../errors.js";
import type { Logger } from "../logger.js";

export interface Mem0Message {
  role: string;
  content: string;
}

export interface Mem0ClientOptions {
  config: AppConfig;
  logger: Logger;
}

export class Mem0Client {
  private readonly baseUrl: URL;
  private readonly timeoutMs: number;
  private readonly logger: Logger;

  constructor(options: Mem0ClientOptions) {
    this.baseUrl = options.config.mem0ApiUrl;
    this.timeoutMs = options.config.requestTimeoutMs;
    this.logger = options.logger;
  }

  async addMemory(
    apiKey: string,
    body: {
      messages: Mem0Message[];
      user_id: string;
      metadata?: Record<string, unknown>;
      agent_id?: string;
      run_id?: string;
      infer?: boolean;
    },
  ): Promise<unknown> {
    return this.request(apiKey, "POST", "/memories", body);
  }

  async searchMemories(
    apiKey: string,
    body: {
      query: string;
      user_id: string;
      limit?: number;
      filters?: Record<string, unknown>;
    },
  ): Promise<unknown> {
    return this.request(apiKey, "POST", "/search", body);
  }

  async getMemories(
    apiKey: string,
    query: {
      user_id: string;
      page?: number;
      page_size?: number;
    },
  ): Promise<unknown> {
    const params = new URLSearchParams();
    params.set("user_id", query.user_id);
    if (query.page !== undefined) {
      params.set("page", String(query.page));
    }
    if (query.page_size !== undefined) {
      params.set("page_size", String(query.page_size));
    }
    return this.request(apiKey, "GET", `/memories?${params.toString()}`);
  }

  async getMemory(apiKey: string, memoryId: string): Promise<unknown> {
    return this.request(apiKey, "GET", `/memories/${encodeURIComponent(memoryId)}`);
  }

  async updateMemory(
    apiKey: string,
    memoryId: string,
    body: { text?: string; metadata?: Record<string, unknown> },
  ): Promise<unknown> {
    return this.request(
      apiKey,
      "PUT",
      `/memories/${encodeURIComponent(memoryId)}`,
      body,
    );
  }

  async deleteMemory(apiKey: string, memoryId: string): Promise<unknown> {
    return this.request(
      apiKey,
      "DELETE",
      `/memories/${encodeURIComponent(memoryId)}`,
    );
  }

  async listEntities(apiKey: string): Promise<unknown> {
    return this.request(apiKey, "GET", "/entities");
  }

  private async request(
    apiKey: string,
    method: string,
    path: string,
    body?: unknown,
  ): Promise<unknown> {
    const url = new URL(path, this.baseUrl);
    assertSameOrigin(url, this.baseUrl);

    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), this.timeoutMs);

    try {
      this.logger.debug("mem0 request", { method, path });

      const response = await fetch(url, {
        method,
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
          "X-API-Key": apiKey,
        },
        body: body === undefined ? undefined : JSON.stringify(body),
        signal: controller.signal,
        redirect: "manual",
      });

      if (response.status >= 300 && response.status < 400) {
        throw new UpstreamError(
          "Mem0 returned a redirect; refusing to follow redirects",
          response.status,
        );
      }

      const text = await response.text();
      const payload = text ? safeJsonParse(text) : undefined;

      if (!response.ok) {
        const detail =
          typeof payload === "object" &&
          payload !== null &&
          "detail" in payload &&
          typeof payload.detail === "string"
            ? payload.detail
            : mapUpstreamStatus(response.status);
        throw new UpstreamError(detail, response.status);
      }

      return payload;
    } catch (error) {
      if (error instanceof UpstreamError) {
        throw error;
      }
      if (error instanceof Error && error.name === "AbortError") {
        throw new UpstreamError(
          `Mem0 request timed out after ${this.timeoutMs}ms`,
        );
      }
      if (error instanceof TypeError) {
        throw new UpstreamError("Mem0 API is unreachable");
      }
      throw error;
    } finally {
      clearTimeout(timeout);
    }
  }
}

function safeJsonParse(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}
