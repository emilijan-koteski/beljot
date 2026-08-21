// Beljot · Rules content — shared numeric base.
//
// Point values, strength order, and declaration tiers/kinds live here ONCE so
// they can never drift between the en/mk/hr/sr translations. Locale files
// supply only the human-readable strings (card names, declaration prose, etc.).

import type { DeclarationKind, Rank } from "./types";

export type CardBase = { rank: Rank; pts: number; strength: number };

// Trump suit — Jack and 9 leap to the top.
export const TRUMP_ROWS: CardBase[] = [
  { rank: "J", pts: 20, strength: 1 },
  { rank: "9", pts: 14, strength: 2 },
  { rank: "A", pts: 11, strength: 3 },
  { rank: "10", pts: 10, strength: 4 },
  { rank: "K", pts: 4, strength: 5 },
  { rank: "Q", pts: 3, strength: 6 },
  { rank: "8", pts: 0, strength: 7 },
  { rank: "7", pts: 0, strength: 8 },
];

// Every other suit — the familiar order.
export const PLAIN_ROWS: CardBase[] = [
  { rank: "A", pts: 11, strength: 1 },
  { rank: "10", pts: 10, strength: 2 },
  { rank: "K", pts: 4, strength: 3 },
  { rank: "Q", pts: 3, strength: 4 },
  { rank: "J", pts: 2, strength: 5 },
  { rank: "9", pts: 0, strength: 6 },
  { rank: "8", pts: 0, strength: 7 },
  { rank: "7", pts: 0, strength: 8 },
];

export type DeclarationBase = {
  id: string;
  pts: number;
  tier: 0 | 1 | 2 | 3;
  kind: DeclarationKind;
};

export const DECLARATIONS_BASE: DeclarationBase[] = [
  { id: "belot", pts: 20, tier: 0, kind: "belot" },
  { id: "terca", pts: 20, tier: 0, kind: "run" },
  { id: "kvarta", pts: 50, tier: 1, kind: "run" },
  { id: "kvinta", pts: 100, tier: 1, kind: "run" },
  { id: "carre", pts: 100, tier: 1, kind: "set" },
  { id: "carre9", pts: 150, tier: 2, kind: "set" },
  { id: "carreJ", pts: 200, tier: 2, kind: "set" },
  // Belote / Bela — all eight cards of the TRUMP suit in one hand ends the match
  // on the spot, and the engine DOES implement that: game.checkInstantWin
  // (server/internal/game/scoring.go) runs after the deal and after a resolved
  // pick, setting WinnerTeam and PhaseMatchEnd. It awards no points — it names a
  // winner — so the prose must not promise a 1001-point payout.
  // Only the ANY-suit generalisation is deferred (see deferred-work.md). The
  // earlier wording here claimed the whole rule was documentation-only with no
  // engine support; that was false, and it propagated into the locale copy
  // through two stories before anyone re-read the engine.
  // The 1001 below, and this entry sitting in a list of DECLARATIONS at all, are
  // deliberate holdovers: moving it out is a wire-shape change to this module and
  // is deferred by owner decision.
  // Tier 3 = its own "wins the match on the spot" visual treatment.
  { id: "bela", pts: 1001, tier: 3, kind: "run" },
];
