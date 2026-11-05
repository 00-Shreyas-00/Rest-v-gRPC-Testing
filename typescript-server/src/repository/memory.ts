// In-memory repository. Same data structure and algorithms as
// go-server/internal/repository/memory.go:
//   - array sorted by id, binary search for lookups
//   - case-insensitive literal substring search on precomputed lowercase names
//   - ids continue from the highest loaded id and are never reused
//   - timestamps in UTC with microsecond precision
// No lock is needed: JavaScript execution in a Node.js process is single-threaded.

import { NotFoundError, type Product, type ProductInput } from '../product/product.ts';
import { formatMicros, nowMicros } from '../product/time.ts';
import type { Repository } from '../service/service.ts';

interface Item {
  p: Product;
  lowerName: string;
}

export class MemoryRepository implements Repository {
  private items: Item[];
  private nextId: number;
  /** Clock in microseconds since the epoch (replaceable in tests). */
  now: () => number = nowMicros;

  constructor(products: Product[]) {
    this.items = products.map((p) => ({ p, lowerName: p.name.toLowerCase() }));
    this.items.sort((a, b) => a.p.id - b.p.id);
    const last = this.items.at(-1);
    this.nextId = last ? last.p.id + 1 : 1;
  }

  get size(): number {
    return this.items.length;
  }

  /** Index of id, or -1. */
  private find(id: number): number {
    let lo = 0;
    let hi = this.items.length - 1;
    while (lo <= hi) {
      const mid = (lo + hi) >>> 1;
      const midId = this.items[mid]!.p.id;
      if (midId === id) return mid;
      if (midId < id) lo = mid + 1;
      else hi = mid - 1;
    }
    return -1;
  }

  async get(id: number): Promise<Product> {
    const i = this.find(id);
    if (i < 0) throw new NotFoundError();
    return this.items[i]!.p;
  }

  async list(limit: number, offset: number): Promise<Product[]> {
    const start = Math.min(offset, this.items.length);
    const end = Math.min(start + limit, this.items.length);
    const out: Product[] = [];
    for (let i = start; i < end; i++) out.push(this.items[i]!.p);
    return out;
  }

  async search(query: string, limit: number): Promise<Product[]> {
    const q = query.toLowerCase();
    const out: Product[] = [];
    for (const it of this.items) {
      if (it.lowerName.includes(q)) {
        out.push(it.p);
        if (out.length === limit) break;
      }
    }
    return out;
  }

  async create(input: ProductInput): Promise<Product> {
    const ts = formatMicros(this.now());
    const p: Product = {
      id: this.nextId++,
      name: input.name,
      description: input.description,
      category: input.category,
      price: input.price,
      stock: input.stock,
      rating: input.rating,
      created_at: ts,
      updated_at: ts,
    };
    this.items.push({ p, lowerName: p.name.toLowerCase() }); // ids only grow
    return p;
  }

  async update(id: number, input: ProductInput): Promise<Product> {
    const i = this.find(id);
    if (i < 0) throw new NotFoundError();
    const old = this.items[i]!.p;
    // Replace rather than mutate, like Go's value copy, so earlier results are unaffected.
    const p: Product = {
      id: old.id,
      name: input.name,
      description: input.description,
      category: input.category,
      price: input.price,
      stock: input.stock,
      rating: input.rating,
      created_at: old.created_at,
      updated_at: formatMicros(this.now()),
    };
    this.items[i] = { p, lowerName: p.name.toLowerCase() };
    return p;
  }

  async delete(id: number): Promise<void> {
    const i = this.find(id);
    if (i < 0) throw new NotFoundError();
    this.items.splice(i, 1);
  }
}
