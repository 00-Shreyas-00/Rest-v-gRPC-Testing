import assert from 'node:assert/strict';
import type { AddressInfo } from 'node:net';
import { describe, it } from 'node:test';
import { loadConfig } from '../src/config.ts';
import { createMetricsServer, RuntimeMonitor } from '../src/monitoring.ts';

describe('RuntimeMonitor', () => {
  it('reports cumulative counters that advance', async () => {
    const m = new RuntimeMonitor();
    try {
      const before = m.snapshot();
      let junk: number[][] = [];
      for (let i = 0; i < 200; i++) junk.push(new Array(10_000).fill(i));
      junk = [];
      await new Promise((r) => setTimeout(r, 60)); // let the delay histogram sample
      const after = m.snapshot();

      assert.equal(after.runtime, 'node');
      assert.ok(after.uptime_seconds > before.uptime_seconds);
      assert.ok(after.cpu_seconds > before.cpu_seconds);
      assert.ok(after.rss_bytes > 0 && after.heap_total_bytes >= after.heap_used_bytes);
      assert.ok(after.event_loop_active_ms + after.event_loop_idle_ms > before.event_loop_active_ms + before.event_loop_idle_ms);
      assert.ok(after.event_loop_delay_ms.p99 !== null && after.event_loop_delay_ms.p99 >= 0);
      assert.ok(after.gc_count >= before.gc_count);
    } finally {
      m.stop();
    }
  });

  it('serves GET /runtime with the fields shared with Go', async () => {
    const m = new RuntimeMonitor();
    const server = createMetricsServer(m);
    await new Promise<void>((r) => server.listen(0, '127.0.0.1', r));
    try {
      const { port } = server.address() as AddressInfo;
      const res = await fetch(`http://127.0.0.1:${port}/runtime`);
      assert.equal(res.status, 200);
      const body = (await res.json()) as Record<string, unknown>;
      for (const k of ['runtime', 'uptime_seconds', 'cpu_seconds', 'rss_bytes', 'heap_used_bytes',
        'heap_total_bytes', 'gc_count', 'gc_pause_seconds']) {
        assert.ok(k in body, `missing ${k}`);
      }
      assert.equal((await fetch(`http://127.0.0.1:${port}/other`)).status, 404);
    } finally {
      server.close();
      server.closeAllConnections();
      m.stop();
    }
  });
});

describe('config', () => {
  it('defaults METRICS_PORT to 9100 and allows 0 to disable', () => {
    assert.equal(loadConfig({}).metricsPort, 9100);
    assert.equal(loadConfig({ METRICS_PORT: '0' }).metricsPort, 0);
    assert.throws(() => loadConfig({ METRICS_PORT: '-1' }), /METRICS_PORT/);
  });
});
