import { randomUUID } from "node:crypto";
import type { Request, Response } from "express";
import { createMcpExpressApp } from "@modelcontextprotocol/sdk/server/express.js";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { isInitializeRequest } from "@modelcontextprotocol/sdk/types.js";
import type { AppConfig } from "../config.js";
import {
  API_KEY_HEADER,
  readApiKeyFromHeaders,
  requestContext,
} from "../context.js";
import { AuthenticationError } from "../errors.js";
import type { Logger } from "../logger.js";
import { Mem0Client } from "../mem0/client.js";
import { registerMem0Tools } from "../mcp/tools.js";

type TransportMap = Record<string, StreamableHTTPServerTransport>;

export interface HttpServer {
  app: ReturnType<typeof createMcpExpressApp>;
  start(): Promise<void>;
  close(): Promise<void>;
}

export function createHttpServer(
  config: AppConfig,
  logger: Logger,
): HttpServer {
  const app = createMcpExpressApp({
    host: config.host,
    ...(config.allowedHosts ? { allowedHosts: config.allowedHosts } : {}),
  });
  const client = new Mem0Client({ config, logger });
  const transports: TransportMap = {};

  app.get("/healthz", (_req, res) => {
    res.status(200).json({ status: "ok" });
  });

  app.all(config.mcpPath, async (req: Request, res: Response) => {
    const apiKey = readApiKeyFromHeaders(
      req.headers as Record<string, string | string[] | undefined>,
    );
    if (!apiKey) {
      sendAuthError(res);
      return;
    }

    await requestContext.run({ apiKey }, async () => {
      try {
        await handleMcpRequest(req, res, transports, client, config, logger);
      } catch (error) {
        logger.error("mcp request failed", {
          error: error instanceof Error ? error.message : "unknown",
        });
        if (!res.headersSent) {
          res.status(500).json({
            jsonrpc: "2.0",
            error: { code: -32603, message: "Internal server error" },
            id: null,
          });
        }
      }
    });
  });

  let server: ReturnType<typeof app.listen> | undefined;

  return {
    app,
    async start() {
      await new Promise<void>((resolve) => {
        server = app.listen(config.port, config.host, () => {
          logger.info("mem0-mcp listening", {
            host: config.host,
            port: config.port,
            path: config.mcpPath,
            mem0Origin: config.mem0ApiUrl.origin,
            stateless: config.stateless,
            allowedHosts: config.allowedHosts?.length ?? 0,
          });
          resolve();
        });
      });
    },
    async close() {
      for (const transport of Object.values(transports)) {
        await transport.close();
      }
      await new Promise<void>((resolve, reject) => {
        if (!server) {
          resolve();
          return;
        }
        server.close((error) => {
          if (error) {
            reject(error);
            return;
          }
          resolve();
        });
      });
    },
  };
}

async function handleMcpRequest(
  req: Request,
  res: Response,
  transports: TransportMap,
  client: Mem0Client,
  config: AppConfig,
  logger: Logger,
): Promise<void> {
  if (config.stateless) {
    const transport = createStatelessTransport(logger);
    const server = createMcpServer(client, config);
    await server.connect(transport);
    await transport.handleRequest(req, res, req.body);
    await transport.close();
    await server.close();
    return;
  }

  const sessionId = req.headers["mcp-session-id"] as string | undefined;
  let transport = sessionId ? transports[sessionId] : undefined;

  if (!transport && isInitializeRequest(req.body)) {
    transport = createStatefulTransport(logger, transports);
    const server = createMcpServer(client, config);
    await server.connect(transport);
  }

  if (!transport) {
    res.status(400).json({
      jsonrpc: "2.0",
      error: {
        code: -32000,
        message: "Bad Request: invalid or missing MCP session",
      },
      id: null,
    });
    return;
  }

  await transport.handleRequest(req, res, req.body);
}

function createStatelessTransport(logger: Logger): StreamableHTTPServerTransport {
  const transport = new StreamableHTTPServerTransport({
    sessionIdGenerator: undefined,
  });

  transport.onerror = (error) => {
    logger.error("transport error", { error: error.message });
  };

  return transport;
}

function createStatefulTransport(
  logger: Logger,
  transports: TransportMap,
): StreamableHTTPServerTransport {
  const transport = new StreamableHTTPServerTransport({
    sessionIdGenerator: () => randomUUID(),
    onsessioninitialized: (sessionId) => {
      transports[sessionId] = transport;
    },
  });

  transport.onclose = () => {
    const sessionId = transport.sessionId;
    if (sessionId) {
      delete transports[sessionId];
    }
  };

  transport.onerror = (error) => {
    logger.error("transport error", { error: error.message });
  };

  return transport;
}

function sendAuthError(res: Response): void {
  res.status(401).json({
    jsonrpc: "2.0",
    error: {
      code: -32001,
      message: new AuthenticationError().message,
    },
    id: null,
  });
}

function createMcpServer(client: Mem0Client, config: AppConfig): McpServer {
  const server = new McpServer(
    {
      name: "mem0-mcp",
      version: "0.1.0",
    },
    {
      instructions:
        "Self-hosted Mem0 OSS MCP adapter. Authentication uses the HTTP X-API-Key header; memory scope is limited to the server-configured user identity.",
    },
  );

  registerMem0Tools(server, client, config);
  return server;
}

export { API_KEY_HEADER };
