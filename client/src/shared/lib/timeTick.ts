const TICK_INTERVAL_MS = 30_000;

let tick = 0;
const subscribers = new Set<() => void>();
let intervalHandle: ReturnType<typeof setInterval> | null = null;

function ensureRunning() {
  if (intervalHandle !== null) return;
  if (typeof window === "undefined") return;
  intervalHandle = setInterval(() => {
    tick = (tick + 1) | 0;
    for (const cb of subscribers) cb();
  }, TICK_INTERVAL_MS);
}

/**
 * Subscribes to one shared 30-second tick, so every relative timestamp
 * re-renders from a single timer.
 */
export function subscribeTimeTick(cb: () => void): () => void {
  subscribers.add(cb);
  ensureRunning();
  return () => {
    subscribers.delete(cb);
    if (subscribers.size === 0 && intervalHandle !== null) {
      clearInterval(intervalHandle);
      intervalHandle = null;
    }
  };
}

export function getTimeTick(): number {
  return tick;
}
