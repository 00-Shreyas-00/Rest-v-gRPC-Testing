// Minimal structured JSON logger, the counterpart of Go's slog JSON handler.
// Nothing is logged per request unless LOG_REQUESTS=true.

import type { LogLevel } from './config.ts';

const order: Record<LogLevel, number> = { debug: 0, info: 1, warn: 2, error: 3 };

export interface Logger {
  debug(msg: string, fields?: Record<string, unknown>): void;
  info(msg: string, fields?: Record<string, unknown>): void;
  warn(msg: string, fields?: Record<string, unknown>): void;
  error(msg: string, fields?: Record<string, unknown>): void;
}

export function createLogger(level: LogLevel, out: (line: string) => void = (l) => process.stdout.write(l)): Logger {
  const min = order[level];
  const write = (lvl: LogLevel) => (msg: string, fields?: Record<string, unknown>) => {
    if (order[lvl] < min) return;
    out(JSON.stringify({ time: new Date().toISOString(), level: lvl.toUpperCase(), msg, ...fields }) + '\n');
  };
  return { debug: write('debug'), info: write('info'), warn: write('warn'), error: write('error') };
}

export const silentLogger: Logger = createLogger('error', () => {});
