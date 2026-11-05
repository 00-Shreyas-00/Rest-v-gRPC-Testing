// HTTP behaviour. Mirrors go-server/internal/rest/handler_test.go, asserting
// the same status codes and byte-identical bodies.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { buildApp, MAX_BODY_BYTES } from '../src/app.ts';
import { silentLogger } from '../src/log.ts';
import { MemoryRepository } from '../src/repository/memory.ts';
import { ProductService } from '../src/service/service.ts';
import { product1JSON, seedProducts } from './fixtures.ts';

function newApp(health: () => Promise<void> = async () => {}) {
  const svc = new ProductService(new MemoryRepository(seedProducts()));
  return buildApp({ service: svc, health, log: silentLogger, logRequests: false });
}

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH';

async function call(app: ReturnType<typeof newApp>, method: Method, url: string, payload?: string) {
  const res = await app.inject({ method, url, payload });
  return { status: res.statusCode, body: res.body, headers: res.headers };
}

async function expect(app: ReturnType<typeof newApp>, method: Method, url: string, status: number, body: string, payload?: string) {
  const res = await call(app, method, url, payload);
  assert.deepEqual({ status: res.status, body: res.body }, { status, body }, `${method} ${url}`);
}

const errNotFound = '{"error":"product not found"}';
const errBadBody = '{"error":"invalid request body"}';
const errBadId = '{"error":"validation failed","fields":[{"field":"id","message":"must be an integer between 1 and 9007199254740991"}]}';

const ids = (body: string) => (JSON.parse(body) as { products: { id: number }[] }).products.map((p) => p.id);

describe('GET /products/:id', () => {
  it('returns byte-identical JSON with explicit Content-Length', async () => {
    const app = newApp();
    const res = await call(app, 'GET', '/products/1');
    assert.equal(res.status, 200);
    assert.equal(res.body, product1JSON);
    assert.equal(res.headers['content-type'], 'application/json; charset=utf-8');
    assert.equal(res.headers['content-length'], String(Buffer.byteLength(product1JSON)));
    assert.equal(res.headers['transfer-encoding'], undefined);
  });

  it('handles missing and invalid ids', async () => {
    const app = newApp();
    await expect(app, 'GET', '/products/999', 404, errNotFound);
    for (const id of ['0', '-1', 'abc', '1.5', '9007199254740992', '99999999999999999999', 'x'.repeat(200)]) {
      await expect(app, 'GET', `/products/${id}`, 400, errBadId);
    }
    await expect(app, 'GET', '/products/+1', 200, product1JSON); // same as Go strconv.ParseInt
  });
});

describe('GET /products', () => {
  it('lists with defaults and paging', async () => {
    const app = newApp();
    await expect(app, 'GET', '/products?limit=1', 200, `{"products":[${product1JSON}]}`);
    assert.deepEqual(ids((await call(app, 'GET', '/products')).body), [1, 2, 3]);
    assert.deepEqual(ids((await call(app, 'GET', '/products?limit=2&offset=1')).body), [2, 3]);
    assert.deepEqual(ids((await call(app, 'GET', '/products?limit=1&limit=3')).body), [1], 'first value wins');
    await expect(app, 'GET', '/products?offset=100', 200, '{"products":[]}');
  });

  it('validates parameters like Go', async () => {
    const app = newApp();
    await expect(app, 'GET', '/products?limit=0&offset=-1', 400,
      '{"error":"validation failed","fields":[{"field":"limit","message":"must be between 1 and 1000"},{"field":"offset","message":"must be between 0 and 1000000000"}]}');
    await expect(app, 'GET', '/products?limit=99999999999999999999', 400,
      '{"error":"validation failed","fields":[{"field":"limit","message":"must be between 1 and 1000"}]}');
    await expect(app, 'GET', '/products?limit=abc&offset=-1', 400,
      '{"error":"validation failed","fields":[{"field":"limit","message":"must be an integer"}]}');
    await expect(app, 'GET', '/products?limit=&offset=1.5', 400,
      '{"error":"validation failed","fields":[{"field":"limit","message":"must be an integer"},{"field":"offset","message":"must be an integer"}]}');
  });
});

describe('GET /products/search', () => {
  it('searches case-insensitively with literal wildcards', async () => {
    const app = newApp();
    await expect(app, 'GET', '/products/search?q=PHONE', 200, `{"products":[${product1JSON}]}`);
    assert.deepEqual(ids((await call(app, 'GET', '/products/search?q=o&limit=2')).body), [1, 2]);
    assert.deepEqual(ids((await call(app, 'GET', '/products/search?q=smart+phone')).body), [1], '+ decodes to space');
    await expect(app, 'GET', '/products/search?q=%25', 200, '{"products":[]}');
    await expect(app, 'GET', '/products/search', 400,
      '{"error":"validation failed","fields":[{"field":"q","message":"is required"}]}');
    await expect(app, 'GET', '/products/search?q=x&limit=1001', 400,
      '{"error":"validation failed","fields":[{"field":"limit","message":"must be between 1 and 1000"}]}');
  });
});

describe('writes', () => {
  it('creates, updates and deletes', async () => {
    const app = newApp();
    const res = await call(app, 'POST', '/products',
      '{"name":"Desk <Lamp> & Co","description":"","category":"Office","price":19.99,"stock":5,"rating":4.5}');
    assert.equal(res.status, 201);
    assert.ok(res.body.includes('"name":"Desk <Lamp> & Co"'), 'no HTML escaping');
    const created = JSON.parse(res.body) as { id: number; created_at: string };
    assert.equal(created.id, 4);

    const upd = await call(app, 'PUT', '/products/4', '{"name":"Desk Lamp","category":"Office","price":25,"stock":1,"rating":5}');
    const updated = JSON.parse(upd.body) as { name: string; price: number; created_at: string };
    assert.equal(upd.status, 200);
    assert.equal(updated.name, 'Desk Lamp');
    assert.equal(updated.price, 25);
    assert.equal(updated.created_at, created.created_at);

    await expect(app, 'DELETE', '/products/4', 204, '');
    await expect(app, 'DELETE', '/products/4', 404, errNotFound);
    await expect(app, 'PUT', '/products/4', 404, errNotFound, '{"name":"x","category":"y"}');
    await expect(app, 'DELETE', '/products/x', 400, errBadId);
  });

  it('rejects bad bodies like Go', async () => {
    const app = newApp();
    const bad: Record<string, string> = {
      malformed: '{"name":',
      array: '[]',
      'unknown field': '{"name":"a","category":"b","color":"red"}',
      'wrong type': '{"name":"a","category":"b","price":"10"}',
      'fractional int': '{"name":"a","category":"b","stock":1.5}',
      'trailing data': '{"name":"a","category":"b"} {}',
      empty: '',
    };
    for (const [name, body] of Object.entries(bad)) {
      const res = await call(app, 'POST', '/products', body);
      assert.deepEqual({ status: res.status, body: res.body }, { status: 400, body: errBadBody }, name);
    }
    const required = '{"error":"validation failed","fields":[{"field":"name","message":"is required"},{"field":"category","message":"is required"}]}';
    await expect(app, 'POST', '/products', 400, required, '{}');
    await expect(app, 'POST', '/products', 400, required, 'null');
    await expect(app, 'POST', '/products', 400,
      '{"error":"validation failed","fields":[{"field":"price","message":"must have at most 2 decimal places"}]}',
      '{"name":"a","category":"b","price":1.005}');
    await expect(app, 'PUT', '/products/0', 400, errBadId, '{"name":"","category":"b"}');

    const big = '{"name":"' + 'a'.repeat(MAX_BODY_BYTES) + '","category":"b"}';
    await expect(app, 'POST', '/products', 413, '{"error":"request body too large"}', big);
  });
});

describe('routing and health', () => {
  it('returns JSON 404s and health status', async () => {
    const app = newApp();
    await expect(app, 'GET', '/health', 200, '{"status":"ok"}');
    await expect(app, 'GET', '/nope', 404, '{"error":"not found"}');
    await expect(app, 'GET', '/products/', 404, '{"error":"not found"}');
    await expect(app, 'DELETE', '/products/', 404, '{"error":"not found"}');
    await expect(app, 'PUT', '/products/', 404, '{"error":"not found"}', '{}');
    await expect(app, 'PATCH', '/products/1', 404, '{"error":"not found"}', '{}');

    const down = newApp(async () => { throw new Error('db down'); });
    await expect(down, 'GET', '/health', 503, '{"status":"unavailable"}');
  });
});
