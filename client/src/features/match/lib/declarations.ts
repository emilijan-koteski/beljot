import type { Card, Declaration, Rank, Suit } from "@/shared/types/matchTypes";

// Natural rank order for sequences: 7 < 8 < 9 < T < J < Q < K < A.
const NATURAL_RANK_INDEX: Record<Rank, number> = {
  "7": 0,
  "8": 1,
  "9": 2,
  T: 3,
  J: 4,
  Q: 5,
  K: 6,
  A: 7,
};

const SEQUENCE_POINTS: Record<number, number> = {
  3: 20,
  4: 50,
  // 5+ = 100 (handled below)
};

// Only ranks with non-zero card points are declarable (no 4x7 or 4x8).
const FOUR_OF_A_KIND_POINTS: Partial<Record<Rank, number>> = {
  J: 200,
  "9": 150,
  A: 100,
  T: 100,
  K: 100,
  Q: 100,
};

/**
 * i18n sub-key (under `match.declaration`) for a meld's Rules name: Tierce /
 * Quarte / Quint by run length, Carré for four-of-a-kind — matching the rules
 * reference page. Points are shown separately in the UI, never in this label.
 */
export function declarationLabelKey(
  type: string,
  cardCount: number,
): "tierce" | "quarte" | "quint" | "carre" {
  if (type === "four_of_a_kind") return "carre";
  if (cardCount >= 5) return "quint";
  if (cardCount === 4) return "quarte";
  return "tierce";
}

/**
 * Mirror of server/internal/game/declarations.go `detectDeclarations`.
 * Scans a hand for sequences (3+ consecutive same-suit cards in natural order)
 * and four-of-a-kind (same rank across all 4 suits). Longer sequences subsume
 * shorter subsequences within them.
 *
 * `overlap` must come from `rulesForVariant(...).declarationOverlap` — the
 * single client resolver — and is required so no caller can silently fall back
 * to the wrong variant's rule. When it is true a card may count toward more
 * than one declaration, so the one-card-one-group dedup is skipped and every
 * detected meld survives, exactly as the server records it.
 */
export function detectDeclarations(hand: Card[], overlap: boolean): Declaration[] {
  const decls: Declaration[] = [];

  // Sequences
  const bySuit: Record<Suit, Card[]> = { S: [], H: [], D: [], C: [] };
  for (const c of hand) {
    bySuit[c.suit].push(c);
  }

  for (const suit of Object.keys(bySuit) as Suit[]) {
    const cards = bySuit[suit];
    if (cards.length < 3) continue;
    cards.sort((a, b) => NATURAL_RANK_INDEX[a.rank] - NATURAL_RANK_INDEX[b.rank]);

    let seqStart = 0;
    for (let i = 1; i <= cards.length; i++) {
      const prev = cards[i - 1];
      const curr = cards[i];
      const consecutive =
        i < cards.length &&
        prev !== undefined &&
        curr !== undefined &&
        NATURAL_RANK_INDEX[curr.rank] === NATURAL_RANK_INDEX[prev.rank] + 1;
      if (!consecutive) {
        const seqLen = i - seqStart;
        if (seqLen >= 3) {
          const seqCards = cards.slice(seqStart, i);
          const pts = SEQUENCE_POINTS[seqLen] ?? 100;
          decls.push({
            type: "sequence",
            cards: seqCards,
            value: pts,
            playerSeat: -1,
          });
        }
        seqStart = i;
      }
    }
  }

  // Four-of-a-kind
  const byRank: Partial<Record<Rank, Card[]>> = {};
  for (const c of hand) {
    const arr = byRank[c.rank] ?? [];
    arr.push(c);
    byRank[c.rank] = arr;
  }
  for (const rank of Object.keys(byRank) as Rank[]) {
    const cards = byRank[rank];
    if (cards && cards.length === 4) {
      const pts = FOUR_OF_A_KIND_POINTS[rank];
      if (pts !== undefined) {
        decls.push({
          type: "four_of_a_kind",
          cards: cards.slice(),
          value: pts,
          playerSeat: -1,
        });
      }
    }
  }

  // A card may participate in several declarations under this config, so every
  // detected meld stands on its own.
  if (overlap) return decls;
  return dedupBitola(decls);
}

/**
 * Applies the one-card-one-group rule (what `declarationOverlap: false`
 * selects). Among declarations that share at least one card, the highest-value
 * one is kept and the rest are dropped. Stable — original order is preserved
 * among survivors.
 *
 * Equal-value ties keep the four-of-a-kind, mirroring `declarationBeats` rule 2
 * in the Go engine, so the survivor is the meld the clash comparison would have
 * preferred. Rule 2 alone settles every reachable tie: overlap is only possible
 * between a sequence and a four-of-a-kind, since sequences are maximal
 * per-suit runs and four-of-a-kinds are rank-disjoint.
 */
function dedupBitola(decls: Declaration[]): Declaration[] {
  if (decls.length <= 1) return decls;

  const order = decls.map((_, i) => i);
  // Stable sort by value descending, four-of-a-kind first on a tie (Array.sort
  // in modern engines is stable).
  order.sort((a, b) => {
    const da = decls[a]!;
    const db = decls[b]!;
    if (da.value !== db.value) return db.value - da.value;
    if (da.type !== db.type) return da.type === "four_of_a_kind" ? -1 : 1;
    return 0;
  });

  const used = new Set<string>();
  const keep = new Array<boolean>(decls.length).fill(false);
  for (const idx of order) {
    const d = decls[idx]!;
    let conflict = false;
    for (const c of d.cards) {
      if (used.has(`${c.rank}${c.suit}`)) {
        conflict = true;
        break;
      }
    }
    if (conflict) continue;
    for (const c of d.cards) {
      used.add(`${c.rank}${c.suit}`);
    }
    keep[idx] = true;
  }

  return decls.filter((_, i) => keep[i]);
}
