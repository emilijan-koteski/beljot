/** Formats a coin amount with locale digit grouping, the single path for every coin surface. */
export function formatCoins(amount: number): string {
  return amount.toLocaleString();
}
