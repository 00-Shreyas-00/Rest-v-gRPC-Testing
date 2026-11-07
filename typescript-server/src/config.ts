// Server configuration from environment variables. Same variables and defaults
// as the Go servers (go-server/internal/config).

export type DataSource = 'postgres' | 'memory' | 'none';
export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

export interface Config {
  dataSource: DataSource; // DATA_SOURCE
  databaseUrl: string; // DATABASE_URL
  dbMaxConns: number; // DB_MAX_CONNS: pool size, identical across implementations
  httpPort: number; // HTTP_PORT
  metricsPort: number; // METRICS_PORT: runtime metrics listener, 0 disables
  logLevel: LogLevel; // LOG_LEVEL
  logRequests: boolean; // LOG_REQUESTS: per-request logging; keep off when benchmarking
}

export function loadConfig(env: NodeJS.ProcessEnv = process.env): Config {
  const errors: string[] = [];
  const get = (key: string, def: string): string => {
    const v = env[key]?.trim();
    return v ? v : def;
  };
  const intVar = (key: string, def: number, min: number, max: number): number => {
    const s = get(key, String(def));
    const n = /^[+-]?\d+$/.test(s) ? Number(s) : NaN;
    if (!Number.isInteger(n) || n < min || n > max) {
      errors.push(`${key} must be an integer between ${min} and ${max}, got "${s}"`);
      return def;
    }
    return n;
  };

  const dataSource = get('DATA_SOURCE', 'postgres').toLowerCase();
  if (dataSource !== 'postgres' && dataSource !== 'memory' && dataSource !== 'none') {
    errors.push(`DATA_SOURCE must be "postgres", "memory" or "none", got "${dataSource}"`);
  }
  const logLevel = get('LOG_LEVEL', 'info').toLowerCase();
  if (!['debug', 'info', 'warn', 'error'].includes(logLevel)) {
    errors.push(`LOG_LEVEL must be debug, info, warn or error, got "${logLevel}"`);
  }
  const logRequestsRaw = get('LOG_REQUESTS', 'false').toLowerCase();
  if (!['true', '1', 'false', '0'].includes(logRequestsRaw)) {
    errors.push(`LOG_REQUESTS must be true or false, got "${logRequestsRaw}"`);
  }

  const cfg: Config = {
    dataSource: dataSource as DataSource,
    databaseUrl: get('DATABASE_URL', 'postgres://bench:bench@localhost:5432/products'),
    dbMaxConns: intVar('DB_MAX_CONNS', 20, 1, 1000),
    httpPort: intVar('HTTP_PORT', 8080, 1, 65535),
    metricsPort: intVar('METRICS_PORT', 9100, 0, 65535),
    logLevel: logLevel as LogLevel,
    logRequests: logRequestsRaw === 'true' || logRequestsRaw === '1',
  };
  if (errors.length > 0) {
    throw new Error(`invalid configuration: ${errors.join('; ')}`);
  }
  return cfg;
}
