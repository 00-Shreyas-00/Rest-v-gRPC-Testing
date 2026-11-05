// Validation rules shared with the Go servers (go-server/internal/product/validation.go).
// Field order, limits and messages must stay identical.

import type { ProductInput } from './product.ts';

export const MAX_NAME_LEN = 200;
export const MAX_DESCRIPTION_LEN = 2000;
export const MAX_CATEGORY_LEN = 100;
export const MAX_PRICE = 99_999_999.99; // NUMERIC(10,2)
export const MAX_RATING = 5;
export const MAX_QUERY_LEN = 100;
export const MAX_ID = Number.MAX_SAFE_INTEGER; // 2^53 - 1

export const DEFAULT_LIST_LIMIT = 20;
export const DEFAULT_SEARCH_LIMIT = 50;
export const MAX_LIMIT = 1000;
export const MAX_OFFSET = 1_000_000_000;

export const ID_MESSAGE = 'must be an integer between 1 and 9007199254740991';

export interface FieldError {
  field: string;
  message: string;
}

export class ValidationError extends Error {
  readonly fields: FieldError[];
  constructor(fields: FieldError[]) {
    super('validation failed: ' + fields.map((f) => `${f.field}: ${f.message}`).join('; '));
    this.name = 'ValidationError';
    this.fields = fields;
  }
}

/** Length in Unicode code points, like Go utf8.RuneCountInString and PostgreSQL char_length. */
function codePointLength(s: string): number {
  let n = 0;
  for (const _ of s) n++;
  return n;
}

class Validator {
  readonly fields: FieldError[] = [];

  add(field: string, message: string): void {
    this.fields.push({ field, message });
  }

  stringLen(field: string, s: string, min: number, max: number): void {
    const n = codePointLength(s);
    if (n < min && min === 1) {
      this.add(field, 'is required');
    } else if (n < min || n > max) {
      this.add(field, `must be between ${min} and ${max} characters`);
    }
  }

  // Excess decimal places are rejected rather than rounded (see Go validator.decimal).
  decimal(field: string, x: number, max: number, places: number): void {
    if (!Number.isFinite(x) || x < 0 || x > max) {
      this.add(field, `must be between 0 and ${max}`);
      return;
    }
    const scale = 10 ** places;
    if (Math.round(x * scale) / scale !== x) {
      this.add(field, `must have at most ${places} decimal places`);
    }
  }

  error(): ValidationError | null {
    return this.fields.length > 0 ? new ValidationError(this.fields) : null;
  }
}

export function validateInput(input: ProductInput): ValidationError | null {
  const v = new Validator();
  v.stringLen('name', input.name, 1, MAX_NAME_LEN);
  v.stringLen('description', input.description, 0, MAX_DESCRIPTION_LEN);
  v.stringLen('category', input.category, 1, MAX_CATEGORY_LEN);
  v.decimal('price', input.price, MAX_PRICE, 2);
  if (input.stock < 0) {
    v.add('stock', 'must be greater than or equal to 0');
  }
  v.decimal('rating', input.rating, MAX_RATING, 1);
  return v.error();
}

export function validateId(id: number): ValidationError | null {
  if (!Number.isInteger(id) || id < 1 || id > MAX_ID) {
    return new ValidationError([{ field: 'id', message: ID_MESSAGE }]);
  }
  return null;
}

export function validatePage(limit: number, offset: number): ValidationError | null {
  const v = new Validator();
  if (limit < 1 || limit > MAX_LIMIT) {
    v.add('limit', `must be between 1 and ${MAX_LIMIT}`);
  }
  if (offset < 0 || offset > MAX_OFFSET) {
    v.add('offset', `must be between 0 and ${MAX_OFFSET}`);
  }
  return v.error();
}

export function validateSearch(query: string, limit: number): ValidationError | null {
  const v = new Validator();
  v.stringLen('q', query, 1, MAX_QUERY_LEN);
  if (limit < 1 || limit > MAX_LIMIT) {
    v.add('limit', `must be between 1 and ${MAX_LIMIT}`);
  }
  return v.error();
}
