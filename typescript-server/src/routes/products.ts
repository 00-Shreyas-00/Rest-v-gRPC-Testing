// REST routes. Same wire contract as go-server/internal/rest/handler.go
// (see README "REST API contract"); only HTTP <-> service translation here.

import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { NotFoundError, type ProductInput } from '../product/product.ts';
import {
  DEFAULT_LIST_LIMIT,
  DEFAULT_SEARCH_LIMIT,
  ID_MESSAGE,
  ValidationError,
  type FieldError,
} from '../product/validation.ts';
import type { ProductService } from '../service/service.ts';
import type { Logger } from '../log.ts';

const JSON_CONTENT_TYPE = 'application/json; charset=utf-8';

type Query = Record<string, string | string[] | undefined>;

/** Sends a compact JSON body (JSON.stringify, no response schema). */
export function sendJSON(reply: FastifyReply, status: number, body: unknown): FastifyReply {
  return reply.code(status).header('content-type', JSON_CONTENT_TYPE).send(JSON.stringify(body));
}

function sendValidation(reply: FastifyReply, fields: FieldError[]): FastifyReply {
  return sendJSON(reply, 400, { error: 'validation failed', fields });
}

const INTEGER = /^[+-]?[0-9]+$/;

/**
 * Parses the {id} path segment. Same accepted syntax as Go strconv.ParseInt
 * (optional sign, ASCII digits); anything else gets the id validation error.
 */
function pathId(params: unknown): number {
  const raw = (params as { id?: string }).id ?? '';
  return INTEGER.test(raw) ? Number(raw) : 0;
}

/**
 * Fastify matches "/products/" with an empty :id, but Go's router does not
 * match it at all; treat it as an unknown route.
 */
function emptyId(params: unknown): boolean {
  return ((params as { id?: string }).id ?? '') === '';
}

const routeNotFound = (reply: FastifyReply) => sendJSON(reply, 404, { error: 'not found' });

/** First value of a query parameter (Go url.Values.Get semantics). */
function first(v: string | string[] | undefined): string | undefined {
  return Array.isArray(v) ? v[0] : v;
}

/**
 * Integer query parameters. Absent -> default. Not an integer (including
 * empty) -> "must be an integer". Huge integers become out-of-range numbers
 * and fail range validation, as in Go.
 */
class ParamErrors {
  readonly fields: FieldError[] = [];

  int(query: Query, name: string, def: number): number {
    const s = first(query[name]);
    if (s === undefined) return def;
    if (!INTEGER.test(s)) {
      this.fields.push({ field: name, message: 'must be an integer' });
      return def;
    }
    return Number(s);
  }
}

const STRING_FIELDS = new Set(['name', 'description', 'category']);

/**
 * Decodes a create/update body with the same rules as the Go server: a single
 * JSON object (null counts as {}), no unknown fields, correct JSON types,
 * stock a 32-bit integer; absent or null fields keep their zero value.
 * Returns null when the body is invalid.
 */
export function decodeInput(raw: unknown): ProductInput | null {
  if (!Buffer.isBuffer(raw) || raw.length === 0) return null;
  let body: unknown;
  try {
    body = JSON.parse(raw.toString('utf8'));
  } catch {
    return null;
  }
  if (body === null) body = {};
  if (typeof body !== 'object' || body === null || Array.isArray(body)) return null;

  const input: ProductInput = { name: '', description: '', category: '', price: 0, stock: 0, rating: 0 };
  for (const [key, value] of Object.entries(body)) {
    if (STRING_FIELDS.has(key)) {
      if (value === null) continue;
      if (typeof value !== 'string') return null;
      input[key as 'name' | 'description' | 'category'] = value;
    } else if (key === 'price' || key === 'rating') {
      if (value === null) continue;
      if (typeof value !== 'number' || !Number.isFinite(value)) return null;
      input[key] = value;
    } else if (key === 'stock') {
      if (value === null) continue;
      if (!Number.isInteger(value) || (value as number) < -2147483648 || (value as number) > 2147483647) return null;
      input.stock = value as number;
    } else {
      return null; // unknown field
    }
  }
  return input;
}

export function registerProductRoutes(app: FastifyInstance, svc: ProductService, log: Logger): void {
  const handleError = (reply: FastifyReply, err: unknown): FastifyReply => {
    if (err instanceof ValidationError) return sendValidation(reply, err.fields);
    if (err instanceof NotFoundError) return sendJSON(reply, 404, { error: 'product not found' });
    log.error('request failed', { err: String(err) });
    return sendJSON(reply, 500, { error: 'internal server error' });
  };

  const invalidId = (reply: FastifyReply) => sendValidation(reply, [{ field: 'id', message: ID_MESSAGE }]);
  const invalidBody = (reply: FastifyReply) => sendJSON(reply, 400, { error: 'invalid request body' });

  app.get('/products', async (req: FastifyRequest, reply) => {
    const pe = new ParamErrors();
    const query = req.query as Query;
    const limit = pe.int(query, 'limit', DEFAULT_LIST_LIMIT);
    const offset = pe.int(query, 'offset', 0);
    if (pe.fields.length > 0) return sendValidation(reply, pe.fields);
    try {
      return sendJSON(reply, 200, { products: await svc.list(limit, offset) });
    } catch (err) {
      return handleError(reply, err);
    }
  });

  app.get('/products/search', async (req, reply) => {
    const pe = new ParamErrors();
    const query = req.query as Query;
    const limit = pe.int(query, 'limit', DEFAULT_SEARCH_LIMIT);
    if (pe.fields.length > 0) return sendValidation(reply, pe.fields);
    try {
      return sendJSON(reply, 200, { products: await svc.search(first(query.q) ?? '', limit) });
    } catch (err) {
      return handleError(reply, err);
    }
  });

  app.get('/products/:id', async (req, reply) => {
    if (emptyId(req.params)) return routeNotFound(reply);
    try {
      return sendJSON(reply, 200, await svc.get(pathId(req.params)));
    } catch (err) {
      return handleError(reply, err);
    }
  });

  app.post('/products', async (req, reply) => {
    const input = decodeInput(req.body);
    if (input === null) return invalidBody(reply);
    try {
      return sendJSON(reply, 201, await svc.create(input));
    } catch (err) {
      return handleError(reply, err);
    }
  });

  app.put('/products/:id', async (req, reply) => {
    if (emptyId(req.params)) return routeNotFound(reply);
    // An invalid id is reported before the body is decoded, as in Go.
    const id = pathId(req.params);
    if (id < 1 || id > Number.MAX_SAFE_INTEGER) return invalidId(reply);
    const input = decodeInput(req.body);
    if (input === null) return invalidBody(reply);
    try {
      return sendJSON(reply, 200, await svc.update(id, input));
    } catch (err) {
      return handleError(reply, err);
    }
  });

  app.delete('/products/:id', async (req, reply) => {
    if (emptyId(req.params)) return routeNotFound(reply);
    try {
      await svc.delete(pathId(req.params));
      return reply.code(204).send();
    } catch (err) {
      return handleError(reply, err);
    }
  });
}
