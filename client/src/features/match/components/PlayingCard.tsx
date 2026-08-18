import { useEffect } from "react";

import type { Card, CardId, Rank, Suit } from "@/shared/types/matchTypes";

import type { CardSize } from "../lib/cardFace";
import { CARD_FACE_BACKGROUND, CARD_SIZES, cardFaceUrl } from "../lib/cardFace";

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

const RANK_FULL_NAME: Record<Rank, string> = {
  "7": "Seven",
  "8": "Eight",
  "9": "Nine",
  T: "Ten",
  J: "Jack",
  Q: "Queen",
  K: "King",
  A: "Ace",
};

const SUIT_FULL_NAME: Record<Suit, string> = {
  S: "Spades",
  H: "Hearts",
  D: "Diamonds",
  C: "Clubs",
};

// Derived from the a11y label maps rather than declared again, so a second
// rank/suit table can't drift out of sync with the one the labels use.
const RANKS = Object.keys(RANK_FULL_NAME) as Rank[];
const SUITS = Object.keys(SUIT_FULL_NAME) as Suit[];

let deckWarmed = false;
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
 * Runs once per page load. Guarded on `document` rather than `Image` because
 * jsdom defines `Image` — it just never fetches — so an `Image` guard would not
 * keep this out of the test environment.
 */
function warmDeck(): void {
  if (deckWarmed || typeof document === "undefined") return;
  deckWarmed = true;

  for (const rank of RANKS) {
    for (const suit of SUITS) {
      const img = new Image();
      img.src = cardFaceUrl(`${rank}${suit}`);
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
 * Face-up cards render the vendored deck SVG for their ID over a parchment
 * background. The artwork's own face rect is transparent and its ink is already
 * retargeted onto `--suit-red` / `--suit-black` (see `scripts/recolor-cards.mjs`),
 * so there is no blend layer: the pips sit directly on {@link
 * CARD_FACE_BACKGROUND} at their exact token colour. That background therefore
 * doubles as the fallback — a face that has not loaded, or cannot, degrades to a
 * blank card rather than a hole in the hand.
 *
 * The image is decorative (`alt=""`, `aria-hidden`) — the wrapper's `aria-label`
 * is the one announcement — and pointer-transparent, so the wrapper owns every
 * click, key, and hover.
 *
 * Geometry comes from {@link CARD_SIZES}: an exact 5:7 box so the artwork is
 * never stretched, and a corner radius equal to the artwork's own so the two
 * edges coincide. The card draws no CSS border — the artwork strokes its own
 * outline, and a second one would leave a seam at the corners.
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
  const isFaceDown = state === "face-down" || card === null;
  const isPlayable = state === "playable";
  const isUnplayable = state === "unplayable";

  useEffect(() => {
    warmDeck();
  }, []);

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

  const ariaLabel = isFaceDown
    ? "face-down card"
    : `${RANK_FULL_NAME[card!.rank]} of ${SUIT_FULL_NAME[card!.suit]}`;

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
      {/* `object-fill` is the default, and deliberate: the face is authored with
          `preserveAspectRatio="none"`, and the box is an exact 5:7 to match, so
          filling the box is an identity scale rather than a distortion. */}
      <img
        src={cardFaceUrl(cardId!)}
        alt=""
        aria-hidden="true"
        draggable={false}
        className="pointer-events-none absolute inset-0 h-full w-full"
        // A missing asset does NOT 404 in production — the SPA fallback serves
        // index.html with a 200, which browsers paint as a broken-image glyph.
        // Hiding the element degrades to a blank parchment card instead.
        onError={(e) => {
          e.currentTarget.style.visibility = "hidden";
        }}
      />
    </div>
  );
}
