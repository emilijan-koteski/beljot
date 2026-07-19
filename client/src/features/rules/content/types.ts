// Beljot · Rules content — shared types.
//
// The standalone Rules page (and, later, the in-game Rules overlay) render
// from this typed content. Numeric facts (point values, strength, declaration
// tiers) live in `shared.ts` ONCE; only the human-readable strings vary by
// locale. A parity test asserts the four locales never drift in structure.

export type Rank = "7" | "8" | "9" | "10" | "J" | "Q" | "K" | "A";

export type RulesLang = "en" | "mk" | "hr" | "sr";

// One row of a card-value ladder, in strength order (1 = strongest).
export type CardRow = {
  rank: Rank;
  name: string; // localized
  pts: number;
  strength: number;
  note: string; // localized; "" when none
};

export type DeclarationKind = "belot" | "run" | "set";

export type Declaration = {
  id: string;
  pts: number;
  tier: 0 | 1 | 2 | 3; // visual grouping: small / mid / jackpot / match-winner
  kind: DeclarationKind;
  name: string; // localized
  summary: string; // localized
  detail: string; // localized
};

export type StepItem = { t: string; d: string };

// A content block inside a chapter. The prototype's unused `split` is omitted.
export type RuleBlock =
  | { kind: "p"; text: string }
  | { kind: "rule"; title: string; text: string }
  | { kind: "steps"; items: StepItem[] }
  | { kind: "cards" }
  | { kind: "melds" }
  | { kind: "note"; text: string };

export type RuleSection = {
  id: string;
  label: string;
  title: string;
  lede: string;
  blocks: RuleBlock[];
};

export type Fact = { label: string; value: string; caption: string };

// Chrome / label strings. `ov*` are reserved for the future in-game overlay.
export type RulesUi = {
  heroEyebrow: string;
  heroTitle: string;
  heroIntro: string;
  facts: Fact[];
  tocTitle: string;
  footerTitle: string;
  footerBody: string;
  footerCta: string;
  noteLabel: string;
  pts: string;
  ladderTrumpTitle: string;
  ladderTrumpEyebrow: string;
  ladderPlainTitle: string;
  ladderPlainEyebrow: string;
  colCard: string;
  colPoints: string;
  colPower: string;
  meldKinds: Record<DeclarationKind, string>;
  ovReference: string;
  ovTitle: string;
  ovChapters: string;
  ovFullRef: string;
  ovClose: string;
};

// Per-locale source: strings only. Numbers are merged in from `shared.ts`.
export type RulesLangData = {
  cardNames: Record<Rank, string>;
  trumpNotes: Partial<Record<Rank, string>>;
  plainNotes: Partial<Record<Rank, string>>;
  declarations: Record<string, { name: string; summary: string; detail: string }>;
  sections: RuleSection[];
  ui: RulesUi;
};

// Assembled, render-ready content for one locale.
export type RulesContent = {
  cardsTrump: CardRow[];
  cardsPlain: CardRow[];
  declarations: Declaration[];
  sections: RuleSection[];
  ui: RulesUi;
};
