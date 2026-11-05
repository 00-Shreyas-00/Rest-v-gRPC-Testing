// Workload: fetch one product by id.
//   REST  GET /products/{id}       gRPC  GetProduct
//
// Ids: 1000 deterministic ids (LCG, seed 42) spread over the whole table;
// each VU walks the list from its own starting point.
//
//   k6 run -e TARGET=go-rest -e VUS=50 get-product.js

import { getProduct } from './lib/client.js';
import { PRODUCT_IDS, pick } from './lib/data.js';
import { buildOptions } from './scenarios.js';

export const options = buildOptions('get-product', { ids: PRODUCT_IDS.length });

export default function () {
  getProduct(pick(PRODUCT_IDS, 97));
}

export { handleSummary, monitor, setup } from './scenarios.js';
