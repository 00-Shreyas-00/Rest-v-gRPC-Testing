// Deterministic benchmark inputs. Every run sends exactly the same requests,
// and go-server/equivalence checks these same inputs across all servers.

export const SEEDED_PRODUCTS = 100000;

/**
 * Product ids for the get-product workload: 32-bit LCG (Numerical Recipes
 * constants), seed 42, mapped into 1..100000. Identical to
 * benchmarkProductIDs in go-server/equivalence.
 */
export function lcgProductIds(n, seed = 42) {
  const ids = new Array(n);
  let state = seed >>> 0;
  for (let i = 0; i < n; i++) {
    state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
    ids[i] = (state % SEEDED_PRODUCTS) + 1;
  }
  return ids;
}

export const PRODUCT_IDS = lcgProductIds(1000);

/** Search terms; each matches a known number of seeded names (README). */
export const SEARCH_TERMS = ['keyboard', 'phone', 'laptop', 'monitor', 'mouse'];

/** List limits benchmarked by default. */
export const LIST_LIMITS = [10, 50, 100, 500];

/**
 * List requests page through the first LIST_PAGES pages (offset = page * limit),
 * like a client paging a catalogue. Deep offsets are avoided on purpose: their
 * cost is dominated by PostgreSQL skipping rows, not by the API.
 */
export const LIST_PAGES = 20;

/**
 * Deterministic element for the current VU and iteration. Each VU starts at a
 * different position (stride) and walks the sequence, so concurrent VUs do not
 * request the same item in lock-step.
 */
export function pick(seq, stride) {
  return seq[((__VU - 1) * stride + __ITER) % seq.length];
}
