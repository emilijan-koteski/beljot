import type { DeclarationKind, Rank } from "./types";

export type CardBase = { rank: Rank; pts: number; strength: number };

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
  { id: "bela", pts: 1001, tier: 3, kind: "run" },
];
