// Timestamp helpers. All API timestamps are RFC 3339 UTC strings with
// microsecond precision and trailing zero fraction digits removed, which is
// exactly what the Go servers emit (time.RFC3339Nano on PostgreSQL values).

/**
 * Converts PostgreSQL TIMESTAMPTZ text output (session TimeZone=UTC,
 * DateStyle=ISO), e.g. "2026-11-05 06:25:14.165483+00", to
 * "2026-11-05T06:25:14.165483Z". PostgreSQL already omits trailing zeros.
 */
export function pgTimestampToRFC3339(text: string): string {
  if (text.length > 20 && text.endsWith('+00') && text.charCodeAt(10) === 32 /* space */) {
    return text.slice(0, 10) + 'T' + text.slice(11, -3) + 'Z';
  }
  // Not expected with TimeZone=UTC; fall back to a millisecond-precision conversion.
  return formatMicros(Date.parse(text) * 1000);
}

/** Current wall-clock time in whole microseconds since the Unix epoch. */
export function nowMicros(): number {
  return Math.floor((performance.timeOrigin + performance.now()) * 1000);
}

/** Formats microseconds since the epoch like Go's RFC3339Nano in UTC. */
export function formatMicros(us: number): string {
  const iso = new Date(Math.floor(us / 1000)).toISOString(); // YYYY-MM-DDTHH:MM:SS.mmmZ
  const frac = String(((us % 1_000_000) + 1_000_000) % 1_000_000)
    .padStart(6, '0')
    .replace(/0+$/, '');
  return iso.slice(0, 19) + (frac ? '.' + frac : '') + 'Z';
}
