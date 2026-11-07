// Experiment 2 codec micro-benchmark for Node.js: JSON.stringify / JSON.parse
// of each payload profile, without network or HTTP. Counterpart of
// `loadgen codec` (Go encoding/json and Protobuf). Prints one JSON line per case.
//
//   node dist/x2/codec-bench.js [seconds-per-operation]

import { items, type Profile } from './data.ts';

const per = Number(process.argv[2] ?? '1') * 1000;

function bench(fn: () => void): number {
  fn();
  let n = 1;
  for (;;) {
    const t = performance.now();
    for (let i = 0; i < n; i++) fn();
    if (performance.now() - t > per / 20) break;
    n *= 2;
  }
  const res: number[] = [];
  for (let k = 0; k < 5; k++) {
    const t = performance.now();
    for (let i = 0; i < n; i++) fn();
    res.push(((performance.now() - t) * 1000) / n);
  }
  res.sort((a, b) => a - b);
  return Math.round((res[2] as number) * 1000) / 1000;
}

const cases: [Profile | 'small', number][] = [
  ['text', 10], ['text', 250], ['text', 4000],
  ['numeric', 10], ['numeric', 250], ['numeric', 4000],
  ['binary', 10], ['binary', 250], ['binary', 4000],
  ['small', 1000], ['small', 100000],
];

let sink = 0;
for (const [profile, count] of cases) {
  const list = { items: items(profile, count) };
  const json = JSON.stringify(list);
  const row = {
    runtime: 'node',
    profile,
    count,
    json_bytes: Buffer.byteLength(json),
    json_encode_us: bench(() => {
      sink += JSON.stringify(list).length;
    }),
    json_decode_us: bench(() => {
      sink += (JSON.parse(json) as { items: unknown[] }).items.length;
    }),
  };
  process.stdout.write(JSON.stringify(row) + '\n');
}
if (sink < 0) process.stdout.write('');
