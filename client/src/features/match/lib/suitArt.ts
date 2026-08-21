import type { Rank, Suit } from "@/shared/types/matchTypes";

import type { CardDeck } from "./cardFace";

/**
 * Suit marks for the UI chrome — the trump indicator, the trump prompt's suit
 * tiles, the trump reveal seal, and the seat trump chip (Story 12.4, Scope
 * Amendment 1).
 *
 * This is the **one seam** those four surfaces resolve a suit mark through.
 * Before this file each of them declared its own glyph map *and* its own
 * `suit === "H" || suit === "D"` red/black colour test — four copies of the same
 * two decisions. A fifth per-file branch for the Croatian deck is exactly what
 * this module exists to prevent.
 *
 * Scope is bounded on purpose. `shared/components/ui/suit-rule.tsx` (a
 * decorative all-four-suits divider that also renders unauthenticated) and
 * `features/rules/components/CardLadder.tsx` (the rules reference, Story 12.9's
 * territory) stay French-suited under every deck and must not import this.
 *
 * The card *faces* are a separate seam — see `cardFace.ts`. These are the
 * symbols, not the cards.
 *
 * ---
 *
 * This module is deliberately **pure** — palette and URL derivation, no React —
 * mirroring `cardFace.ts` next door. The component that renders a mark is
 * `components/SuitMark.tsx`; keeping them apart is what lets the palette be
 * unit-tested without a DOM, and it is also what `react-refresh` requires (a
 * module exporting both a component and constants breaks fast refresh).
 */

/**
 * The French deck's Unicode glyphs, unchanged from the four maps this replaces.
 * The French deck ships no icon files at all: these glyphs are already sized,
 * coloured and kerned across four surfaces, and swapping them for images would
 * change French rendering for no reason. So `SuitMark` branches on **deck**,
 * not on asset availability.
 */
export const SUIT_GLYPH: Record<Suit, string> = {
  S: "♠",
  H: "♥",
  D: "♦",
  C: "♣",
};

/** Decks whose suit marks are image assets rather than Unicode glyphs. */
const ICON_DECKS: ReadonlySet<CardDeck> = new Set<CardDeck>(["croatian"]);

/**
 * Accent colour per deck per suit — the single source for the chrome that
 * surrounds a suit mark: orb border, radial halo, glow, tile ink, seal ring.
 *
 * The **French row reproduces today's values exactly**, so French rendering is
 * unchanged by this story. The literals are safe substitutes for the
 * `var(--suit-red, …)` / `var(--suit-black, …)` forms two of the call sites used
 * before: all four consumers render inside `.game-table`, where `index.css`
 * defines those tokens as precisely `#c62828` and `#1a1a1a`. (Outside that scope
 * the tokens are the parchment variants, which is why the marketing card and the
 * decorative divider keep using the vars and are not consumers here.)
 *
 * The **Croatian row is sampled from the artwork by coverage, not eyeballed** —
 * see `docs/card-deck.md`. Hearts are 30% `#e00020`; bells read gold (`#e0c020`)
 * over a dark outline; leaves are 25% `#204020`/`#406020`. Leaves **and** acorns
 * are both green-dominant, so acorns take the brown of their nut (`#a04000`) —
 * the conventional Grün/Eichel split. Collapsing them to one green would defeat
 * the whole point of a per-suit palette, which is why a test pins all four
 * pairwise distinct.
 *
 * This replaces the `H/D`-red / `S/C`-black split at every consumer. Painting a
 * gold bell with a red halo, or a green leaf with a black one, is the specific
 * defect Scope Amendment 1 exists to avoid.
 */
export const SUIT_ACCENT: Record<CardDeck, Record<Suit, string>> = {
  french: { S: "#1a1a1a", H: "#c62828", D: "#c62828", C: "#1a1a1a" },
  croatian: { S: "#4a7a3a", H: "#c62828", D: "#c9a23c", C: "#96501e" },
};

/**
 * Halo strength, as the two hex digits appended to an accent for a glow.
 *
 * These are **derived, not chosen**: alpha rises with the accent's WCAG relative
 * luminance, because a dark accent at a bright accent's strength reads as a
 * smudge under the parchment orb rather than a halo. Linear between French's two
 * pre-existing anchors, clamped to them:
 *
 *     L(#c62828) = 0.1368 -> 0x77      L(#1a1a1a) = 0.0103 -> 0x55
 *
 * That rule reproduces French's pre-existing asymmetry *exactly* (`55/77/77/55`),
 * which is the evidence it is the real design intent rather than a fitted curve —
 * and it is what `suitArt.test.ts` pins for both decks. Croatian's accents are
 * all more luminous than the French red, so three clamp to `77`; acorn brown is
 * the only one that lands below it. An earlier flat `66` was judgment, and it
 * under-lit three of the four suits against this rule.
 *
 * Lives here rather than in the consumer so the whole palette is one file, per
 * the spec's "orb border, radial halo, glow, glyph ink come from a per-deck
 * palette" constraint — but the consumer still decides *whether* it glows.
 */
export const SUIT_GLOW_ALPHA: Record<CardDeck, Record<Suit, string>> = {
  french: { S: "55", H: "77", D: "77", C: "55" },
  croatian: { S: "77", H: "77", D: "77", C: "73" },
};

/**
 * Suit ink for text that sits on **dark felt** rather than parchment.
 *
 * A second row is needed because {@link SUIT_ACCENT} is tuned for the cream orb
 * and the card-face tiles: `#c62828` on felt is nearly unreadable, which is what
 * `--suit-red-up` exists for. French reproduces its two pre-existing values
 * exactly (`#ff8585` on the reds, the parchment ink `#f5f2e8` on the blacks).
 *
 * Every value here is **measured against a contrast floor**, not eyeballed. The
 * bar is the weaker of French's own two anchors — `#ff8585` at **4.89:1** — taken
 * against `#20402b`, the *lightest* end of the reveal panel's gradient and so the
 * worst case for light ink. Croatian hearts deliberately reuses `#ff8585`: the
 * suit is red in both decks, so treating it differently would be gratuitous.
 *
 * Measured: leaves 6.27, hearts 4.89, bells 6.99, acorns 4.89.
 *
 * Acorns is the one value the measurement changed. It shipped as `#d69a63` while
 * these were judgment values — **4.73:1, under the bar** — and is now `#e19967`,
 * the lightest hex on its own hue that clears the floor rather than a rounded
 * guess (it passes by 0.005, so the floor assertion is strict, not fudged).
 * `suitArt.test.ts` pins that floor for every entry in both decks, so a future
 * edit cannot silently drop under it.
 *
 * Only `TrumpReveal`'s body copy needs this today. It lives here anyway so the
 * whole per-deck palette is one file to read and one file to change.
 */
export const SUIT_INK_ON_FELT: Record<CardDeck, Record<Suit, string>> = {
  french: { S: "#f5f2e8", H: "#ff8585", D: "#ff8585", C: "#f5f2e8" },
  croatian: { S: "#8fd07a", H: "#ff8585", D: "#e8c766", C: "#e19967" },
};

/**
 * URL of a deck's icon for one suit, or `null` for a deck that draws glyphs.
 *
 * Returning `null` rather than a would-be-404 path keeps the "French has no
 * icons" fact in the type system instead of in a comment: a caller has to handle
 * the glyph case, which is what `SuitMark` does.
 *
 * Built off `BASE_URL`, like `cardFaceUrl`, so the icons still resolve if the app
 * is served from a sub-path.
 */
export function suitIconUrl(suit: Suit, deck: CardDeck): string | null {
  if (!ICON_DECKS.has(deck)) return null;
  return `${import.meta.env.BASE_URL}suits/${deck}/${suit}.webp`;
}

/**
 * The colour a suit-keyed palette lookup falls back to.
 *
 * Needed because `Suit` is a compile-time union but not a runtime guarantee: the
 * WS contract types a suit as `z.string()` (`wsEvents.schemas.ts`), so a value
 * outside `S/H/D/C` reaches the UI unvalidated. A bare `SUIT_ACCENT[deck][suit]`
 * then yields `undefined`, and the consumers interpolate it — `border: "2px
 * solid undefined"` is dropped by the browser and the orb's whole chrome
 * vanishes. The old `suit === "H" || suit === "D"` test degraded to black for
 * anything unrecognised, so this is exactly that value: the accessors below keep
 * the pre-existing graceful degradation rather than regressing it.
 */
const FALLBACK_ACCENT = "#1a1a1a";
const FALLBACK_GLOW_ALPHA = "55";
const FALLBACK_INK_ON_FELT = "#f5f2e8";

/**
 * Read one suit's entry out of a per-deck palette row, or the fallback.
 *
 * `Object.hasOwn`, not `??`: a plain lookup walks the prototype chain, so
 * `row["toString"]` returns `Object.prototype.toString` — a FUNCTION, which is
 * neither null nor undefined and therefore sails straight past a `??` guard into
 * a style string. Same defect class as `resolveCardDeck`'s (`cardFace.ts`), found
 * by the test that pins these accessors against non-suit input.
 */
function paletteEntry(row: Record<Suit, string>, suit: Suit, fallback: string): string {
  return Object.hasOwn(row, suit) ? row[suit] : fallback;
}

/**
 * Accent for one suit on one deck. Use these accessors, never the raw tables —
 * they are what makes an unrecognised suit degrade instead of blanking the
 * chrome. See {@link FALLBACK_ACCENT}.
 */
export function suitAccent(suit: Suit, deck: CardDeck): string {
  return paletteEntry(SUIT_ACCENT[deck], suit, FALLBACK_ACCENT);
}

/** Halo strength for one suit on one deck. See {@link suitAccent}. */
export function suitGlowAlpha(suit: Suit, deck: CardDeck): string {
  return paletteEntry(SUIT_GLOW_ALPHA[deck], suit, FALLBACK_GLOW_ALPHA);
}

/** On-felt text ink for one suit on one deck. See {@link suitAccent}. */
export function suitInkOnFelt(suit: Suit, deck: CardDeck): string {
  return paletteEntry(SUIT_INK_ON_FELT[deck], suit, FALLBACK_INK_ON_FELT);
}

/**
 * The glyph for one suit, or an empty string for anything unrecognised.
 *
 * Same prototype-chain trap as {@link paletteEntry}: a bare `SUIT_GLYPH[suit]`
 * returns `Object.prototype.toString` for the key "toString", and React throws
 * outright on a function passed as a child ("Functions are not valid as a React
 * child") — so an unvalidated suit off the wire would crash the render rather
 * than degrade. An empty mark inside an intact orb is the graceful outcome.
 */
export function suitGlyph(suit: Suit): string {
  return Object.hasOwn(SUIT_GLYPH, suit) ? SUIT_GLYPH[suit] : "";
}

/**
 * i18n key for a suit's NAME, in the active deck's vocabulary.
 *
 * This is the seam that stops a gold bell being captioned "Diamonds". The name
 * is as load-bearing as the artwork: it is rendered as visible text beside the
 * trump orb and it is what a screen reader announces, so it has to follow the
 * deck exactly as the mark does (the spec's frozen `Always`: "`aria-label` names
 * the suit as the active deck depicts it, in the player's language").
 *
 * There is deliberately only ONE suit vocabulary in the locale files. An earlier
 * word-keyed `match.suits.spades` set duplicated these values verbatim in all
 * four locales with nothing able to detect the two drifting apart — the parity
 * test compares key SETS, not values.
 */
export function suitNameKey(suit: Suit, deck: CardDeck): string {
  return `match.card.suit.${deck}.${suit}`;
}

/**
 * i18n key for a rank's NAME. Deck-independent — both decks use the same eight
 * ranks with the same values — but it lives here beside {@link suitNameKey} so
 * the card vocabulary has one home. Replaces the word-keyed `match.ranks.*`.
 */
export function rankNameKey(rank: Rank): string {
  return `match.card.rank.${rank}`;
}
