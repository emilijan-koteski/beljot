// Card-ID validation shared by the WS dispatcher and the match feature.
//
// A card ID is EXACTLY two characters: a rank character followed by a suit
// character (e.g. "JS" = Jack of Spades). Every server payload that carries a
// card carries it in this form, so anything else on the wire is malformed and
// must be rejected rather than sliced into a plausible-but-wrong card — `"10S"`
// silently becoming the Ace of Spades ("1"/"0"…) is the failure mode this
// guards.

import type { CardId, Rank, Suit } from "@/shared/types/matchTypes";

const RANKS: ReadonlySet<string> = new Set<Rank>(["7", "8", "9", "T", "J", "Q", "K", "A"]);
const SUITS: ReadonlySet<string> = new Set<Suit>(["S", "H", "D", "C"]);

/** True when `id` is exactly a valid rank character followed by a valid suit character. */
export function isCardId(id: unknown): id is CardId {
  return typeof id === "string" && id.length === 2 && RANKS.has(id[0]!) && SUITS.has(id[1]!);
}
