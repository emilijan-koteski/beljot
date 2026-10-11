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
  withTransition?: boolean;
  glowColor?: string;
}

const RANKS: Rank[] = ["7", "8", "9", "T", "J", "Q", "K", "A"];
const SUITS: Suit[] = ["S", "H", "D", "C"];

const decksWarmed = new Set<CardDeck>();
const warmed: HTMLImageElement[] = [];

/**
 * Preloads every face of a deck once per page load so faces not yet shown are cached.
 * Guarded on `document` because jsdom defines `Image` but never fetches.
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

const BACK_RADIUS = 6;

const DEFAULT_GLOW = "var(--turn-lime, #00e5a0)";
const DEFAULT_GLOW_HALO = "rgba(0, 229, 160, 0.8)";

/**
 * Playing card drawn from the player's active deck, in playable, unplayable, or face-down state.
 * The artwork is decorative; the wrapper owns the aria-label and all pointer and key events.
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

  const stateClasses = isPlayable
    ? "motion-safe:translate-y-[-10px] cursor-pointer motion-safe:hover:translate-y-[-14px] focus-visible:ring-2 focus-visible:ring-[color:var(--turn-lime)] focus-visible:ring-offset-2 focus-visible:ring-offset-[color:var(--felt-deep)]"
    : isUnplayable
      ? "motion-safe:translate-y-[4px] cursor-not-allowed"
      : "";

  const transitionClasses = withTransition
    ? "motion-safe:transition-transform motion-safe:duration-150"
    : "";

  const cardId: CardId | undefined = card ? `${card.rank}${card.suit}` : undefined;
  const faceUrl = cardId !== undefined ? cardFaceUrl(cardId, deck) : undefined;
  const role = isPlayable ? "button" : undefined;

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
        ...boxStyle,
        background: CARD_FACE_BACKGROUND,
        boxShadow: playableGlow,
      }}
    >
      <img
        key={faceUrl}
        src={faceUrl}
        alt=""
        aria-hidden="true"
        draggable={false}
        className="pointer-events-none absolute inset-0 h-full w-full"
        onError={(e) => {
          e.currentTarget.style.visibility = "hidden";
        }}
        onLoad={(e) => {
          e.currentTarget.style.visibility = "";
        }}
      />
    </div>
  );
}
