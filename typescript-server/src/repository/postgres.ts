// PostgreSQL repository. The SQL text and parameter order are copied verbatim
// from go-server/internal/repository/postgres.go; keep them identical.

import pg from 'pg';
import { NotFoundError, type Product, type ProductInput } from '../product/product.ts';
import { pgTimestampToRFC3339 } from '../product/time.ts';
import type { Repository } from '../service/service.ts';

const columns = 'id, name, description, category, price, stock, rating, created_at, updated_at';

export const SQL = {
  get: `SELECT ${columns} FROM products WHERE id = $1`,
  list: `SELECT ${columns} FROM products ORDER BY id LIMIT $1 OFFSET $2`,
  search: `SELECT ${columns} FROM products WHERE name ILIKE $1 ORDER BY id LIMIT $2`,
  create: `INSERT INTO products (name, description, category, price, stock, rating)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING ${columns}`,
  update: `UPDATE products
SET name = $2, description = $3, category = $4, price = $5, stock = $6, rating = $7, updated_at = now()
WHERE id = $1 RETURNING ${columns}`,
  delete: `DELETE FROM products WHERE id = $1`,
  all: `SELECT ${columns} FROM products ORDER BY id`,
} as const;

// Result decoding. node-postgres returns BIGINT and NUMERIC as strings and
// would turn TIMESTAMPTZ into a millisecond-precision Date; these parsers
// produce the same JSON values the Go server emits.
const OID_INT8 = 20;
const OID_NUMERIC = 1700;
const OID_TIMESTAMPTZ = 1184;

const typeOverrides: pg.CustomTypesConfig = {
  getTypeParser: ((oid: number, format?: string) => {
    switch (oid) {
      case OID_INT8:
        return (v: string) => Number(v); // ids are capped at 2^53-1
      case OID_NUMERIC:
        return (v: string) => Number(v);
      case OID_TIMESTAMPTZ:
        return pgTimestampToRFC3339;
      default:
        return pg.types.getTypeParser(oid, format as 'text');
    }
  }) as pg.CustomTypesConfig['getTypeParser'],
};

/**
 * Creates the connection pool. Same size as the Go pool (DB_MAX_CONNS).
 * TimeZone=UTC makes TIMESTAMPTZ text output "...+00".
 */
export function createPool(connectionString: string, max: number): pg.Pool {
  return new pg.Pool({
    connectionString,
    max,
    options: '-c TimeZone=UTC',
    types: typeOverrides,
  });
}

/** Escapes LIKE wildcards so the query matches as a literal substring. */
export function likePattern(query: string): string {
  return '%' + query.replace(/[\\%_]/g, (c) => '\\' + c) + '%';
}

export class PostgresRepository implements Repository {
  private readonly pool: pg.Pool;

  constructor(pool: pg.Pool) {
    this.pool = pool;
  }

  // Named queries are prepared once per connection and reused, matching
  // pgx's default statement cache in the Go server.
  private async queryOne(name: string, text: string, values: unknown[]): Promise<Product> {
    const res = await this.pool.query<Product>({ name, text, values });
    const row = res.rows[0];
    if (row === undefined) throw new NotFoundError();
    return row;
  }

  private async queryMany(name: string, text: string, values: unknown[]): Promise<Product[]> {
    const res = await this.pool.query<Product>({ name, text, values });
    return res.rows;
  }

  get(id: number): Promise<Product> {
    return this.queryOne('products_get', SQL.get, [id]);
  }

  list(limit: number, offset: number): Promise<Product[]> {
    return this.queryMany('products_list', SQL.list, [limit, offset]);
  }

  search(query: string, limit: number): Promise<Product[]> {
    return this.queryMany('products_search', SQL.search, [likePattern(query), limit]);
  }

  create(input: ProductInput): Promise<Product> {
    return this.queryOne('products_create', SQL.create, [
      input.name, input.description, input.category, input.price, input.stock, input.rating,
    ]);
  }

  update(id: number, input: ProductInput): Promise<Product> {
    return this.queryOne('products_update', SQL.update, [
      id, input.name, input.description, input.category, input.price, input.stock, input.rating,
    ]);
  }

  async delete(id: number): Promise<void> {
    const res = await this.pool.query({ name: 'products_delete', text: SQL.delete, values: [id] });
    if (res.rowCount === 0) throw new NotFoundError();
  }

  /** Every product ordered by id; used once at startup for memory mode. */
  all(): Promise<Product[]> {
    return this.queryMany('products_all', SQL.all, []);
  }
}
