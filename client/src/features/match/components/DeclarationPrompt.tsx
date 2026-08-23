import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import type { Declaration } from "@/shared/types/matchTypes";

import { declarationLabelKey } from "../lib/declarations";
import { ButtonTimerRing } from "./overlay/ButtonTimerRing";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";
import { PlayingCard } from "./PlayingCard";

const AUTO_SKIP_SEC = MOTION.DECLARATION_PHASE_AUTO_SKIP / 1000;

interface DeclarationPromptProps {
  declarations: Declaration[];
  onDeclare: () => void;
  onSkip: () => void;
  /**
   * Server-driven expiry, used by Bitola's trick-1 prompt where the phase runs
   * on the per-move turn timer. Omitted in the dedicated declaration phase,
   * which has no active turn and runs its own fixed window instead.
   */
  turnExpiresAt?: string | null;
  timerDurationSec?: number;
  // Defensive component-level invariant: ring renders only when the viewer is
  // the active player. Caller (MatchPage) already gates this prompt on
  // activePlayerSeat === myPlayerSeat, so this defaults true. Irrelevant in the
  // simultaneous phase, where nobody is the active player.
  isActivePlayer?: boolean;
  /**
   * Simultaneous mode — the dedicated declaration phase. All four seats are
   * asked at once, so the dialog runs its own mount-anchored countdown, does
   * not close on the answer, and reports how many seats have answered.
   */
  simultaneous?: boolean;
  /** This viewer has answered and is waiting on the rest. Simultaneous only. */
  answered?: boolean;
  /** How many of the four seats have answered. Simultaneous only. */
  answeredCount?: number;
}

function declarationLabel(
  decl: Declaration,
  t: (key: string, opts?: Record<string, unknown>) => string,
): string {
  // Rules name only (Tierce / Quarte / Quint / Carré) — the points show on the
  // right of the row and in the total, so they're intentionally omitted here.
  return t(`match.declaration.${declarationLabelKey(decl.type, decl.cards.length)}`);
}

/**
 * "Declare or skip" dialog, in two modes.
 *
 * Bitola (`simultaneous` false) is the original: only the prompted seat sees
 * it, the ring tracks the server's per-move deadline, and answering closes it.
 *
 * The Croatian dedicated phase (`simultaneous` true) shows it to ALL FOUR seats
 * at once, including seats holding nothing — which is the point. The old phase
 * prompted meld holders one at a time, so the table could read who held a meld
 * off whose turn it was, and skipping hid nothing. Here every seat gets the same
 * panel with the same footprint; a seat with no melds gets an empty state and a
 * disabled Declare. The dialog then stays up in a waiting state until the phase
 * itself ends, so answering never closes anyone's screen early and reveals
 * nothing about what they held. (That a seat HAS answered is public — every
 * client counts it for this dialog — but what they answered is not, and the
 * melds themselves stay masked until the contest resolves.)
 */
export function DeclarationPrompt({
  declarations,
  onDeclare,
  onSkip,
  turnExpiresAt,
  timerDurationSec,
  isActivePlayer = true,
  simultaneous = false,
  answered = false,
  answeredCount = 0,
}: DeclarationPromptProps) {
  const { t } = useTranslation();
  const promptRef = useFocusTrap<HTMLDivElement>();
  const total = declarations.reduce((sum, d) => sum + d.value, 0);
  const hasDeclarations = declarations.length > 0;

  // Local send latch for the simultaneous phase. `answered` is server truth and
  // only arrives a round-trip after the click, while the ring's onExpire runs on
  // its own schedule — so a Skip clicked at 7.9s would be followed by the ring's
  // auto-skip at 8.0s, and the engine rejects that second answer with a
  // wrong-phase error the player would see as a toast for something they did
  // right. Latching locally makes the FIRST action the only one sent, exactly as
  // ScoreReveal's firedRef does for the score pause.
  const [sent, setSent] = useState(false);
  const sentRef = useRef(false);
  const sendOnce = (fn: () => void) => {
    if (!simultaneous) {
      fn();
      return;
    }
    if (sentRef.current) return;
    sentRef.current = true;
    setSent(true);
    fn();
  };
  const handleSkip = () => sendOnce(onSkip);
  const handleDeclare = () => sendOnce(onDeclare);

  // Waiting state: the viewer is done, whether or not the server has echoed it
  // back yet. The local half stops the dialog flickering back to live buttons
  // for the duration of the round-trip.
  const waiting = simultaneous && (answered || sent);

  // Bitola tracks the server's turn deadline; the simultaneous phase has none
  // (turnExpiresAt is null for its whole duration) and counts down from mount.
  const showRing = simultaneous
    ? !waiting
    : isActivePlayer && Boolean(turnExpiresAt) && (timerDurationSec ?? 0) > 0;

  const skipButton = (
    <ClassicButton onClick={handleSkip} disabled={waiting} data-testid="declaration-prompt-skip">
      {waiting
        ? // `answered`, not `count` — i18next reserves `count` for plural
          // resolution and would look for a `_other` suffixed key that does not
          // exist.
          t("match.declaration.waitingOthers", { answered: answeredCount })
        : t("match.declaration.skip")}
    </ClassicButton>
  );

  return (
    <div className="fixed inset-0" style={{ zIndex: Z.PROMPT }} data-testid="declaration-prompt">
      <OverlayBackdrop dim={0.5}>
        <div
          ref={promptRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="declaration-prompt-title"
          // Programmatically focusable so the trap always has somewhere to hold
          // focus. In the waiting state both buttons are disabled, and a modal
          // whose only focusable children are disabled drops focus to <body> and
          // stops containing Tab at all.
          tabIndex={-1}
          className="outline-none motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            width={500}
            title={
              <span id="declaration-prompt-title">
                {hasDeclarations ? t("match.declaration.title") : t("match.declaration.noneTitle")}
              </span>
            }
          >
            {hasDeclarations ? (
              <>
                <div className="flex flex-col gap-2 mb-4">
                  {declarations.map((decl, i) => (
                    <div
                      key={i}
                      className="flex flex-col gap-2 rounded-md px-3 py-2"
                      style={{
                        background: "rgba(0,0,0,0.22)",
                        border: "1px solid rgba(201,168,118,0.25)",
                      }}
                    >
                      <div className="flex items-center justify-between">
                        <span
                          className="font-body text-sm"
                          style={{ color: "var(--ink-light, #f5f2e8)" }}
                        >
                          {declarationLabel(decl, t)}
                        </span>
                        <span
                          className="font-display text-base font-semibold tabular-nums"
                          style={{ color: "var(--brass, #c9a876)" }}
                        >
                          {decl.value} {t("match.declaration.pts")}
                        </span>
                      </div>
                      <div className="flex flex-wrap gap-1">
                        {decl.cards.map((card) => (
                          <PlayingCard
                            key={`${card.rank}${card.suit}`}
                            card={card}
                            state="default"
                            size="sm"
                            withTransition={false}
                          />
                        ))}
                      </div>
                    </div>
                  ))}
                </div>

                <div
                  className="flex items-center justify-between pt-3 mb-4"
                  style={{ borderTop: "1px solid rgba(201,168,118,0.22)" }}
                  data-testid="declaration-prompt-total"
                >
                  <span
                    className="font-body text-sm"
                    style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
                  >
                    {t("match.declaration.total")}
                  </span>
                  <span
                    className="font-display text-lg font-bold tabular-nums"
                    style={{ color: "var(--brass, #c9a876)" }}
                  >
                    {total} {t("match.declaration.pts")}
                  </span>
                </div>
              </>
            ) : (
              // Empty state — same panel, same width, same button row. A seat
              // holding nothing must not be distinguishable from a seat holding
              // a carré by anything but its own screen.
              <p
                className="font-body text-sm leading-snug mb-4 py-6 text-center"
                style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.75 }}
                data-testid="declaration-prompt-none"
              >
                {t("match.declaration.noneBody")}
              </p>
            )}

            <div className="flex justify-end items-center gap-3">
              <ClassicButton
                variant="primary"
                onClick={handleDeclare}
                disabled={!hasDeclarations || waiting}
                data-testid="declaration-prompt-declare"
              >
                {t("match.declaration.declare")}
              </ClassicButton>
              {showRing ? (
                // Bitola: visual countdown only — the server auto-skips on the
                // turn timer, and a client onExpire racing it surfaces a
                // wrong-phase toast (same reasoning as BelotPrompt).
                //
                // Simultaneous: the client IS the actor, exactly as ScoreReveal
                // is for the score pause. It auto-skips at the ring's end so an
                // AFK seat can't hold the table, and the server's force-close
                // ceiling sits well beyond this window rather than racing it.
                <ButtonTimerRing
                  turnExpiresAt={simultaneous ? null : turnExpiresAt}
                  totalDuration={simultaneous ? AUTO_SKIP_SEC : (timerDurationSec ?? 0)}
                  clientCountdown={simultaneous}
                  onExpire={simultaneous ? handleSkip : undefined}
                >
                  {skipButton}
                </ButtonTimerRing>
              ) : (
                skipButton
              )}
            </div>
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );
}
