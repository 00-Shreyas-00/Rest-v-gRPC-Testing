// Experiment 2 REST routes (large payloads and streaming). Same contract as
// go-server/internal/x2/rest.go; see proto/x2.proto for the endpoint list.

import { once } from 'node:events';
import type { ServerResponse } from 'node:http';
import type { Readable } from 'node:stream';
import { StringDecoder } from 'node:string_decoder';
import { promisify } from 'node:util';
import zlib from 'node:zlib';
import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { EXPORT_SIZE, MAX_RATE, POOL_SIZE, PROFILES, eventFor, getData, nowUs, type Item, type Profile } from './data.ts';

/** Import body limit, same as the Go server. */
export const MAX_IMPORT_BYTES = 64 << 20;
/** Write size of streamed NDJSON, same as the Go server. */
const NDJSON_CHUNK = 16 << 10;
/** A subscriber with this many events unsent is skipped (Go: 256-event queue). */
const SUBSCRIBER_BUFFER = 256;

const JSON_CONTENT_TYPE = 'application/json; charset=utf-8';
// Same gzip level as the Go servers (BestSpeed); see go-server/internal/x2/rest.go.
const GZIP_OPTIONS = { level: 1 };
const gzip = promisify(zlib.gzip);

type Query = Record<string, string | string[] | undefined>;

class BadRequest extends Error {}

function send(reply: FastifyReply, status: number, body: string | Buffer): FastifyReply {
  return reply.code(status).header('content-type', JSON_CONTENT_TYPE).send(body);
}

function badRequest(reply: FastifyReply, message: string): FastifyReply {
  return send(reply, 400, JSON.stringify({ error: message }));
}

/** First value; Go url.Values.Get semantics. */
function param(q: Query, name: string): string {
  const v = q[name];
  return (Array.isArray(v) ? v[0] : v) ?? '';
}

function intParam(raw: string, name: string, lo: number, hi: number): number {
  if (!/^[+-]?\d+$/.test(raw)) throw new BadRequest(`${name} must be an integer`);
  const n = Number(raw);
  if (n < lo || n > hi) throw new BadRequest(`${name} must be between ${lo} and ${hi}`);
  return n;
}

function acceptsGzip(req: FastifyRequest): boolean {
  const h = req.headers['accept-encoding'] ?? '';
  return h.split(',').some((part) => part.split(';')[0]?.trim() === 'gzip');
}

/** Serialize the whole list, optionally gzip it, send with Content-Length (as Go). */
async function sendItems(req: FastifyRequest, reply: FastifyReply, list: Item[], compress: boolean) {
  const body = JSON.stringify({ items: list });
  if (!compress || !acceptsGzip(req)) return send(reply, 200, body);
  const z = await gzip(body, GZIP_OPTIONS);
  reply.header('content-encoding', 'gzip').header('vary', 'Accept-Encoding');
  return send(reply, 200, z);
}

/** Write with backpressure; false if the client went away. */
async function write(res: ServerResponse, chunk: string): Promise<boolean> {
  if (res.destroyed) return false;
  if (!res.write(chunk)) {
    await Promise.race([once(res, 'drain'), once(res, 'close')]);
  }
  return !res.destroyed;
}

/** Splits a byte stream into lines, decoding UTF-8 across chunk boundaries. */
async function* lines(stream: Readable, limit: number): AsyncGenerator<string> {
  const dec = new StringDecoder('utf8');
  let rest = '';
  let bytes = 0;
  for await (const chunk of stream) {
    bytes += (chunk as Buffer).length;
    if (bytes > limit) throw new BadRequest('request body too large');
    rest += dec.write(chunk as Buffer);
    let start = 0;
    let i: number;
    while ((i = rest.indexOf('\n', start)) >= 0) {
      const line = rest.slice(start, i);
      start = i + 1;
      if (line.trim() !== '') yield line;
    }
    rest = rest.slice(start);
  }
  rest += dec.end();
  if (rest.trim() !== '') yield rest;
}

async function readAll(stream: Readable, limit: number): Promise<Buffer> {
  const chunks: Buffer[] = [];
  let n = 0;
  for await (const c of stream) {
    n += (c as Buffer).length;
    if (n > limit) throw new BadRequest('request body too large');
    chunks.push(c as Buffer);
  }
  return Buffer.concat(chunks);
}

interface Summary {
  count: number;
  id_sum: number;
  stock_sum: number;
}

/** Price feed: one timer per rate; every subscriber gets every event. */
class Hub {
  private readonly subs = new Set<ServerResponse>();
  private timer: NodeJS.Timeout | undefined;
  private rate = 0;
  private seq = 0;
  dropped = 0;

  add(res: ServerResponse, rate: number): boolean {
    if (this.subs.size > 0 && rate !== this.rate) return false;
    if (this.subs.size === 0) {
      this.rate = rate;
      this.timer = setInterval(() => this.tick(), 1000 / rate);
    }
    this.subs.add(res);
    return true;
  }

  remove(res: ServerResponse): void {
    if (this.subs.delete(res) && this.subs.size === 0) clearInterval(this.timer);
  }

  private tick(): void {
    const { small } = getData();
    this.seq++;
    const ev = { ...eventFor(this.seq, small), sent_at_us: nowUs() };
    const frame = `data: ${JSON.stringify(ev)}\n\n`; // encoded once, written to every subscriber
    const max = SUBSCRIBER_BUFFER * frame.length;
    for (const res of this.subs) {
      if (res.writableLength > max) this.dropped++;
      else res.write(frame);
    }
  }
}

export function registerX2Routes(app: FastifyInstance): void {
  const hub = new Hub();

  void app.register(async (x2) => {
    // Bodies arrive as raw streams; /x2/import parses them by content type.
    x2.removeAllContentTypeParsers();
    x2.addContentTypeParser('*', (_req, payload, done) => done(null, payload));

    x2.setErrorHandler((err, _req, reply) => {
      if (err instanceof BadRequest) return badRequest(reply, err.message);
      if (err instanceof SyntaxError) return badRequest(reply, 'invalid request body');
      return send(reply, 500, JSON.stringify({ error: 'internal server error' }));
    });

    x2.get('/x2/payload', async (req, reply) => {
      const q = req.query as Query;
      const profile = param(q, 'profile');
      if (!(PROFILES as readonly string[]).includes(profile)) throw new BadRequest('profile must be one of text, numeric, binary');
      const n = intParam(param(q, 'count'), 'count', 1, POOL_SIZE);
      return sendItems(req, reply, getData().profiles[profile as Profile].slice(0, n), true);
    });

    x2.get('/x2/export', async (req, reply) => {
      const q = req.query as Query;
      const n = intParam(param(q, 'count'), 'count', 1, EXPORT_SIZE);
      const format = param(q, 'format');
      if (format !== 'json' && format !== 'ndjson') throw new BadRequest('format must be json or ndjson');
      const small = getData().small;
      if (format === 'json') return sendItems(req, reply, small.slice(0, n), false);

      reply.hijack();
      const res = reply.raw;
      res.writeHead(200, { 'content-type': 'application/x-ndjson' });
      let chunk = '';
      for (let i = 0; i < n; i++) {
        chunk += JSON.stringify(small[i]) + '\n';
        if (chunk.length >= NDJSON_CHUNK) {
          if (!(await write(res, chunk))) return;
          chunk = '';
        }
      }
      if (chunk !== '' && !(await write(res, chunk))) return;
      res.end();
    });

    x2.post('/x2/import', async (req, reply) => {
      const ct = req.headers['content-type'] ?? '';
      const body = req.body as Readable;
      const sum: Summary = { count: 0, id_sum: 0, stock_sum: 0 };
      const add = (it: Item) => {
        sum.count++;
        sum.id_sum += it.id;
        sum.stock_sum += it.stock;
      };
      if (ct.startsWith('application/x-ndjson')) {
        for await (const line of lines(body, MAX_IMPORT_BYTES)) add(JSON.parse(line) as Item);
      } else if (ct.startsWith('application/json')) {
        const list = JSON.parse((await readAll(body, MAX_IMPORT_BYTES)).toString('utf8')) as { items: Item[] };
        for (const it of list.items) add(it);
      } else {
        throw new BadRequest('content-type must be application/json or application/x-ndjson');
      }
      return send(reply, 200, JSON.stringify(sum));
    });

    x2.get('/x2/quote/:id', async (req, reply) => {
      const raw = (req.params as { id?: string }).id ?? '';
      const id = /^[+-]?\d+$/.test(raw) ? Number(raw) : 0;
      if (id < 1 || id > EXPORT_SIZE) throw new BadRequest(`id must be between 1 and ${EXPORT_SIZE}`);
      const it = getData().small[id - 1] as Item;
      return send(reply, 200, JSON.stringify({ id: it.id, price: it.price, stock: it.stock }));
    });

    x2.get('/x2/feed', async (req, reply) => {
      const rate = intParam(param(req.query as Query, 'rate'), 'rate', 1, MAX_RATE);
      getData(); // build before the first tick
      reply.hijack();
      const res = reply.raw;
      if (!hub.add(res, rate)) {
        res.writeHead(409, { 'content-type': JSON_CONTENT_TYPE });
        res.end(JSON.stringify({ error: 'a feed with a different rate is running' }));
        return;
      }
      res.on('close', () => hub.remove(res));
      res.writeHead(200, { 'content-type': 'text/event-stream', 'cache-control': 'no-cache' });
      res.flushHeaders();
    });
  });
}
