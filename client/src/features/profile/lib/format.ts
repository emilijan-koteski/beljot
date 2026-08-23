/**
 * Whole days between `iso` and now, floored, never negative. Used for the hero
 * "last played" line and any relative-day labels on the profile.
 */
export function daysSince(iso: string): number {
  const then = new Date(iso).getTime();
  if (!Number.isFinite(then)) return 0;
  return Math.max(0, Math.floor((Date.now() - then) / 86_400_000));
}
