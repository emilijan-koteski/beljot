// Beljot · Rules content — shared types.
//
// The standalone Rules page (and, later, the in-game Rules overlay) render
// from this typed content. Numeric facts (point values, strength, declaration
// tiers) live in `shared.ts` ONCE; only the human-readable strings vary by
// locale. A parity test asserts the four locales never drift in structure.

import type { HonorTier } from "@/shared/lib/honor";
import type { Variant } from "@/shared/types/matchTypes";

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

// One honour tier line: the token picks the name (from the shared i18n tier
// keys) and colour (HONOR_TIER_COLOR) at render time; only the sentence tail
// is authored per locale. The score range is derived from HONOR_TIER_BANDS,
// so a tier retune never leaves stale numbers in four content files.
export type TierItem = { tier: HonorTier; d: string };

// Variant scoping, carried by every block kind.
//
// Most of the rules are identical in both variants, so the default — no
// `variant` — means "renders under both tabs". Only a genuinely divergent block
// names one, and it renders only while that variant's tab is active. Whole
// chapters are never duplicated: the visible diff between the two tabs IS the
// divergence list.
//
// `otherVariantNote` is authored prose, not a derived badge — a marker that
// says "this differs" tells a player nothing; the useful sentence is what the
// OTHER variant does. It lives beside the block it annotates so it is
// translated with it.
export type VariantScope = {
  variant?: Variant;
  otherVariantNote?: string;
};

// A content block inside a chapter. The prototype's unused `split` is omitted.
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
  // Chrome for the variant split: the eyebrow above the tab bar (tab labels
  // themselves come from `variantLabel`, not from here) and the accessible
  // name of the per-block difference marker.
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
