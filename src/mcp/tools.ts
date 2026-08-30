import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { AppConfig } from "../config.js";
import { getRequestApiKey } from "../context.js";
import { jsonResponse, toToolError } from "../errors.js";
import type { Mem0Client } from "../mem0/client.js";

const messageSchema = z.object({
  role: z.string().min(1),
  content: z.string().min(1),
});

const metadataSchema = z.record(z.unknown()).optional();

function rejectIdentityOverride(args: Record<string, unknown>): void {
  for (const key of ["user_id", "agent_id", "run_id"]) {
    if (key in args && args[key] !== undefined) {
      throw new Error(
        `${key} is managed by the MCP adapter and cannot be set in tool arguments`,
      );
    }
  }
}

function withScopedUserId(
  config: AppConfig,
  args: Record<string, unknown>,
): { user_id: string } {
  rejectIdentityOverride(args);
  return { user_id: config.defaultUserId };
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
    async (args) => {
      try {
        const apiKey = getRequestApiKey();
        const scope = withScopedUserId(config, args);
        const result = await client.addMemory(apiKey, {
          messages: args.messages,
          metadata: args.metadata,
          infer: args.infer,
          ...scope,
        });
        return jsonResponse(result);
      } catch (error) {
        return toToolError(error);
      }
    },
  );

  server.registerTool(
    "search_memories",
    {
      title: "Search memories",
      description: "Semantic search across memories for the configured user.",
      inputSchema: {
        query: z.string().min(1).describe("Natural language search query."),
        limit: z
          .number()
          .int()
          .positive()
          .max(100)
          .optional()
          .describe("Maximum number of results."),
        filters: metadataSchema.describe("Optional Mem0 search filters."),
      },
    },
    async (args) => {
      try {
        const apiKey = getRequestApiKey();
        const scope = withScopedUserId(config, args);
        const result = await client.searchMemories(apiKey, {
          query: args.query,
          limit: args.limit,
          filters: args.filters,
          ...scope,
        });
        return jsonResponse(result);
      } catch (error) {
        return toToolError(error);
      }
    },
  );

  server.registerTool(
    "get_memories",
    {
      title: "Get memories",
      description: "List memories for the configured user with optional pagination.",
      inputSchema: {
        page: z.number().int().positive().optional(),
        page_size: z.number().int().positive().max(100).optional(),
      },
    },
    async (args) => {
      try {
        const apiKey = getRequestApiKey();
        const scope = withScopedUserId(config, args);
        const result = await client.getMemories(apiKey, {
          page: args.page,
          page_size: args.page_size,
          ...scope,
        });
        return jsonResponse(result);
      } catch (error) {
        return toToolError(error);
      }
    },
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
    async (args) => {
      try {
        const apiKey = getRequestApiKey();
        rejectIdentityOverride(args);
        const result = await client.getMemory(apiKey, args.memory_id);
        return jsonResponse(result);
      } catch (error) {
        return toToolError(error);
      }
    },
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
    async (args) => {
      try {
        const apiKey = getRequestApiKey();
        rejectIdentityOverride(args);
        const result = await client.updateMemory(apiKey, args.memory_id, {
          text: args.text,
          metadata: args.metadata,
        });
        return jsonResponse(result);
      } catch (error) {
        return toToolError(error);
      }
    },
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
    async (args) => {
      try {
        const apiKey = getRequestApiKey();
        rejectIdentityOverride(args);
        await client.deleteMemory(apiKey, args.memory_id);
        return jsonResponse({
          deleted: true,
          memory_id: args.memory_id,
          summary: `Deleted memory ${args.memory_id}`,
        });
      } catch (error) {
        return toToolError(error);
      }
    },
  );

  server.registerTool(
    "list_entities",
    {
      title: "List entities",
      description:
        "List distinct user/agent/run entities known to Mem0 with memory counts.",
      inputSchema: {},
    },
    async (args) => {
      try {
        const apiKey = getRequestApiKey();
        rejectIdentityOverride(args);
        const result = await client.listEntities(apiKey);
        return jsonResponse(result);
      } catch (error) {
        return toToolError(error);
      }
    },
  );
}
