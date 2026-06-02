import { useTranslation } from "react-i18next";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import type { Card, Suit } from "@/shared/types/matchTypes";

import { ButtonTimerRing } from "./overlay/ButtonTimerRing";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";
import { PlayingCard } from "./PlayingCard";

interface TrumpPromptProps {
  trumpCandidate: Card | null;
  biddingRound: number;
  isActiveBidder: boolean;
  /** Username of the player currently deciding trump — shown to everyone else. */
  activePlayerName?: string | null;
  onPick: (suit?: Suit) => void;
  onPass: () => void;
  turnExpiresAt?: string | null;
  timerDurationSec?: number;
}

const SUITS: Suit[] = ["S", "H", "D", "C"];

const SUIT_SYMBOL: Record<Suit, string> = {
  S: "♠",
  H: "♥",
  D: "♦",
  C: "♣",
};

// Card-on-white suit colors — these buttons read like miniature playing cards,
// so use the deep card-red (`--suit-red`) rather than the lifted variant
// (`--suit-red-up`) which is only legible on dark felt.
const SUIT_COLOR: Record<Suit, string> = {
  S: "var(--suit-black, #1a1a1a)",
  H: "var(--suit-red, #c62828)",
  D: "var(--suit-red, #c62828)",
  C: "var(--suit-black, #1a1a1a)",
};

export function TrumpPrompt({
  trumpCandidate,
  biddingRound,
  isActiveBidder,
  activePlayerName,
  onPick,
  onPass,
  turnExpiresAt,
  timerDurationSec,
}: TrumpPromptProps) {
  const { t } = useTranslation();
  const promptRef = useFocusTrap<HTMLDivElement>();
  const showRing = isActiveBidder && Boolean(turnExpiresAt) && (timerDurationSec ?? 0) > 0;

  if (!isActiveBidder) {
    return (
      <div
        className="absolute inset-0 flex items-center justify-center pointer-events-none z-20"
        data-testid="trump-prompt"
      >
        <div
          className="mx-4 max-w-[20rem] rounded-lg px-4 py-3 md:max-w-none"
          style={{
            background: "var(--panel-dark, rgba(20,45,30,0.85))",
            border: "1px solid rgba(201,168,118,0.4)",
            backdropFilter: "blur(8px)",
            WebkitBackdropFilter: "blur(8px)",
          }}
        >
          <p
            className="font-body text-sm text-center"
            style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.85 }}
          >
            {biddingRound === 1
              ? t("match.trumpPrompt.waitingRound1", { name: activePlayerName ?? "" })
              : t("match.trumpPrompt.waitingRound2", { name: activePlayerName ?? "" })}
          </p>
        </div>
      </div>
    );
  }

  const title =
    biddingRound === 1 ? t("match.trumpPrompt.titleRound1") : t("match.trumpPrompt.titleRound2");

  // Subtitle copy stays generic — the candidate card is already rendered at
  // 80×116 directly below, so a "Candidate: T♣" prefix would just repeat in
  // text what the visual already shows.
  const subtitle =
    biddingRound === 1
      ? t("match.trumpPrompt.subtitleRound1")
      : t("match.trumpPrompt.subtitleRound2");

  // Round 2: all four suits render so the layout is stable, but the
  // candidate suit (Bitola "spent suit") is disabled — visually muted and
  // not clickable. Keeping it on screen makes the lock-out explicit rather
  // than leaving the player guessing why a suit they thought was available
  // disappeared.

  return (
    <OverlayBackdrop dim={0.5}>
      <div
        ref={promptRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="trump-prompt-title"
        data-testid="trump-prompt"
        className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
      >
        {/* Scroll guard moves onto the panel itself. A wrapping div with
            overflow-y-auto would clip the panel's brass halo at its
            rectangular bounds (CSS forces overflow-x:auto when overflow-y is
            set). Inline `overflowY: auto` on the panel only overrides the Y
            axis — X stays hidden for corner clipping, and the panel's own
            box-shadow is unaffected by its own overflow. */}
        <ClassicPanel
          width={560}
          title={<span id="trump-prompt-title">{title}</span>}
          subtitle={subtitle}
          style={{ maxHeight: "90vh", overflowY: "auto" }}
        >
          {biddingRound === 1 ? (
            // Round 1: candidate card sits on the left, descriptive copy on
            // the right — matches the design's 80×116 card + flex-1 paragraph.
            trumpCandidate && (
              <div className="flex items-center gap-5 mb-5">
                <PlayingCard
                  card={trumpCandidate}
                  state="default"
                  size="lg"
                  withTransition={false}
                />
                <p
                  className="font-body text-[13px] leading-relaxed flex-1"
                  style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.8 }}
                >
                  {t("match.trumpPrompt.bodyRound1")}
                </p>
              </div>
            )
          ) : (
            <>
              {trumpCandidate && (
                <div className="flex justify-center mb-4">
                  <PlayingCard
                    card={trumpCandidate}
                    state="default"
                    size="lg"
                    withTransition={false}
                  />
                </div>
              )}
              {/* Card-style picker buttons: white parchment surface mirroring
                  PlayingCard so the tap target reads as "pick this suit's
                  card". Just the suit glyph — the suit name is redundant
                  next to a 60×80 white card with a 40 px symbol. The
                  candidate suit stays in the grid as a disabled tile so the
                  layout doesn't shift and the lock-out is visible. */}
              <div className="grid grid-cols-4 gap-2.5 mb-3.5">
                {SUITS.map((suit) => {
                  const isLocked = trumpCandidate?.suit === suit;
                  return (
                    <button
                      key={suit}
                      type="button"
                      disabled={isLocked}
                      aria-disabled={isLocked}
                      onClick={() => onPick(suit)}
                      aria-label={t(`match.suits.${suitName(suit)}`)}
                      data-testid={`trump-prompt-suit-${suit}`}
                      className="flex items-center justify-center rounded-md transition-[filter,transform] duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brass focus-visible:ring-offset-2 focus-visible:ring-offset-(--felt-deep,#072a14) disabled:cursor-not-allowed not-disabled:cursor-pointer not-disabled:hover:brightness-105 not-disabled:motion-safe:hover:-translate-y-0.5"
                      style={{
                        height: 52,
                        background: "linear-gradient(180deg, #fdfaf0 0%, #f4ecd8 100%)",
                        border: "1px solid rgba(0,0,0,0.15)",
                        boxShadow: "0 3px 6px rgba(0,0,0,0.3)",
                        color: SUIT_COLOR[suit],
                        fontFamily: "var(--font-suit)",
                        fontSize: 28,
                        lineHeight: 1,
                        opacity: isLocked ? 0.4 : 1,
                        filter: isLocked ? "grayscale(0.85)" : undefined,
                      }}
                    >
                      <span aria-hidden="true">{SUIT_SYMBOL[suit]}</span>
                    </button>
                  );
                })}
              </div>
            </>
          )}

          <div className="flex items-center justify-between gap-3">
            <span
              className="font-body text-[11px]"
              style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.55 }}
            >
              {t("match.trumpPrompt.roundLabel", {
                round: biddingRound,
                defaultValue: `Round ${biddingRound} / 2`,
              })}
            </span>
            <div className="flex items-center gap-3.5">
              {showRing ? (
                // Visual countdown only — server-authoritative auto-pass on
                // expiry. A client-side onExpire would race the server's
                // ActionPassTrump auto-action and surface a wrong-phase toast.
                <ButtonTimerRing
                  turnExpiresAt={turnExpiresAt}
                  totalDuration={timerDurationSec ?? 0}
                >
                  <ClassicButton onClick={onPass} data-testid="trump-prompt-pass">
                    {t("match.trumpPrompt.pass")}
                  </ClassicButton>
                </ButtonTimerRing>
              ) : (
                <ClassicButton onClick={onPass} data-testid="trump-prompt-pass">
                  {t("match.trumpPrompt.pass")}
                </ClassicButton>
              )}
              {biddingRound === 1 && (
                <ClassicButton
                  variant="primary"
                  onClick={() => onPick()}
                  data-testid="trump-prompt-pick"
                >
                  {t("match.trumpPrompt.pick")}
                </ClassicButton>
              )}
            </div>
          </div>
        </ClassicPanel>
      </div>
    </OverlayBackdrop>
  );
}

function suitName(suit: Suit): "spades" | "hearts" | "diamonds" | "clubs" {
  switch (suit) {
    case "S":
      return "spades";
    case "H":
      return "hearts";
    case "D":
      return "diamonds";
    case "C":
      return "clubs";
  }
}
