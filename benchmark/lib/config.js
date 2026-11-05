// Benchmark configuration from environment variables (k6 -e NAME=value).
//
// Target selection (nothing is hard-coded to one implementation):
//   TARGET=go-rest | ts-rest | go-grpc   known compose services (default go-rest)
//   BASE_URL=http://host:port            any REST server (implies protocol rest)
//   GRPC_ADDR=host:port                  any gRPC server (implies protocol grpc)
//
// Load shape:
//   VUS=10        concurrent virtual users (closed model, no think time)
//   WARMUP=10s    warm-up phase, excluded from results (0s to skip)
//   DURATION=30s  measured phase
//   TIMEOUT=10s   per-request timeout; expiries are counted as timeouts

const TARGETS = {
  'go-rest': { protocol: 'rest', address: 'http://go-rest:8080' },
  'ts-rest': { protocol: 'rest', address: 'http://ts-rest:8080' },
  'go-grpc': { protocol: 'grpc', address: 'go-grpc:9090' },
};

function resolveTarget() {
  if (__ENV.BASE_URL) {
    return { name: __ENV.TARGET || __ENV.BASE_URL, protocol: 'rest', address: __ENV.BASE_URL.replace(/\/$/, '') };
  }
  if (__ENV.GRPC_ADDR) {
    return { name: __ENV.TARGET || __ENV.GRPC_ADDR, protocol: 'grpc', address: __ENV.GRPC_ADDR };
  }
  const name = __ENV.TARGET || 'go-rest';
  const known = TARGETS[name];
  if (!known) {
    throw new Error(`unknown TARGET "${name}"; use ${Object.keys(TARGETS).join(', ')}, BASE_URL or GRPC_ADDR`);
  }
  return { name, ...known };
}

/** Parses "500ms", "10s", "2m" into seconds. */
export function seconds(duration) {
  const m = /^(\d+(?:\.\d+)?)(ms|s|m)$/.exec(String(duration).trim());
  if (!m) throw new Error(`invalid duration "${duration}" (use e.g. 500ms, 10s, 2m)`);
  const n = Number(m[1]);
  return m[2] === 'ms' ? n / 1000 : m[2] === 'm' ? n * 60 : n;
}

function positiveInt(name, def) {
  const raw = __ENV[name];
  if (raw === undefined || raw === '') return def;
  const n = Number(raw);
  if (!Number.isInteger(n) || n < 1) throw new Error(`${name} must be a positive integer, got "${raw}"`);
  return n;
}

export const target = resolveTarget();

export const settings = {
  vus: positiveInt('VUS', 10),
  warmup: __ENV.WARMUP || '10s',
  duration: __ENV.DURATION || '30s',
  timeout: __ENV.TIMEOUT || '10s',
};
seconds(settings.warmup);
seconds(settings.duration);
seconds(settings.timeout);

export { positiveInt };
