// Business rules: validate, then delegate to the repository.
// Mirrors go-server/internal/service.

import type { Product, ProductInput } from '../product/product.ts';
import {
  ValidationError,
  validateId,
  validateInput,
  validatePage,
  validateSearch,
} from '../product/validation.ts';

/**
 * Data source contract, implemented by PostgresRepository and MemoryRepository.
 * Unknown ids reject with NotFoundError; list/search always resolve to an array.
 */
export interface Repository {
  get(id: number): Promise<Product>;
  list(limit: number, offset: number): Promise<Product[]>;
  search(query: string, limit: number): Promise<Product[]>;
  create(input: ProductInput): Promise<Product>;
  update(id: number, input: ProductInput): Promise<Product>;
  delete(id: number): Promise<void>;
}

/**
 * Errors: ValidationError for invalid input (repository not called),
 * NotFoundError for unknown ids, anything else is an internal error.
 */
export class ProductService {
  private readonly repo: Repository;

  constructor(repo: Repository) {
    this.repo = repo;
  }

  async get(id: number): Promise<Product> {
    throwIf(validateId(id));
    return this.repo.get(id);
  }

  async list(limit: number, offset: number): Promise<Product[]> {
    throwIf(validatePage(limit, offset));
    return this.repo.list(limit, offset);
  }

  async search(query: string, limit: number): Promise<Product[]> {
    throwIf(validateSearch(query, limit));
    return this.repo.search(query, limit);
  }

  async create(input: ProductInput): Promise<Product> {
    throwIf(validateInput(input));
    return this.repo.create(input);
  }

  async update(id: number, input: ProductInput): Promise<Product> {
    const fields = [...(validateId(id)?.fields ?? []), ...(validateInput(input)?.fields ?? [])];
    if (fields.length > 0) throw new ValidationError(fields);
    return this.repo.update(id, input);
  }

  async delete(id: number): Promise<void> {
    throwIf(validateId(id));
    return this.repo.delete(id);
  }
}

function throwIf(err: ValidationError | null): void {
  if (err) throw err;
}
