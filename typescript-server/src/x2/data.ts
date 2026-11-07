// Deterministic data set of experiment 2. Line-by-line port of
// go-server/internal/x2data: both must produce identical items (checked by
// `loadgen verify`). Only operations with identical results in Go and
// JavaScript are used: 32-bit integer math via Math.imul / >>> 0, and doubles.

export const POOL_SIZE = 4000;
export const EXPORT_SIZE = 100000;
export const MAX_RATE = 1000;
const DESCRIPTION_LEN = 1000;
const SERIES_LEN = 64;
const THUMBNAIL_LEN = 1000;
const SERIES_START_MS = 1767225600000; // 2026-01-01T00:00:00Z

export const PROFILES = ['text', 'numeric', 'binary'] as const;
export type Profile = (typeof PROFILES)[number];
const SALTS: Record<Profile | 'small', number> = { small: 1, text: 2, numeric: 3, binary: 4 };

/**
 * Base fields first, then at most one heavy field: same key order as the Go
 * struct, so JSON.stringify output is byte-identical to Go's encoding/json.
 */
export interface Item {
  id: number;
  sku: string;
  name: string;
  price: number;
  stock: number;
  description?: string;
  timestamps?: number[];
  values?: number[];
  thumbnail?: Buffer;
}

const WORDS = [
  'alpha', 'basic', 'cargo', 'delta', 'eagle', 'fable', 'giant', 'harbor',
  'ivory', 'jolly', 'karma', 'lunar', 'maple', 'noble', 'ocean', 'prism',
  'quartz', 'rapid', 'solar', 'tiger', 'ultra', 'vivid', 'wander', 'xenon',
  'yield', 'zephyr', 'amber', 'breeze', 'cobalt', 'dune', 'ember', 'fjord',
];

/** mulberry32 */
class Rng {
  private a: number;
  constructor(id: number, salt: number) {
    this.a = (Math.imul(id, 0x9e3779b1) + salt) >>> 0;
  }
  next(): number {
    this.a = (this.a + 0x6d2b79f5) >>> 0;
    let t = this.a;
    t = Math.imul(t ^ (t >>> 15), 1 | t);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return (t ^ (t >>> 14)) >>> 0;
  }
  /** double in [0, 1) with 53 random bits */
  float(): number {
    const a = this.next() >>> 5;
    const b = this.next() >>> 6;
    return (a * 67108864 + b) / 9007199254740992;
  }
}

const word = (r: Rng) => WORDS[r.next() % 32] as string;

// Description pseudo-words (1-3 syllables): a large vocabulary, so the text
// compresses like real product copy. Same list and order as Go.
const SYLLABLES = [
  'ka', 'lo', 'mi', 'ne', 'ru', 'sa', 'to', 'vi', 'ber', 'con', 'dex', 'fal',
  'gen', 'hor', 'lin', 'mon', 'por', 'quel', 'ras', 'sten', 'tur', 'val', 'wex', 'zor',
];
function pseudoWord(r: Rng): string {
  const n = 1 + (r.next() % 3);
  let w = '';
  for (let i = 0; i < n; i++) w += SYLLABLES[r.next() % 24] as string;
  return w;
}

/**
 * A thumbnail item serializes its bytes as base64 on every JSON.stringify,
 * like Go's encoding/json does for []byte (no cached string).
 */
function withThumbnail(base: Item, thumb: Buffer): Item {
  return {
    ...base,
    thumbnail: thumb,
    toJSON() {
      return { id: base.id, sku: base.sku, name: base.name, price: base.price, stock: base.stock, thumbnail: thumb.toString('base64') };
    },
  } as Item;
}

export function generate(id: number, profile: Profile | 'small'): Item {
  const b = new Rng(id, SALTS.small);
  const name = `${word(b)} ${word(b)} ${word(b)}`;
  const price = Math.round(b.float() * 100000) / 100;
  const stock = b.next() % 1000;
  const base: Item = { id, sku: `SKU-${String(id).padStart(7, '0')}`, name, price, stock };
  const r = new Rng(id, SALTS[profile]);
  switch (profile) {
    case 'text': {
      let s = '';
      while (s.length < DESCRIPTION_LEN) {
        if (s.length > 0) s += ' ';
        s += pseudoWord(r);
      }
      return { ...base, description: s.slice(0, DESCRIPTION_LEN) };
    }
    case 'numeric': {
      const timestamps: number[] = [];
      const values: number[] = [];
      for (let i = 0; i < SERIES_LEN; i++) {
        timestamps.push(SERIES_START_MS + i * 60000 + (r.next() % 1000));
        values.push(r.float() * 1000);
      }
      return { ...base, timestamps, values };
    }
    case 'binary': {
      const thumb = Buffer.alloc(THUMBNAIL_LEN);
      for (let i = 0; i < THUMBNAIL_LEN; i += 4) thumb.writeUInt32LE(r.next(), i);
      return withThumbnail(base, thumb);
    }
    default:
      return base;
  }
}

export function items(profile: Profile | 'small', n: number): Item[] {
  const out: Item[] = new Array(n);
  for (let i = 0; i < n; i++) out[i] = generate(i + 1, profile);
  return out;
}

export interface PriceEvent {
  seq: number;
  item_id: number;
  price: number;
  sent_at_us: number;
}

export function eventFor(seq: number, small: Item[]): Omit<PriceEvent, 'sent_at_us'> {
  const id = ((seq * 7919) % EXPORT_SIZE) + 1;
  return { seq, item_id: id, price: (small[id - 1] as Item).price };
}

/** Unix time in microseconds (high resolution, same clock as Go time.Now). */
export function nowUs(): number {
  return Math.round((performance.timeOrigin + performance.now()) * 1000);
}

interface Dataset {
  small: Item[];
  profiles: Record<Profile, Item[]>;
}
let dataset: Dataset | undefined;

/** Built lazily on first use, like the Go servers. */
export function getData(): Dataset {
  dataset ??= {
    small: items('small', EXPORT_SIZE),
    profiles: { text: items('text', POOL_SIZE), numeric: items('numeric', POOL_SIZE), binary: items('binary', POOL_SIZE) },
  };
  return dataset;
}
