import { Trans, useTranslation } from "react-i18next";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { Z } from "@/shared/lib/zLayers";
import type { Card, Suit } from "@/shared/types/matchTypes";

import { CARD_FACE_BACKGROUND, CARD_FACE_BORDER } from "../lib/cardFace";
import { type SeatTeam, teamColors } from "../lib/tableTheme";
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
  /**
   * Viewer-relative team of the player currently deciding trump. Colors their
   * name in the waiting banner — gold for you/your partner, silver for the
   * opponents — so waiting players can read at a glance whose decision it is.
   */
  activePlayerTeam?: SeatTeam | null;
  onPick: (suit?: Suit) => void;
  onPass: () => void;
  /**
   * Whether a pass is legal for the active bidder right now. False only for the
   * dealer bidding last in round 2 under a variant where the hand must find a
   * taker: the server refuses that pass outright, so the control must not be
   * offered. Server-derived (matchState.mustPickTrump) — the client never
   * re-derives the rule.
   *
   * Also drives the sibling seats' waiting copy, which otherwise promises the
   * table a pass that cannot happen.
   */
  canPass?: boolean;
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

/**
 * The four-suit picker grid. One definition serves every case that needs it:
 * Bitola round 2 (where the candidate's suit is locked out as "spent"), and
 * both rounds of a variant with no candidate at all (where nothing is locked
 * because no suit was ever offered and turned down).
 *
 * Card-style tiles: they take their face treatment from PlayingCard's exported
 * constants rather than restating it, so the tap target keeps reading as "pick
 * this suit's card" even if the deck face changes. Just the suit glyph — the
 * suit name is redundant next to a 60x80 card with a 40px symbol. A locked tile
 * stays in the grid, visibly disabled, so the layout doesn't shift and the
 * lock-out is explicit rather than a suit that silently vanished.
 */
function SuitPickerGrid({
  lockedSuit,
  onPick,
}: {
  lockedSuit: Suit | null;
  onPick: (suit: Suit) => void;
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
            aria-label={t(`match.suits.${suitName(suit)}`)}
            data-testid={`trump-prompt-suit-${suit}`}
            className="flex items-center justify-center rounded-md transition-[filter,transform] duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brass focus-visible:ring-offset-2 focus-visible:ring-offset-(--felt-deep,#072a14) disabled:cursor-not-allowed not-disabled:cursor-pointer not-disabled:hover:brightness-105 not-disabled:motion-safe:hover:-translate-y-0.5"
            style={{
              height: 52,
              background: CARD_FACE_BACKGROUND,
              border: CARD_FACE_BORDER,
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
  );
}

/**
 * Replaces the Pass button when the server says this seat has no legal pass.
 *
 * Deliberately shaped like a ClassicButton rather than styled as fine print, for
 * two reasons. First, ButtonTimerRing traces a rounded rect around whatever it
 * wraps at `radius` 8 — around a bare line of text that radius exceeds half the
 * height and the sweep degenerates into a lozenge, so the note needs
 * button-height padding and the same 8px corner for the ring to read correctly.
 * Second, it occupies the slot the player's eye is already on; a 0.55-opacity
 * caption there reads as decoration, and the one thing they must understand is
 * that the button they were about to press is gone on purpose.
 *
 * role="status" so the disappearance is announced rather than silently visual —
 * a screen-reader user otherwise just finds one fewer control.
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
        color: "var(--ink-light, #f5f2e8)",
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
  const showRing = isActiveBidder && Boolean(turnExpiresAt) && (timerDurationSec ?? 0) > 0;

  if (!isActiveBidder) {
    // The active bidder's name is bolded in their viewer-relative team color
    // (gold = you/partner, silver = opponents). `opacity` would dim the whole
    // line including the name, so the surrounding copy uses a translucent ink
    // *color* instead — keeping the team-colored name at full strength.
    const nameColor = activePlayerTeam
      ? teamColors(activePlayerTeam)[0]
      : "var(--ink-light, #f5f2e8)";
    // Surface all four suits as little parchment "suit chips" beside the copy
    // whenever the active bidder is choosing freely: Bitola round 2, where the
    // candidate suit is shown muted/disabled (it can't be picked) mirroring the
    // active bidder's locked tile, and both rounds of a candidate-less variant,
    // where nothing is locked.
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
            // shrink-0 so the flex row can't squash the card narrower than its
            // 44×64 footprint — without it the card compresses to ~32px wide,
            // stretching the aspect ratio and overflowing the centred pip.
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
                    ? // The active bidder has no pass to make, so the copy must
                      // not promise the table one. Same key family, forced
                      // variant.
                      "match.trumpPrompt.waitingForcedPick"
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
                      aria-label={t(`match.suits.${suitName(suit)}`)}
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
                        color: SUIT_COLOR[suit],
                        fontFamily: "var(--font-suit)",
                        fontSize: 15,
                        lineHeight: 1,
                        opacity: isLocked ? 0.4 : 1,
                        filter: isLocked ? "grayscale(0.85)" : undefined,
                      }}
                    >
                      <span aria-hidden="true">{SUIT_SYMBOL[suit]}</span>
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

  // A round-1 take-it-or-pass on a face-up candidate only exists when there IS a
  // candidate. Without one, trump is a freely named suit in BOTH rounds, so
  // round 1 gets the same four-suit grid as round 2 and the single "Take"
  // button — which sends no suit — must not render.
  const isFreeSuitPick = trumpCandidate === null;

  const title = isFreeSuitPick
    ? t("match.trumpPrompt.titleFreePick")
    : biddingRound === 1
      ? t("match.trumpPrompt.titleRound1")
      : t("match.trumpPrompt.titleRound2");

  // Subtitle copy stays generic — the candidate card is already rendered at
  // 80×116 directly below, so a "Candidate: T♣" prefix would just repeat in
  // text what the visual already shows.
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
          {biddingRound === 1 && trumpCandidate ? (
            // Round 1 with a candidate: the card sits on the left, descriptive
            // copy on the right — matches the design's 80×116 card + flex-1
            // paragraph. The decision is take-this-suit or pass, so no grid.
            <div className="flex items-center gap-5 mb-5">
              <PlayingCard card={trumpCandidate} state="default" size="lg" withTransition={false} />
              <p
                className="font-body text-[13px] leading-relaxed flex-1"
                style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.8 }}
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
              {/* With a candidate, its suit is spent and locked out. With none,
                  every suit is on the table and nothing is locked. */}
              <SuitPickerGrid lockedSuit={trumpCandidate?.suit ?? null} onPick={onPick} />
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
              {/* No Pass control when the server would refuse the pass: the
                  dealer bidding last in round 2 where the hand must find a
                  taker. Hiding it is the fix — offering a button whose only
                  outcome is a rejection toast is the defect, and a dedicated
                  error code for "you must pick" is explicitly not the answer.
                  The suit grid above is already the complete set of legal
                  moves, and the ring stays on the note in its place so the
                  countdown survives (an expiry here auto-PICKS server-side). */}
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
              {biddingRound === 1 && trumpCandidate && (
                // Suitless pick — the server binds trump to the candidate's
                // suit. Never rendered without a candidate: there would be
                // nothing for the server to bind to and the action is rejected.
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
