// Workload: case-insensitive product name search.
//   REST  GET /products/search?q=TERM[&limit=N]    gRPC  SearchProducts(query[, limit])
//
// TERM unset (default): VUs rotate through keyboard, phone, laptop, monitor,
// mouse, and results are also reported per term (payload sizes differ by
// term). TERM=<word> pins one term. LIMIT unset sends no limit, so the server
// default (50) applies.
//
//   k6 run -e TARGET=go-grpc -e VUS=250 search-products.js

import { positiveInt } from './lib/config.js';
import { searchProducts } from './lib/client.js';
import { SEARCH_TERMS, pick } from './lib/data.js';
import { buildOptions } from './scenarios.js';

const TERMS = __ENV.TERM ? [__ENV.TERM] : SEARCH_TERMS;
const LIMIT = __ENV.LIMIT ? positiveInt('LIMIT', 50) : undefined;

export const options = buildOptions(
  'search-products',
  { terms: TERMS, limit: LIMIT === undefined ? 'server default (50)' : LIMIT },
  { tag: 'term', values: TERMS },
);

export default function () {
  searchProducts(pick(TERMS, 1), LIMIT);
}

export { handleSummary, monitor, setup } from './scenarios.js';
