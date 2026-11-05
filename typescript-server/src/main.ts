// Entry point: TypeScript REST/JSON server for the Product API.
// Configuration: DATA_SOURCE, DATABASE_URL, DB_MAX_CONNS, HTTP_PORT, LOG_LEVEL,
// LOG_REQUESTS, METRICS_PORT.

import os from 'node:os';
import { buildApp, openDataSource } from './app.ts';
import { loadConfig } from './config.ts';
import { createLogger } from './log.ts';
import { serveMetrics } from './monitoring.ts';

async function main(): Promise<void> {
  const cfg = loadConfig();
  const log = createLogger(cfg.logLevel);
  const ds = await openDataSource(cfg, log);
  const app = buildApp({ service: ds.service, health: ds.health, log, logRequests: cfg.logRequests });

  // Runtime metrics on their own port, off the benchmarked request path.
  const metrics = await serveMetrics(cfg.metricsPort, log);

  await app.listen({ host: '0.0.0.0', port: cfg.httpPort });
  log.info('typescript-server listening', {
    addr: `:${cfg.httpPort}`,
    data_source: cfg.dataSource,
    node_version: process.version,
    available_parallelism: os.availableParallelism(),
    log_requests: cfg.logRequests,
  });

  let stopping = false;
  const shutdown = async (signal: string) => {
    if (stopping) return;
    stopping = true;
    log.info('shutting down', { signal });
    const timer = setTimeout(() => process.exit(1), 10_000);
    timer.unref();
    await app.close();
    await metrics.close();
    await ds.close();
    process.exit(0);
  };
  process.on('SIGINT', () => void shutdown('SIGINT'));
  process.on('SIGTERM', () => void shutdown('SIGTERM'));
}

main().catch((err: unknown) => {
  process.stderr.write(`typescript-server: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(1);
});
