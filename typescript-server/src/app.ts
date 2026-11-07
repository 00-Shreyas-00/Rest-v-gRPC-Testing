// Builds the Fastify application and the configured data source.

import Fastify, { type FastifyError, type FastifyInstance } from 'fastify';
import type pg from 'pg';
import type { Config } from './config.ts';
import type { Logger } from './log.ts';
import { MemoryRepository } from './repository/memory.ts';
import { PostgresRepository, createPool } from './repository/postgres.ts';
import { registerProductRoutes, sendJSON } from './routes/products.ts';
import { ProductService } from './service/service.ts';
import { registerX2Routes } from './x2/routes.ts';

/** Same request body limit as the Go server. */
export const MAX_BODY_BYTES = 1 << 20;

export interface AppOptions {
  /** null with DATA_SOURCE=none: experiment-2 routes only. */
  service: ProductService | null;
  /** Resolves when the data source can serve requests. Only used by GET /health. */
  health: () => Promise<void>;
  log: Logger;
  logRequests: boolean;
}

export function buildApp({ service, health, log, logRequests }: AppOptions): FastifyInstance {
  const app = Fastify({
    logger: false, // no per-request logger work; see LOG_REQUESTS below
    bodyLimit: MAX_BODY_BYTES,
    keepAliveTimeout: 120_000, // matches Go IdleTimeout
    routerOptions: {
      maxParamLength: 16_384, // effectively unlimited, like Go: long ids get a 400, not a 404
    },
  });

  // The Go server reads bodies regardless of Content-Type, so accept any type
  // as a raw buffer and decode JSON in the route (decodeInput).
  app.removeAllContentTypeParsers();
  app.addContentTypeParser('*', { parseAs: 'buffer' }, (_req, body, done) => done(null, body));

  app.get('/health', async (_req, reply) => {
    try {
      await health();
      return sendJSON(reply, 200, { status: 'ok' });
    } catch (err) {
      log.warn('health check failed', { err: String(err) });
      return sendJSON(reply, 503, { status: 'unavailable' });
    }
  });

  if (service) registerProductRoutes(app, service, log);
  registerX2Routes(app); // experiment 2: /x2/...

  // Unknown paths and unsupported methods: JSON 404, as in the Go server.
  app.setNotFoundHandler((_req, reply) => sendJSON(reply, 404, { error: 'not found' }));

  app.setErrorHandler((err: FastifyError, _req, reply) => {
    if (err.code === 'FST_ERR_CTP_BODY_TOO_LARGE') {
      return sendJSON(reply, 413, { error: 'request body too large' });
    }
    if (err.statusCode !== undefined && err.statusCode >= 400 && err.statusCode < 500) {
      return sendJSON(reply, 400, { error: 'invalid request body' });
    }
    log.error('request failed', { err: String(err) });
    return sendJSON(reply, 500, { error: 'internal server error' });
  });

  if (logRequests) {
    app.addHook('onResponse', async (req, reply) => {
      log.info('request', {
        method: req.method,
        path: req.url,
        status: reply.statusCode,
        duration_ms: reply.elapsedTime,
      });
    });
  }

  return app;
}

export interface DataSource {
  service: ProductService | null;
  health: () => Promise<void>;
  close: () => Promise<void>;
}

/**
 * postgres: requests query PostgreSQL through a pool of DB_MAX_CONNS.
 * none:     no product API, experiment-2 routes only.
 * memory:   the table is loaded once at startup, the pool is closed, and
 *           requests never touch the database.
 */
export async function openDataSource(cfg: Config, log: Logger): Promise<DataSource> {
  if (cfg.dataSource === 'none') {
    log.info('data source ready', { data_source: cfg.dataSource });
    return { service: null, health: async () => {}, close: async () => {} };
  }
  const pool: pg.Pool = createPool(cfg.databaseUrl, cfg.dbMaxConns);
  pool.on('error', (err) => log.error('idle database client error', { err: String(err) }));
  const ping = async () => {
    await pool.query('SELECT 1');
  };
  try {
    await ping();
  } catch (err) {
    await pool.end();
    throw new Error(`connect to database: ${String(err)}`);
  }
  const pgRepo = new PostgresRepository(pool);

  if (cfg.dataSource === 'memory') {
    const start = performance.now();
    let mem: MemoryRepository;
    try {
      mem = new MemoryRepository(await pgRepo.all());
    } finally {
      await pool.end();
    }
    log.info('data source ready', {
      data_source: cfg.dataSource,
      products: mem.size,
      load_ms: Math.round(performance.now() - start),
    });
    return { service: new ProductService(mem), health: async () => {}, close: async () => {} };
  }

  log.info('data source ready', { data_source: cfg.dataSource, max_conns: cfg.dbMaxConns });
  return { service: new ProductService(pgRepo), health: ping, close: () => pool.end() };
}
