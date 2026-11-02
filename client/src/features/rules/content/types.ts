import type { Variant } from "@/shared/types/matchTypes";

export type Rank = "7" | "8" | "9" | "10" | "J" | "Q" | "K" | "A";

export type Suit = "S" | "H" | "D" | "C";

export type RulesLang = "en" | "mk" | "hr" | "sr";

/** One row of a card-value table, in strength order (1 = strongest). */
export type CardRow = {
  rank: Rank;
  name: string;
  pts: number;
  strength: number;
};

/**
 * One declaration, pictured by its `cards`; a `set` deals them one per suit, otherwise all in `suit`.
 * `wins` marks the all-eight-trumps hand, which ends the match instead of scoring points.
 */
export type Declaration = {
  id: string;
  pts: number;
  cards: Rank[];
  suit: Suit;
  set: boolean;
  wins: boolean;
  name: string;
  summary?: string;
};

/**
 * Variant scoping for a block or item; no `variant` means it renders under both tabs.
 * `otherVariantNote` is authored prose describing what the other variant does.
 */
export type VariantScope = {
  variant?: Variant;
  otherVariantNote?: string;
};

/** One rule or step: a short bold line and one sentence under it, scoped per item. */
export type StepItem = VariantScope & { t: string; d: string };

/** A content block inside a chapter. */
export type RuleBlock =
  | (VariantScope & { kind: "p"; text: string; help?: string })
  | (VariantScope & { kind: "steps"; items: StepItem[] })
  | (VariantScope & { kind: "list"; numbered?: boolean; items: StepItem[] })
  | (VariantScope & { kind: "cards" })
  | (VariantScope & { kind: "melds" })
  | (VariantScope & { kind: "sum" })
  | (VariantScope & { kind: "options" });

export type SectionId = "deal" | "cards" | "play" | "melds" | "scoring" | "options";

export type RuleSection = {
  id: SectionId;
  label: string;
  title: string;
  lede: string;
  blocks: RuleBlock[];
};

export type Fact = { value: string; label: string };

/** Chrome and label strings for the rules page and the in-match rules dialog. */
export type RulesUi = {
  heroEyebrow: string;
  heroTitle: string;
  heroIntro: string;
  facts: Fact[];
  chaptersLabel: string;
  variantLabel: string;
  diffLabel: string;
  helpLabel: string;
  cardsTrump: string;
  cardsOther: string;
  cardsFoot: string;
  meldWin: string;
  sumCards: string;
  sumLastTrick: string;
  sumDeclarations: string;
  optionsFoot: string;
  footerTitle: string;
  footerBody: string;
  footerCta: string;
  ovTitle: string;
  ovClose: string;
};

/** Per-locale source strings; numbers and pictures are merged in from `shared.ts`. */
export type RulesLangData = {
  cardNames: Record<Rank, string>;
  declarations: Record<string, { name: string; summary?: string }>;
  sections: RuleSection[];
  ui: RulesUi;
};

/** Assembled, render-ready content for one locale. */
export type RulesContent = {
  cardsTrump: CardRow[];
  cardsPlain: CardRow[];
  declarations: Declaration[];
  sections: RuleSection[];
  ui: RulesUi;
};
