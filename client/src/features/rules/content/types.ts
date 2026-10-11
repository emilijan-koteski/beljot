import type { HonorTier } from "@/shared/lib/honor";
import type { Variant } from "@/shared/types/matchTypes";

export type Rank = "7" | "8" | "9" | "10" | "J" | "Q" | "K" | "A";

export type RulesLang = "en" | "mk" | "hr" | "sr";

/** One row of a card-value ladder, in strength order (1 = strongest). */
export type CardRow = {
  rank: Rank;
  name: string;
  pts: number;
  strength: number;
  note: string;
};

export type DeclarationKind = "belot" | "run" | "set";

export type Declaration = {
  id: string;
  pts: number;
  tier: 0 | 1 | 2 | 3;
  kind: DeclarationKind;
  name: string;
  summary: string;
  detail: string;
};

/**
 * Variant scoping for a block or step item; no `variant` means it renders under both tabs.
 * `otherVariantNote` is authored prose describing what the other variant does.
 */
export type VariantScope = {
  variant?: Variant;
  otherVariantNote?: string;
};

/** One numbered step, scoped per item because deal-and-bid sequences diverge step by step. */
export type StepItem = VariantScope & { t: string; d: string };

/** One honour tier line; name, colour and score range derive from the tier token at render time. */
export type TierItem = { tier: HonorTier; d: string };

/** A content block inside a chapter. */
export type RuleBlock =
  | (VariantScope & { kind: "p"; text: string })
  | (VariantScope & { kind: "rule"; title: string; text: string })
  | (VariantScope & { kind: "steps"; items: StepItem[] })
  | (VariantScope & { kind: "tiers"; title: string; items: TierItem[]; text: string })
  | (VariantScope & { kind: "cards" })
  | (VariantScope & { kind: "melds" })
  | (VariantScope & { kind: "note"; text: string });

export type RuleSection = {
  id: string;
  label: string;
  title: string;
  lede: string;
  blocks: RuleBlock[];
};

export type Fact = { label: string; value: string; caption: string };

/** Chrome and label strings; `ov*` are reserved for the future in-game overlay. */
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
  variantLabel: string;
  diffLabel: string;
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

/** Per-locale source strings; numbers are merged in from `shared.ts`. */
export type RulesLangData = {
  cardNames: Record<Rank, string>;
  trumpNotes: Partial<Record<Rank, string>>;
  plainNotes: Partial<Record<Rank, string>>;
  declarations: Record<string, { name: string; summary: string; detail: string }>;
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
