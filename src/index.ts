import { loadConfig } from "./config.js";
import { createHttpServer } from "./http/server.js";
import { createLogger } from "./logger.js";

async function main(): Promise<void> {
  const config = loadConfig();
  const logger = createLogger(config.logLevel);
  const server = createHttpServer(config, logger);

  const shutdown = async (signal: string) => {
    logger.info("shutting down", { signal });
    await server.close();
    process.exit(0);
  };

  process.on("SIGINT", () => void shutdown("SIGINT"));
  process.on("SIGTERM", () => void shutdown("SIGTERM"));

  await server.start();
}

main().catch((error) => {
  console.error(
    JSON.stringify({
      ts: new Date().toISOString(),
      level: "error",
      msg: "startup failed",
      error: error instanceof Error ? error.message : "unknown",
    }),
  );
  process.exit(1);
});
