// Protocol-neutral client: the same three operations over REST/JSON or
// gRPC/Protobuf, selected by the target. Each call is exactly one request.
//
// Kept deliberately cheap on the client side:
//   - REST response bodies are discarded (options.discardResponseBodies);
//     only the status is checked. Correctness of bodies is established by
//     scripts/equivalence.sh, not during load.
//   - gRPC responses are always decoded by k6 (unavoidable), which costs more
//     client CPU than REST, especially for large lists. Watch k6's CPU.
//   - Requests are tagged with a fixed `name`, so k6 does not create a time
//     series per URL.

import http from 'k6/http';
import grpc from 'k6/net/grpc';
import { Counter, Rate } from 'k6/metrics';
import { settings, target } from './config.js';

/** Fraction of requests that failed (non-200 / non-OK, timeouts, transport errors). */
export const errors = new Rate('errors');
/** Requests that hit TIMEOUT. */
export const timeouts = new Counter('timeouts');

const K6_REQUEST_TIMEOUT = 1050; // k6 error code for an HTTP request timeout

// gRPC client and proto must be created in the init context (once per VU).
let grpcClient = null;
let grpcConnected = false;
if (target.protocol === 'grpc') {
  grpcClient = new grpc.Client();
  grpcClient.load(['/proto'], 'product.proto');
}

function rest(path, name, extraTags) {
  const res = http.get(target.address + path, {
    timeout: settings.timeout,
    tags: { name, ...extraTags },
  });
  const timedOut = res.error_code === K6_REQUEST_TIMEOUT;
  if (timedOut) timeouts.add(1, extraTags);
  errors.add(res.status !== 200, extraTags);
}

function rpc(method, request, extraTags) {
  let ok = false;
  let timedOut = false;
  try {
    if (!grpcConnected) {
      // One connection per VU, kept for the whole test (like an HTTP keep-alive connection).
      grpcClient.connect(target.address, { plaintext: true, timeout: settings.timeout });
      grpcConnected = true;
    }
    const res = grpcClient.invoke(`product.v1.ProductService/${method}`, request, {
      timeout: settings.timeout,
      tags: { name: method, ...extraTags },
      // The gRPC counterpart of discardResponseBodies: without it k6 converts
      // every response into JavaScript objects, which for large lists costs
      // far more CPU than the server spends and makes k6 the bottleneck.
      discardResponseMessage: true,
    });
    ok = res.status === grpc.StatusOK;
    timedOut = res.status === grpc.StatusDeadlineExceeded;
  } catch (e) {
    grpcConnected = false; // reconnect on the next request
  }
  if (timedOut) timeouts.add(1, extraTags);
  errors.add(!ok, extraTags);
}

export function getProduct(id) {
  if (target.protocol === 'grpc') {
    rpc('GetProduct', { id }, {});
  } else {
    rest(`/products/${id}`, 'GET /products/{id}', {});
  }
}

export function listProducts(limit, offset) {
  if (target.protocol === 'grpc') {
    rpc('ListProducts', { limit, offset }, {});
  } else {
    rest(`/products?limit=${limit}&offset=${offset}`, 'GET /products', {});
  }
}

/** limit may be undefined to use the server default (50). */
export function searchProducts(term, limit) {
  const tags = { term };
  if (target.protocol === 'grpc') {
    rpc('SearchProducts', limit === undefined ? { query: term } : { query: term, limit }, tags);
  } else {
    const q = `q=${encodeURIComponent(term)}` + (limit === undefined ? '' : `&limit=${limit}`);
    rest(`/products/search?${q}`, 'GET /products/search', tags);
  }
}
