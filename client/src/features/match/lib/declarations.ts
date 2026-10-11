import type { Card, Declaration, Rank, Suit } from "@/shared/types/matchTypes";

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
};
const LONG_SEQUENCE_POINTS = 100;

const FOUR_OF_A_KIND_POINTS: Partial<Record<Rank, number>> = {
  J: 200,
  "9": 150,
  A: 100,
  T: 100,
  K: 100,
  Q: 100,
};

/**
 * Return the `match.declaration` i18n sub-key for a meld's rules name: Tierce, Quarte or Quint by run length, Carré for four-of-a-kind.
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
 * Mirror of the server's `detectDeclarations`: find maximal same-suit sequences of 3+ and four-of-a-kinds in a hand.
 * `overlap` (from `rulesForVariant`) lets a card count in several melds; otherwise one-card-one-group dedup applies.
 */
export function detectDeclarations(hand: Card[], overlap: boolean): Declaration[] {
  const decls: Declaration[] = [];

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
          const pts = SEQUENCE_POINTS[seqLen] ?? LONG_SEQUENCE_POINTS;
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

  if (overlap) return decls;
  return dedupBitola(decls);
}

/**
 * Apply the one-card-one-group rule: among melds sharing a card keep the highest value, four-of-a-kind on a tie (as the Go engine's `declarationBeats`).
 * Survivors keep their original order.
 */
function dedupBitola(decls: Declaration[]): Declaration[] {
  if (decls.length <= 1) return decls;

  const order = decls.map((_, i) => i);
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
