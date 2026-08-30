import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { AppConfig } from "../config.js";
import { getRequestApiKey } from "../context.js";
import { jsonResponse, toToolError } from "../errors.js";
import {
  filterEntitiesForUser,
  rejectIdentityOverride,
  sanitizeSearchFilters,
  scopedUserId,
} from "../identity.js";
import type { Mem0Client } from "../mem0/client.js";

const messageSchema = z.object({
  role: z.string().min(1),
  content: z.string().min(1),
});

const metadataSchema = z.record(z.unknown()).optional();

const MAX_TOP_K = 1000;

type ToolHandler<TArgs extends Record<string, unknown>> = (
  apiKey: string,
  args: TArgs,
  scope: { user_id: string },
) => Promise<unknown>;

function withScopedTool<TArgs extends Record<string, unknown>>(
  config: AppConfig,
  handler: ToolHandler<TArgs>,
) {
  return async (args: TArgs) => {
    try {
      const apiKey = getRequestApiKey();
      rejectIdentityOverride(args);
      const result = await handler(apiKey, args, scopedUserId(config));
      return jsonResponse(result);
    } catch (error) {
      return toToolError(error);
    }
  };
}

function withAuthenticatedTool<TArgs extends Record<string, unknown>>(
  handler: (apiKey: string, args: TArgs) => Promise<unknown>,
) {
  return async (args: TArgs) => {
    try {
      const apiKey = getRequestApiKey();
      rejectIdentityOverride(args);
      const result = await handler(apiKey, args);
      return jsonResponse(result);
    } catch (error) {
      return toToolError(error);
    }
  };
}

export function registerMem0Tools(
  server: McpServer,
  client: Mem0Client,
  config: AppConfig,
): void {
  server.registerTool(
    "add_memory",
    {
      title: "Add memory",
      description:
        "Store conversation messages as memories for the configured user identity.",
      inputSchema: {
        messages: z
          .array(messageSchema)
          .min(1)
          .describe("Messages to store as memory input."),
        metadata: metadataSchema.describe("Optional metadata to attach."),
        infer: z
          .boolean()
          .optional()
          .describe("Whether Mem0 should infer facts from messages."),
      },
    },
    withScopedTool(config, async (apiKey, args, scope) =>
      client.addMemory(apiKey, {
        messages: args.messages,
        metadata: args.metadata,
        infer: args.infer,
        ...scope,
      }),
    ),
  );

  server.registerTool(
    "search_memories",
    {
      title: "Search memories",
      description: "Semantic search across memories for the configured user.",
      inputSchema: {
        query: z.string().min(1).describe("Natural language search query."),
        top_k: z
          .number()
          .int()
          .positive()
          .max(100)
          .optional()
          .describe("Maximum number of results (maps to Mem0 top_k)."),
        filters: metadataSchema.describe(
          "Optional Mem0 search filters. Identity keys are not allowed.",
        ),
      },
    },
    withScopedTool(config, async (apiKey, args, scope) =>
      client.searchMemories(apiKey, {
        query: args.query,
        top_k: args.top_k,
        filters: sanitizeSearchFilters(args.filters),
        ...scope,
      }),
    ),
  );

  server.registerTool(
    "get_memories",
    {
      title: "Get memories",
      description:
        "List memories for the configured user. Mem0 OSS supports top_k only (no pagination).",
      inputSchema: {
        top_k: z
          .number()
          .int()
          .positive()
          .max(MAX_TOP_K)
          .optional()
          .describe("Maximum number of memories to return."),
      },
    },
    withScopedTool(config, async (apiKey, args, scope) =>
      client.getMemories(apiKey, {
        top_k: args.top_k,
        ...scope,
      }),
    ),
  );

  server.registerTool(
    "get_memory",
    {
      title: "Get memory",
      description: "Retrieve a single memory by ID.",
      inputSchema: {
        memory_id: z.string().min(1).describe("Memory identifier."),
      },
    },
    withAuthenticatedTool(async (apiKey, args) =>
      client.getMemory(apiKey, args.memory_id),
    ),
  );

  server.registerTool(
    "update_memory",
    {
      title: "Update memory",
      description: "Update memory text and/or metadata by ID.",
      inputSchema: {
        memory_id: z.string().min(1).describe("Memory identifier."),
        text: z.string().optional().describe("Updated memory text."),
        metadata: metadataSchema.describe("Updated metadata."),
      },
    },
    withAuthenticatedTool(async (apiKey, args) =>
      client.updateMemory(apiKey, args.memory_id, {
        text: args.text,
        metadata: args.metadata,
      }),
    ),
  );

  server.registerTool(
    "delete_memory",
    {
      title: "Delete memory",
      description:
        "Delete a single memory by explicit memory_id. Requires a concrete ID.",
      inputSchema: {
        memory_id: z
          .string()
          .min(1)
          .describe("Required memory identifier to delete."),
      },
    },
    withAuthenticatedTool(async (apiKey, args) => {
      await client.deleteMemory(apiKey, args.memory_id);
      return {
        deleted: true,
        memory_id: args.memory_id,
        summary: `Deleted memory ${args.memory_id}`,
      };
    }),
  );

  server.registerTool(
    "list_entities",
    {
      title: "List entities",
      description:
        "List the configured user entity known to Mem0 with memory counts.",
      inputSchema: {},
    },
    withAuthenticatedTool(async (apiKey) => {
      const entities = await client.listEntities(apiKey);
      return filterEntitiesForUser(entities, config.defaultUserId);
    }),
  );
}
