// Integration tests against the products_test database (scripts/db.sh testdb,
// 1000 products). Skipped when TEST_DATABASE_URL is not set. Same contract as
// go-server/internal/repository/contract_test.go.

import assert from 'node:assert/strict';
import { after, before, describe, it } from 'node:test';
import type pg from 'pg';
import { NotFoundError, type Product } from '../src/product/product.ts';
import { DEFAULT_SEARCH_LIMIT } from '../src/product/validation.ts';
import { MemoryRepository } from '../src/repository/memory.ts';
import { PostgresRepository, createPool } from '../src/repository/postgres.ts';
import type { Repository } from '../src/service/service.ts';
import { product1 } from './fixtures.ts';

const url = process.env.TEST_DATABASE_URL;
const SEEDED = 1000;
const ids = (ps: Product[]) => ps.map((p) => p.id);

describe('repository contract', { skip: url ? false : 'TEST_DATABASE_URL not set' }, () => {
  let pool: pg.Pool;
  let pgRepo: PostgresRepository;
  let mem: MemoryRepository;

  before(async () => {
    pool = createPool(url!, 4);
    const { rows } = await pool.query<{ n: number }>('SELECT count(*)::int AS n FROM products');
    assert.equal(rows[0]!.n, SEEDED, 'run scripts/db.sh testdb first');
    pgRepo = new PostgresRepository(pool);
    mem = new MemoryRepository(await pgRepo.all());
  });
  after(() => pool?.end());

  const run = (name: string, repo: () => Repository) => describe(name, () => {
    it('gets a seeded product', async () => {
      assert.deepEqual(await repo().get(1), product1);
      await assert.rejects(repo().get(999_999_999), NotFoundError);
    });

    it('lists in id order', async () => {
      assert.deepEqual(ids(await repo().list(5, 0)), [1, 2, 3, 4, 5]);
      assert.deepEqual(ids(await repo().list(5, SEEDED - 2)), [SEEDED - 1, SEEDED]);
      assert.deepEqual(await repo().list(5, SEEDED + 10), []);
    });

    it('searches', async () => {
      assert.deepEqual(ids(await repo().search('keyboard', 3)), [20, 40, 60]);
      assert.deepEqual(ids(await repo().search('PHONE', 4)), [1, 5, 12, 21]);
      const mice = await repo().search('mouse', DEFAULT_SEARCH_LIMIT);
      assert.equal(mice.length, DEFAULT_SEARCH_LIMIT);
      assert.ok(mice.every((p) => p.name.toLowerCase().includes('mouse')));
      for (const q of ['%', '_', 'no-such-product']) assert.deepEqual(await repo().search(q, 10), []);
    });

    it('creates, updates and deletes', async () => {
      const input = { name: 'Contract Test Lamp', description: 'temp', category: 'Office', price: 12.5, stock: 3, rating: 4.5 };
      const created = await repo().create(input);
      try {
        assert.ok(created.id > SEEDED);
        assert.deepEqual({ ...created, id: 0, created_at: '', updated_at: '' }, { ...input, id: 0, created_at: '', updated_at: '' });
        assert.match(created.created_at, /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d{1,6})?Z$/);
        assert.equal(created.updated_at, created.created_at);
        assert.deepEqual(await repo().get(created.id), created);

        const upd = { name: 'Contract Test Lamp v2', description: '', category: 'Home', price: 0, stock: 0, rating: 0 };
        const updated = await repo().update(created.id, upd);
        assert.equal(updated.name, upd.name);
        assert.equal(updated.price, 0);
        assert.equal(updated.created_at, created.created_at);
        assert.ok(updated.updated_at >= created.updated_at);
      } finally {
        await repo().delete(created.id);
      }
      await assert.rejects(repo().get(created.id), NotFoundError);
      await assert.rejects(repo().update(created.id, input), NotFoundError);
      await assert.rejects(repo().delete(created.id), NotFoundError);
    });
  });

  run('postgres', () => pgRepo);
  run('memory', () => mem);

  it('postgres and memory return identical data', async () => {
    const fresh = new MemoryRepository(await pgRepo.all());
    for (const id of [1, 2, 3, 500, SEEDED]) assert.deepEqual(await pgRepo.get(id), await fresh.get(id));
    for (const [l, o] of [[10, 0], [50, 0], [100, 250], [500, 0], [1000, 0], [10, 995], [10, 5000]] as const) {
      assert.deepEqual(await pgRepo.list(l, o), await fresh.list(l, o), `list ${l},${o}`);
    }
    for (const q of ['keyboard', 'phone', 'laptop', 'monitor', 'mouse', 'KeYbOaRd', 'ke-0000', '%', '_', 'zzz']) {
      assert.deepEqual(await pgRepo.search(q, DEFAULT_SEARCH_LIMIT), await fresh.search(q, DEFAULT_SEARCH_LIMIT), `search ${q}`);
    }
  });
});
