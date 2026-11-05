import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import type { ProductInput } from '../src/product/product.ts';
import {
  ID_MESSAGE,
  MAX_ID,
  MAX_PRICE,
  validateId,
  validateInput,
  validatePage,
  validateSearch,
  type FieldError,
} from '../src/product/validation.ts';

const valid = (): ProductInput => ({ name: 'Desk Lamp', description: '', category: 'Office', price: 19.99, stock: 0, rating: 4.5 });
const fields = (err: { fields: FieldError[] } | null) => err?.fields ?? null;

// Same table as go-server/internal/product/validation_test.go.
describe('validateInput', () => {
  const cases: [string, (i: ProductInput) => void, FieldError[] | null][] = [
    ['valid', () => {}, null],
    ['max lengths', (i) => {
      i.name = 'a'.repeat(200);
      i.description = 'é'.repeat(2000);
      i.category = 'c'.repeat(100);
    }, null],
    ['code points, not UTF-16 units', (i) => { i.name = '😀'.repeat(200); }, null],
    ['boundary numbers', (i) => { i.price = MAX_PRICE; i.rating = 5; i.stock = 2147483647; }, null],
    ['zero numbers', (i) => { i.price = 0; i.rating = 0; }, null],
    ['empty name', (i) => { i.name = ''; }, [{ field: 'name', message: 'is required' }]],
    ['long name', (i) => { i.name = 'a'.repeat(201); }, [{ field: 'name', message: 'must be between 1 and 200 characters' }]],
    ['long description', (i) => { i.description = 'a'.repeat(2001); },
      [{ field: 'description', message: 'must be between 0 and 2000 characters' }]],
    ['empty category', (i) => { i.category = ''; }, [{ field: 'category', message: 'is required' }]],
    ['negative price', (i) => { i.price = -0.01; }, [{ field: 'price', message: 'must be between 0 and 99999999.99' }]],
    ['price too large', (i) => { i.price = 100_000_000; }, [{ field: 'price', message: 'must be between 0 and 99999999.99' }]],
    ['price NaN', (i) => { i.price = NaN; }, [{ field: 'price', message: 'must be between 0 and 99999999.99' }]],
    ['price 3 decimals', (i) => { i.price = 1.005; }, [{ field: 'price', message: 'must have at most 2 decimal places' }]],
    ['negative stock', (i) => { i.stock = -1; }, [{ field: 'stock', message: 'must be greater than or equal to 0' }]],
    ['rating too large', (i) => { i.rating = 5.1; }, [{ field: 'rating', message: 'must be between 0 and 5' }]],
    ['rating 2 decimals', (i) => { i.rating = 4.25; }, [{ field: 'rating', message: 'must have at most 1 decimal places' }]],
    ['all fields, in field order', (i) => { Object.assign(i, { name: '', category: '', price: -1, stock: -1, rating: -1 }); }, [
      { field: 'name', message: 'is required' },
      { field: 'category', message: 'is required' },
      { field: 'price', message: 'must be between 0 and 99999999.99' },
      { field: 'stock', message: 'must be greater than or equal to 0' },
      { field: 'rating', message: 'must be between 0 and 5' },
    ]],
  ];
  for (const [name, mutate, want] of cases) {
    it(name, () => {
      const input = valid();
      mutate(input);
      assert.deepEqual(fields(validateInput(input)), want);
    });
  }

  it('accepts every two-decimal price', () => {
    for (let cents = 0; cents <= 100_000; cents++) {
      const input = valid();
      input.price = cents / 100;
      assert.equal(validateInput(input), null, `price ${input.price}`);
    }
  });
});

describe('id, page and search validation', () => {
  it('validateId', () => {
    assert.equal(validateId(1), null);
    assert.equal(validateId(MAX_ID), null);
    for (const id of [0, -5, MAX_ID + 1, 1.5]) {
      assert.deepEqual(fields(validateId(id)), [{ field: 'id', message: ID_MESSAGE }]);
    }
  });

  it('validatePage', () => {
    assert.equal(validatePage(1, 0), null);
    assert.equal(validatePage(1000, 1_000_000_000), null);
    assert.deepEqual(fields(validatePage(0, -1)), [
      { field: 'limit', message: 'must be between 1 and 1000' },
      { field: 'offset', message: 'must be between 0 and 1000000000' },
    ]);
    assert.notEqual(validatePage(1001, 0), null);
  });

  it('validateSearch', () => {
    assert.equal(validateSearch('keyboard', 50), null);
    assert.deepEqual(fields(validateSearch('', 0)), [
      { field: 'q', message: 'is required' },
      { field: 'limit', message: 'must be between 1 and 1000' },
    ]);
    assert.notEqual(validateSearch('q'.repeat(101), 10), null);
  });
});
