import type { Product } from '../src/product/product.ts';

// Product 1 of the deterministic seed, and its byte-exact JSON encoding.
// The same reference string is asserted by the Go REST tests.
export const product1: Product = {
  id: 1,
  name: 'Orion Smart Phone PH-000001',
  description: 'The Orion smart phone is designed for home offices. Firmware updates are delivered automatically.',
  category: 'Mobile',
  price: 80.18,
  stock: 37,
  rating: 2.3,
  created_at: '2024-01-01T00:05:00Z',
  updated_at: '2024-01-01T07:05:00Z',
};

export const product1JSON =
  '{"id":1,"name":"Orion Smart Phone PH-000001",' +
  '"description":"The Orion smart phone is designed for home offices. Firmware updates are delivered automatically.",' +
  '"category":"Mobile","price":80.18,"stock":37,"rating":2.3,' +
  '"created_at":"2024-01-01T00:05:00Z","updated_at":"2024-01-01T07:05:00Z"}';

export function seedProducts(): Product[] {
  return [
    { ...product1 },
    {
      id: 2, name: 'Quanta Max Laptop LA-000002', description: '', category: 'Computers',
      price: 159.37, stock: 74, rating: 3.6, created_at: product1.created_at, updated_at: product1.updated_at,
    },
    {
      id: 3, name: 'Nimbus Ultra Monitor MO-000003', description: '', category: 'Displays',
      price: 238.56, stock: 111, rating: 4.9, created_at: product1.created_at, updated_at: product1.updated_at,
    },
  ];
}
