// Workload: list a page of products.
//   REST  GET /products?limit=N&offset=M       gRPC  ListProducts(limit, offset)
//
// LIMIT (default 50; benchmark 10, 50, 100, 500) is fixed for a run so each
// payload size is reported separately. Offsets page through the first 20
// pages (offset = page * LIMIT); each VU starts on a different page.
//
//   k6 run -e TARGET=ts-rest -e LIMIT=500 -e VUS=100 list-products.js

import { positiveInt } from './lib/config.js';
import { listProducts } from './lib/client.js';
import { LIST_PAGES, pick } from './lib/data.js';
import { buildOptions } from './scenarios.js';

const LIMIT = positiveInt('LIMIT', 50);
const OFFSETS = Array.from({ length: LIST_PAGES }, (_, page) => page * LIMIT);

export const options = buildOptions('list-products', { limit: LIMIT, pages: LIST_PAGES });

export default function () {
  listProducts(LIMIT, pick(OFFSETS, 7));
}

export { handleSummary, monitor, setup } from './scenarios.js';
