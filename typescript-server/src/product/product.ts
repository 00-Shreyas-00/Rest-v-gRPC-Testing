// Product domain model. Mirrors go-server/internal/product.

export interface Product {
  id: number;
  name: string;
  description: string;
  category: string;
  price: number;
  stock: number;
  rating: number;
  // RFC 3339 UTC with trailing zero fraction digits omitted and microsecond
  // precision (Go's RFC3339Nano for PostgreSQL TIMESTAMPTZ values). Kept as
  // strings because JS Date only has millisecond precision.
  created_at: string;
  updated_at: string;
}

/** Writable fields, used by create and update (PUT semantics). */
export interface ProductInput {
  name: string;
  description: string;
  category: string;
  price: number;
  stock: number;
  rating: number;
}

export class NotFoundError extends Error {
  constructor() {
    super('product not found');
    this.name = 'NotFoundError';
  }
}
