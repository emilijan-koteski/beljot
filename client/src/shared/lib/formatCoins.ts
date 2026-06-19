// Single formatting path for coin amounts. Every coin surface — the header /
// profile balance pills, room buy-in labels, the daily-reward dialog, and the
// match-result settlement line — renders through this so a 4-digit value shows
// the same way everywhere (e.g. 6000 → "6,000") instead of some sites grouping
// and others printing the raw integer.
//
// Wraps toLocaleString() (runtime default locale) to match the long-standing
// pill behavior. Centralizing it here means a later change — locale-aware
// grouping, compact notation for huge stakes — lands in one place.
export function formatCoins(amount: number): string {
  return amount.toLocaleString();
}
