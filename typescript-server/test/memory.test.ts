import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { NotFoundError } from '../src/product/product.ts';
import { formatMicros, pgTimestampToRFC3339 } from '../src/product/time.ts';
import { MemoryRepository } from '../src/repository/memory.ts';
import { likePattern } from '../src/repository/postgres.ts';
import { ProductService, type Repository } from '../src/service/service.ts';
import { ValidationError } from '../src/product/validation.ts';
import { seedProducts } from './fixtures.ts';

const input = (name: string) => ({ name, description: '', category: 'x', price: 1, stock: 1, rating: 1 });

describe('MemoryRepository', () => {
  it('sorts input and continues ids without reuse', async () => {
    const [a, b] = seedProducts();
    const m = new MemoryRepository([{ ...b!, id: 7 }, { ...a!, id: 3 }]);
    assert.deepEqual((await m.list(10, 0)).map((p) => p.id), [3, 7]);
    assert.equal((await m.create(input('c'))).id, 8);
    await m.delete(8);
    assert.equal((await m.create(input('d'))).id, 9);
  });

  it('starts at id 1 when empty', async () => {
    const m = new MemoryRepository([]);
    assert.equal((await m.create(input('a'))).id, 1);
    await assert.rejects(m.get(99), NotFoundError);
  });

  it('uses UTC microsecond timestamps; update keeps created_at', async () => {
    const m = new MemoryRepository([]);
    let clock = Date.UTC(2025, 0, 2, 2, 4, 5) * 1000 + 123456;
    m.now = () => clock;
    const p = await m.create(input('a'));
    assert.equal(p.created_at, '2025-01-02T02:04:05.123456Z');
    assert.equal(p.updated_at, p.created_at);
    clock += 3_600_000_000;
    const u = await m.update(p.id, input('b'));
    assert.equal(u.created_at, p.created_at);
    assert.equal(u.updated_at, '2025-01-02T03:04:05.123456Z');
    assert.equal(p.name, 'a', 'earlier results are not mutated');
  });

  it('refreshes the search index on update', async () => {
    const m = new MemoryRepository(seedProducts());
    await m.update(2, input('New Mouse'));
    assert.deepEqual((await m.search('laptop', 10)).map((p) => p.id), []);
    assert.deepEqual((await m.search('MOUSE', 10)).map((p) => p.id), [2]);
  });
});

describe('time helpers', () => {
  it('formats like Go RFC3339Nano', () => {
    const base = Date.UTC(2024, 0, 1) * 1000;
    assert.equal(formatMicros(base), '2024-01-01T00:00:00Z');
    assert.equal(formatMicros(base + 120_000), '2024-01-01T00:00:00.12Z');
    assert.equal(formatMicros(base + 1), '2024-01-01T00:00:00.000001Z');
  });

  it('converts PostgreSQL timestamptz text', () => {
    assert.equal(pgTimestampToRFC3339('2024-01-01 00:05:00+00'), '2024-01-01T00:05:00Z');
    assert.equal(pgTimestampToRFC3339('2026-11-05 06:25:14.165483+00'), '2026-11-05T06:25:14.165483Z');
    assert.equal(pgTimestampToRFC3339('2024-01-01 00:00:00.5+00'), '2024-01-01T00:00:00.5Z');
  });
});

it('likePattern escapes LIKE wildcards', () => {
  assert.equal(likePattern('keyboard'), '%keyboard%');
  assert.equal(likePattern('50%'), '%50\\%%');
  assert.equal(likePattern('a_b'), '%a\\_b%');
  assert.equal(likePattern('c:\\x'), '%c:\\\\x%');
});

describe('ProductService', () => {
  it('rejects invalid input without calling the repository', async () => {
    let calls = 0;
    const repo = new Proxy(new MemoryRepository(seedProducts()), {
      get(target, prop, recv) {
        const v = Reflect.get(target, prop, recv);
        return typeof v === 'function' ? (...args: unknown[]) => { calls++; return v.apply(target, args); } : v;
      },
    }) as Repository;
    const svc = new ProductService(repo);
    const bad = { name: '', description: '', category: '', price: -1, stock: 0, rating: 0 };
    const cases: [Promise<unknown>, string[]][] = [
      [svc.get(0), ['id']],
      [svc.list(0, 0), ['limit']],
      [svc.search('', 10), ['q']],
      [svc.create(bad), ['name', 'category', 'price']],
      [svc.update(-1, bad), ['id', 'name', 'category', 'price']],
      [svc.delete(0), ['id']],
    ];
    for (const [promise, want] of cases) {
      await assert.rejects(promise, (err: unknown) => {
        assert.ok(err instanceof ValidationError);
        assert.deepEqual(err.fields.map((f) => f.field), want);
        return true;
      });
    }
    assert.equal(calls, 0);
  });
});
