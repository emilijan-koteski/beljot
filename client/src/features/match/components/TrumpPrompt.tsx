import { Trans, useTranslation } from "react-i18next";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { Z } from "@/shared/lib/zLayers";
import { useAuthStore } from "@/shared/stores/authStore";
import type { Card, Suit } from "@/shared/types/matchTypes";

import type { CardDeck } from "../lib/cardFace";
import { CARD_FACE_BACKGROUND, CARD_FACE_BORDER, resolveCardDeck } from "../lib/cardFace";
import { suitAccent, suitNameKey } from "../lib/suitArt";
import { type SeatTeam, teamColors } from "../lib/tableTheme";
import { ButtonTimerRing } from "./overlay/ButtonTimerRing";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";
import { PlayingCard } from "./PlayingCard";
import { SuitMark } from "./SuitMark";

interface TrumpPromptProps {
  trumpCandidate: Card | null;
  biddingRound: number;
  isActiveBidder: boolean;
  activePlayerName?: string | null;
  activePlayerTeam?: SeatTeam | null;
  onPick: (suit?: Suit) => void;
  onPass: () => void;
  canPass?: boolean;
  turnExpiresAt?: string | null;
  timerDurationSec?: number;
}

const SUITS: Suit[] = ["S", "H", "C", "D"];

/**
 * Card-style four-suit picker for free trump naming, used in Bitola round 2 and candidate-less variants.
 * A locked suit stays in the grid as a disabled tile so the layout does not shift.
 */
function SuitPickerGrid({
  lockedSuit,
  onPick,
  deck,
}: {
  lockedSuit: Suit | null;
  onPick: (suit: Suit) => void;
  deck: CardDeck;
}) {
  const { t } = useTranslation();
  return (
    <div className="grid grid-cols-4 gap-2.5 mb-3.5">
      {SUITS.map((suit) => {
        const isLocked = lockedSuit === suit;
        return (
          <button
            key={suit}
            type="button"
            disabled={isLocked}
            aria-disabled={isLocked}
            onClick={() => onPick(suit)}
            aria-label={t(suitNameKey(suit, deck))}
            data-testid={`trump-prompt-suit-${suit}`}
            className="flex items-center justify-center rounded-md transition-[filter,transform] duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brass focus-visible:ring-offset-2 focus-visible:ring-offset-(--felt-deep,#072a14) disabled:cursor-not-allowed not-disabled:cursor-pointer not-disabled:hover:brightness-105 not-disabled:motion-safe:hover:-translate-y-0.5"
            style={{
              height: 52,
              background: CARD_FACE_BACKGROUND,
              border: CARD_FACE_BORDER,
              boxShadow: "0 3px 6px rgba(0,0,0,0.3)",
              color: suitAccent(suit, deck),
              fontFamily: "var(--font-suit)",
              fontSize: 28,
              lineHeight: 1,
              opacity: isLocked ? 0.4 : 1,
              filter: isLocked ? "grayscale(0.85)" : undefined,
            }}
          >
            <SuitMark suit={suit} deck={deck} size={28} />
          </button>
        );
      })}
    </div>
  );
}

/**
 * Button-shaped status note that replaces the Pass button when the server says this seat has no legal pass.
 * It keeps button height and corner radius so the wrapping ButtonTimerRing still traces it correctly.
 */
function MustPickNote() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      data-testid="trump-prompt-must-pick"
      className="inline-flex items-center px-3.5 py-2 text-[13px] sm:px-4.5 sm:py-2.5 sm:text-[14px]"
      style={{
        border: "1px dashed rgba(201,168,118,0.45)",
        borderRadius: 8,
        background: "linear-gradient(180deg, rgba(60,90,70,0.35), rgba(30,50,35,0.35))",
        color: "var(--ink-light)",
        fontFamily: "var(--font-body)",
        fontWeight: 600,
        letterSpacing: 0.4,
      }}
    >
      {t("match.trumpPrompt.mustPick")}
    </div>
  );
}

export function TrumpPrompt({
  trumpCandidate,
  biddingRound,
  isActiveBidder,
  activePlayerName,
  activePlayerTeam,
  onPick,
  onPass,
  canPass = true,
  turnExpiresAt,
  timerDurationSec,
}: TrumpPromptProps) {
  const { t } = useTranslation();
  const promptRef = useFocusTrap<HTMLDivElement>();
  const deck = resolveCardDeck(useAuthStore((s) => s.user?.cardDeckPreference));
  const showRing = isActiveBidder && Boolean(turnExpiresAt) && (timerDurationSec ?? 0) > 0;

  if (!isActiveBidder) {
    const nameColor = activePlayerTeam ? teamColors(activePlayerTeam)[0] : "var(--ink-light)";
    const showSuitChips = biddingRound === 2 || trumpCandidate === null;

    return (
      <div
        className="absolute inset-0 flex items-center justify-center pointer-events-none"
        style={{ zIndex: Z.PROMPT }}
        data-testid="trump-prompt"
      >
        <div
          className="mx-4 flex max-w-[24rem] items-center gap-3 rounded-lg px-4 py-3"
          style={{
            background: "var(--panel-dark, rgba(20,45,30,0.85))",
            border: "1px solid rgba(201,168,118,0.4)",
            backdropFilter: "blur(8px)",
            WebkitBackdropFilter: "blur(8px)",
          }}
        >
          {trumpCandidate && (
            <div className="shrink-0">
              <PlayingCard card={trumpCandidate} state="default" size="sm" withTransition={false} />
            </div>
          )}
          <div className="flex min-w-0 flex-col gap-2">
            <p
              className="font-body text-sm leading-snug"
              style={{ color: "rgba(245,242,232,0.85)" }}
            >
              <Trans
                i18nKey={
                  !canPass
                    ? "match.trumpPrompt.waitingForcedPick"
                    : trumpCandidate === null || biddingRound === 2
                      ? "match.trumpPrompt.waitingRound2"
                      : "match.trumpPrompt.waitingRound1"
                }
                values={{ name: activePlayerName ?? "" }}
                components={{ name: <strong style={{ color: nameColor, fontWeight: 700 }} /> }}
              />
            </p>
            {showSuitChips && (
              <div className="flex items-center gap-1.5" data-testid="trump-prompt-considering">
                {SUITS.map((suit) => {
                  const isLocked = trumpCandidate?.suit === suit;
                  return (
                    <span
                      key={suit}
                      aria-label={t(suitNameKey(suit, deck))}
                      aria-disabled={isLocked}
                      data-locked={isLocked ? "true" : undefined}
                      data-testid={`trump-prompt-considering-${suit}`}
                      className="inline-flex shrink-0 items-center justify-center rounded-[5px]"
                      style={{
                        width: 22,
                        height: 30,
                        background: CARD_FACE_BACKGROUND,
                        border: CARD_FACE_BORDER,
                        boxShadow: "0 1px 3px rgba(0,0,0,0.35)",
                        color: suitAccent(suit, deck),
                        fontFamily: "var(--font-suit)",
                        fontSize: 15,
                        lineHeight: 1,
                        opacity: isLocked ? 0.4 : 1,
                        filter: isLocked ? "grayscale(0.85)" : undefined,
                      }}
                    >
                      <SuitMark suit={suit} deck={deck} size={15} />
                    </span>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </div>
    );
  }

  const isFreeSuitPick = trumpCandidate === null;

  const title = isFreeSuitPick
    ? t("match.trumpPrompt.titleFreePick")
    : biddingRound === 1
      ? t("match.trumpPrompt.titleRound1")
      : t("match.trumpPrompt.titleRound2");

  const subtitle = isFreeSuitPick
    ? t("match.trumpPrompt.subtitleFreePick")
    : biddingRound === 1
      ? t("match.trumpPrompt.subtitleRound1")
      : t("match.trumpPrompt.subtitleRound2");

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
        <ClassicPanel
          width={560}
          title={<span id="trump-prompt-title">{title}</span>}
          subtitle={subtitle}
          style={{ maxHeight: "90vh", overflowY: "auto" }}
        >
          {biddingRound === 1 && trumpCandidate ? (
            <div className="flex items-center gap-5 mb-5">
              <PlayingCard card={trumpCandidate} state="default" size="lg" withTransition={false} />
              <p
                className="font-body text-[13px] leading-relaxed flex-1"
                style={{ color: "var(--ink-light)", opacity: 0.8 }}
              >
                {t("match.trumpPrompt.bodyRound1")}
              </p>
            </div>
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
              <SuitPickerGrid
                lockedSuit={trumpCandidate?.suit ?? null}
                onPick={onPick}
                deck={deck}
              />
            </>
          )}

          <div className="flex items-center justify-between gap-3">
            <span
              className="font-body text-[11px]"
              style={{ color: "var(--ink-light)", opacity: 0.55 }}
            >
              {trumpCandidate !== null &&
                t("match.trumpPrompt.roundLabel", {
                  round: biddingRound,
                  defaultValue: `Round ${biddingRound} / 2`,
                })}
            </span>
            <div className="flex items-center gap-3.5">
              {biddingRound === 1 && trumpCandidate && (
                <ClassicButton
                  variant="primary"
                  onClick={() => onPick()}
                  data-testid="trump-prompt-pick"
                >
                  {t("match.trumpPrompt.pick")}
                </ClassicButton>
              )}
              {!canPass ? (
                showRing ? (
                  <ButtonTimerRing
                    turnExpiresAt={turnExpiresAt}
                    totalDuration={timerDurationSec ?? 0}
                  >
                    <MustPickNote />
                  </ButtonTimerRing>
                ) : (
                  <MustPickNote />
                )
              ) : showRing ? (
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
            </div>
          </div>
        </ClassicPanel>
      </div>
    </OverlayBackdrop>
  );
}
