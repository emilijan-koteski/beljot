const WINDOW_SIZE = 10;
const MAX_SAMPLE_AGE_MS = 120_000;

interface OffsetSample {
  offsetMs: number;
  atMs: number;
}

let samples: OffsetSample[] = [];
let offsetMs = 0;

/**
 * Feeds one `serverNow` stamp into the estimator, ignoring unparseable input. Each
 * sample underestimates the offset by its latency, so the max over a recent window wins.
 */
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

/** Ms until `deadlineIso` on the server's clock, clamped ≥ 0; unparseable input counts as expired. */
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
