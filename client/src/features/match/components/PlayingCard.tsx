import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import { useAuthStore } from "@/shared/stores/authStore";
import type { Card, CardId, Rank, Suit } from "@/shared/types/matchTypes";

import type { CardDeck, CardSize } from "../lib/cardFace";
import { CARD_FACE_BACKGROUND, CARD_SIZES, cardFaceUrl, resolveCardDeck } from "../lib/cardFace";

export type CardState = "default" | "playable" | "unplayable" | "face-down";
export type { CardSize };

interface PlayingCardProps {
  card: Card | null;
  state: CardState;
  size: CardSize;
  onClick?: () => void;
  /** When false, suppress transition classes (e.g., cards in TrickArea). Defaults to true. */
  withTransition?: boolean;
  /**
   * Override the lime halo color used when `state === "playable"`. Default
   * lime; pass a different token (e.g. brass) for prompt previews where the
   * playable highlight needs a different meaning.
   */
  glowColor?: string;
}

// The deck's 32 IDs, as the two axes that produce them. Used both to warm the
// deck and (via the i18n key suffix) to name a card — the rank and suit
// CHARACTERS are the contract in both places, so there is no English table here
// to drift out of sync with the locale files.
const RANKS: Rank[] = ["7", "8", "9", "T", "J", "Q", "K", "A"];
const SUITS: Suit[] = ["S", "H", "D", "C"];

// Keyed by deck, not a single boolean: with one latch the FIRST deck mounted won
// for the page's lifetime, so a player who switched decks mid-session got no
// warm-up at all for the deck they were actually looking at.
const decksWarmed = new Set<CardDeck>();
// Preloads must stay reachable: a detached, still-loading `Image` is eligible
// for collection in some engines, which cancels the request mid-flight and warms
// the deck only partially.
const warmed: HTMLImageElement[] = [];

/**
 * Fetch the whole deck the first time any card mounts, so a face that has not
 * been shown yet is already cached when it appears — later tricks, the prompt
 * dialogs, and subsequent hands.
 *
 * This deliberately does NOT claim to fix the very first deal: it starts in the
 * same frame as those cards' own requests, so it cannot beat them. Making the
 * opening deal instant needs `<link rel="preload">` in `index.html` or warming
 * on match entry, which is a separate change.
 *
 * Runs once PER DECK per page load — switching decks mid-session warms the new
 * one, and switching back does not refetch. Guarded on `document` rather than
 * `Image` because jsdom defines `Image` — it just never fetches — so an `Image`
 * guard would not keep this out of the test environment.
 */
function warmDeck(deck: CardDeck): void {
  if (decksWarmed.has(deck) || typeof document === "undefined") return;
  decksWarmed.add(deck);

  for (const rank of RANKS) {
    for (const suit of SUITS) {
      const img = new Image();
      img.src = cardFaceUrl(`${rank}${suit}`, deck);
      warmed.push(img);
    }
  }
}

/** Corner radius for the face-down back, which has no artwork to match. */
const BACK_RADIUS = 6;

const DEFAULT_GLOW = "var(--turn-lime, #00e5a0)";
// Halo (alpha) twin of the default glow. Hardcoded as `rgba(...)` because
// CSS rejects `var(--x, #fff)cc` — you can't concatenate hex-alpha onto a
// `var()` reference, and that silently drops the whole `box-shadow`
// declaration. Caller-supplied glowColors fall back to this lime halo.
const DEFAULT_GLOW_HALO = "rgba(0, 229, 160, 0.8)";

/**
 * Classic casino-style playing card. Three states drive presentation:
 *  • `playable`   — lifted upward with a lime halo (turn / action channel).
 *  • `unplayable` — kept at full opacity (per design "do not make them
 *                    transparent"); cursor flips to not-allowed.
 *  • `face-down`  — parchment-on-wood back with the "B" monogram.
 *
 * Face-up cards render the face artwork for their ID out of the player's ACTIVE
 * DECK ({@link resolveCardDeck}) over a parchment background. On the French
 * deck the artwork's own face rect is transparent and its ink is already
 * retargeted onto `--suit-red` / `--suit-black` (see `scripts/recolor-cards.mjs`),
 * so there is no blend layer: the pips sit directly on {@link
 * CARD_FACE_BACKGROUND} at their exact token colour. (The Croatian deck is opaque
 * raster and simply covers it.) That background therefore doubles as the
 * fallback — a face that has not loaded, or cannot, degrades to a blank card
 * rather than a hole in the hand.
 *
 * The deck is the ONLY thing the preference changes. Card IDs, geometry, states,
 * glow, the face-down back and every `data-testid` are identical either way.
 *
 * The image is decorative (`alt=""`, `aria-hidden`) — the wrapper's `aria-label`
 * is the one announcement — and pointer-transparent, so the wrapper owns every
 * click, key, and hover.
 *
 * Geometry comes from {@link CARD_SIZES}: an exact 5:7 box so the artwork is
 * never stretched, and a corner radius equal to the artwork's own so the two
 * edges coincide — the Croatian faces are encoded to the same 5:7, so both decks
 * share it. The card draws no CSS border — the artwork strokes its own outline,
 * and a second one would leave a seam at the corners.
 *
 * The lime glow is driven by `.game-table` CSS vars, so tests rendering the card
 * standalone fall back to the literal hex in {@link DEFAULT_GLOW}.
 */
export function PlayingCard({
  card,
  state,
  size,
  onClick,
  withTransition = true,
  glowColor = DEFAULT_GLOW,
}: PlayingCardProps) {
  const { t } = useTranslation();
  // The one place the active deck is resolved. Subscribing through the store
  // selector is what makes a mid-match deck change re-skin the table with no
  // reload and no interruption to play: the preference write re-renders every
  // mounted card, and each one re-derives its own face URL.
  const deck = resolveCardDeck(useAuthStore((s) => s.user?.cardDeckPreference));

  const isFaceDown = state === "face-down" || card === null;
  const isPlayable = state === "playable";
  const isUnplayable = state === "unplayable";

  useEffect(() => {
    warmDeck(deck);
  }, [deck]);

  const handleClick = () => {
    if (isPlayable && onClick) {
      onClick();
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if ((e.key === "Enter" || e.key === " ") && isPlayable && onClick) {
      e.preventDefault();
      onClick();
    }
  };

  // Composed from a locale-owned template plus a PER-DECK suit name, so each
  // locale controls word order and case, and a Croatian card is never announced
  // as a French suit (a bells seven is "bundeva", not "karo").
  const ariaLabel = isFaceDown
    ? t("match.card.faceDown")
    : t("match.card.label", {
        rank: t(`match.card.rank.${card!.rank}`),
        suit: t(`match.card.suit.${deck}.${card!.suit}`),
      });

  const dims = CARD_SIZES[size];
  const boxStyle = {
    width: dims.width,
    height: dims.height,
    borderRadius: dims.radius,
  };

  // State-dependent positioning & cursor — classes the test suite asserts on.
  const stateClasses = isPlayable
    ? "motion-safe:translate-y-[-10px] cursor-pointer motion-safe:hover:translate-y-[-14px] focus-visible:ring-2 focus-visible:ring-[color:var(--turn-lime)] focus-visible:ring-offset-2 focus-visible:ring-offset-[color:var(--felt-deep)]"
    : isUnplayable
      ? "motion-safe:translate-y-[4px] cursor-not-allowed"
      : "";

  const transitionClasses = withTransition
    ? "motion-safe:transition-transform motion-safe:duration-150"
    : "";

  const cardId: CardId | undefined = card ? `${card.rank}${card.suit}` : undefined;
  // One expression feeding both `key` and `src`, so they can never disagree.
  const faceUrl = cardId !== undefined ? cardFaceUrl(cardId, deck) : undefined;
  const role = isPlayable ? "button" : undefined;

  // Lime halo on playable cards. Two channels: a 2 px ring for the green
  // border, and a soft 24 px halo for the glow. The ring uses `glowColor`
  // (which can be a `var()`); the halo uses its hardcoded rgba twin so we
  // don't re-introduce the dropped-shadow bug from concatenating hex alpha
  // onto a `var()` reference.
  const haloColor = glowColor === DEFAULT_GLOW ? DEFAULT_GLOW_HALO : glowColor;
  const playableGlow = isPlayable
    ? `0 10px 22px rgba(0,0,0,0.35), 0 0 0 2px ${glowColor}, 0 0 24px ${haloColor}`
    : isFaceDown
      ? "0 4px 10px rgba(0,0,0,0.45)"
      : "0 3px 6px rgba(0,0,0,0.3)";

  if (isFaceDown) {
    return (
      <div
        className={`relative select-none overflow-hidden ${stateClasses} ${transitionClasses}`}
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        tabIndex={isPlayable ? 0 : -1}
        role={role}
        aria-label={ariaLabel}
        aria-disabled={isUnplayable ? true : undefined}
        data-testid="playing-card-facedown"
        style={{
          ...boxStyle,
          // The back carries no artwork, so there is no `rx` for it to coincide
          // with — it keeps its own softer corner. Taking the face's radius here
          // would push the inset brass frame's arc outside the card's own.
          borderRadius: BACK_RADIUS,
          background: "linear-gradient(135deg, #2a1a10 0%, #4a2818 50%, #2a1a10 100%)",
          border: "2px solid var(--brass, #c9a876)",
          boxShadow: `${playableGlow}, inset 0 0 0 1px rgba(201,168,118,0.35)`,
        }}
      >
        <div
          className="absolute"
          style={{
            inset: 4,
            // Concentric with the outer corner: an inset frame's radius is the
            // outer radius minus the inset, never a fixed value.
            borderRadius: Math.max(0, BACK_RADIUS - 4),
            border: "1px solid rgba(201,168,118,0.45)",
            backgroundImage:
              "repeating-linear-gradient(45deg, transparent 0 6px, rgba(201,168,118,0.1) 6px 7px)",
          }}
        />
        <div
          className="absolute inset-0 flex items-center justify-center"
          style={{
            color: "var(--brass, #c9a876)",
            fontFamily: "var(--font-suit)",
            fontSize: dims.width * 0.3,
            fontStyle: "italic",
            fontWeight: 700,
            textShadow: "0 1px 2px rgba(0,0,0,0.6)",
          }}
        >
          B
        </div>
      </div>
    );
  }

  return (
    // `isolate` scopes the multiply layer below to this card. Without an
    // isolation boundary the blend resolves against whatever stacking context
    // happens to contain the card, which makes the face tone depend on the
    // surface it is dealt onto (felt, dialog, flight overlay).
    <div
      className={`relative select-none overflow-hidden ${stateClasses} ${transitionClasses}`}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      tabIndex={isPlayable ? 0 : -1}
      role={role}
      aria-label={ariaLabel}
      aria-disabled={isUnplayable ? true : undefined}
      data-testid={`playing-card-${cardId}`}
      style={{
        // This parchment IS the card face — the artwork's own face rect is
        // transparent (see scripts/recolor-cards.mjs), so the ink sits straight
        // on this gradient. No CSS border: the artwork strokes its own outline,
        // and radius is matched to the artwork's `rx` via CARD_SIZES.
        ...boxStyle,
        background: CARD_FACE_BACKGROUND,
        boxShadow: playableGlow,
      }}
    >
      {/* `object-fill` is the default, and deliberate: the French faces are
          authored with `preserveAspectRatio="none"` and the Croatian ones are
          encoded to 5:7 at import time, and the box is an exact 5:7 to match, so
          filling the box is an identity scale rather than a distortion. */}
      <img
        // Keyed by the resolved URL so a NEW node mounts whenever the source
        // changes. Without it React reuses this element across src changes, and
        // the imperative `visibility: hidden` below outlives the failure that
        // set it: one transient miss on the French face left the card blank for
        // the page's lifetime and survived switching to the Croatian deck, whose
        // asset was perfectly fine.
        key={faceUrl}
        src={faceUrl}
        alt=""
        aria-hidden="true"
        draggable={false}
        className="pointer-events-none absolute inset-0 h-full w-full"
        // A missing asset does NOT 404 in production — the SPA fallback serves
        // index.html with a 200, which browsers paint as a broken-image glyph.
        // Hiding the element degrades to a blank parchment card instead. Also
        // covers a deck whose artwork has not shipped yet.
        onError={(e) => {
          e.currentTarget.style.visibility = "hidden";
        }}
        // Clears a stale hide if the same node ever does resolve — belt to the
        // key's braces, and the only thing that recovers a node whose src did
        // not change.
        onLoad={(e) => {
          e.currentTarget.style.visibility = "";
        }}
      />
    </div>
  );
}
