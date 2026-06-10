// Client↔server clock-offset estimation.
//
// Match WS messages stamp `serverNow` — the server's wall clock at send time
// (see `WsMessage` / Go `WSMessage`). Each sample observes
// `serverNow − Date.now() = trueOffset − one-way latency`, so every sample
// UNDERestimates the true offset by that message's delivery latency. The max
// over a rolling window is therefore the estimate with the least latency
// error. A bounded window (rather than an all-time max) lets the estimate
// follow genuine clock changes (NTP step, WSL2 drift, laptop resume) within a
// few messages.
//
// Module-level singleton (not React state): countdown hooks read it on every
// tick, and nothing needs to re-render just because the estimate moved by a
// few milliseconds.

const WINDOW_SIZE = 10;
// Age cap on samples: a clock STEP (client NTP catch-up, laptop resume) makes
// every later sample lower, and a pure count-based max would hold the stale
// high estimate until 10 more messages arrive — minutes during idle turns.
// Dropping old samples at record time bounds that staleness. (Eviction runs
// only when a new stamp arrives; between messages nothing reads-and-caches,
// so a lazy sweep is sufficient.)
const MAX_SAMPLE_AGE_MS = 120_000;

interface OffsetSample {
  offsetMs: number;
  atMs: number;
}

let samples: OffsetSample[] = [];
let offsetMs = 0;

/** Feed one `serverNow` envelope stamp into the estimator. NaN-safe (ignored). */
export function recordServerNow(serverNowIso: string): void {
  const serverMs = new Date(serverNowIso).getTime();
  if (Number.isNaN(serverMs)) return;
  const nowMs = Date.now();
  samples.push({ offsetMs: serverMs - nowMs, atMs: nowMs });
  samples = samples.filter((s) => nowMs - s.atMs <= MAX_SAMPLE_AGE_MS).slice(-WINDOW_SIZE);
  offsetMs = Math.max(...samples.map((s) => s.offsetMs));
}

/** Estimated `serverClock − clientClock` in ms; 0 until the first sample. */
export function serverClockOffsetMs(): number {
  return offsetMs;
}

/** Best estimate of the server's current wall clock, in epoch ms. */
export function serverNowMs(): number {
  return Date.now() + offsetMs;
}

/**
 * Ms until `deadlineIso` on the server's clock, clamped ≥ 0. Unparseable
 * input counts as already expired (0) — matches how countdown widgets treat
 * a missing deadline.
 */
export function remainingMsUntil(deadlineIso: string): number {
  const deadlineMs = new Date(deadlineIso).getTime();
  if (Number.isNaN(deadlineMs)) return 0;
  return Math.max(0, deadlineMs - serverNowMs());
}

/** Test-only: clear all samples (Vitest specs share module state). */
export function resetClockSyncForTest(): void {
  samples = [];
  offsetMs = 0;
}
