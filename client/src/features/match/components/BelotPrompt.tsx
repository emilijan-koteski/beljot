import { useTranslation } from "react-i18next";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { Z } from "@/shared/lib/zLayers";
import type { Suit } from "@/shared/types/matchTypes";

import { ButtonTimerRing } from "./overlay/ButtonTimerRing";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";
import { PlayingCard } from "./PlayingCard";

interface BelotPromptProps {
  isKing: boolean;
  /**
   * Trump suit — drives which Q/K to render. Optional so existing test
   * renders that don't care about the cards still work; falls back to
   * spades for purely visual purposes.
   */
  trumpSuit?: Suit;
  onAnnounce: () => void;
  onDecline: () => void;
  turnExpiresAt?: string | null;
  timerDurationSec?: number;
  // Defensive component-level invariant: ring renders only when the viewer is
  // the active player. Caller (MatchPage) already gates this prompt on
  // pendingBelotSeat === myPlayerSeat, so this defaults true; any future
  // caller that mounts the prompt for a non-active viewer must opt out.
  isActivePlayer?: boolean;
}

export function BelotPrompt({
  isKing,
  trumpSuit = "S",
  onAnnounce,
  onDecline,
  turnExpiresAt,
  timerDurationSec,
  isActivePlayer = true,
}: BelotPromptProps) {
  const { t } = useTranslation();
  // Escape declines the announcement and forfeits the +20 bonus per the rule.
  // fixed inset-0 prevents leak through scrolled stacking.
  const promptRef = useFocusTrap<HTMLDivElement>({ onEscape: onDecline });
  const showRing = isActivePlayer && Boolean(turnExpiresAt) && (timerDurationSec ?? 0) > 0;

  const titleKey = isKing ? "match.belot.titleRebelot" : "match.belot.titleBelot";
  const announceKey = isKing ? "match.belot.announceRebelot" : "match.belot.announceBelot";
  const subtitleKey = isKing ? "match.belot.subtitleRebelot" : "match.belot.subtitleBelot";
  const accentStripKey = isKing ? "match.belot.accentStripRebelot" : "match.belot.accentStripBelot";

  return (
    <div className="fixed inset-0" style={{ zIndex: Z.PROMPT }} data-testid="belot-prompt">
      <OverlayBackdrop dim={0.5}>
        <div
          ref={promptRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="belot-prompt-title"
          className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            width={460}
            title={<span id="belot-prompt-title">{t(titleKey)}</span>}
            subtitle={t(subtitleKey)}
          >
            {/* Q + K of trump side-by-side. The card matching the just-played
                K/Q lifts up with the lime playable halo so the player sees
                exactly which card the announcement is anchored to. */}
            <div className="flex justify-center items-end gap-3.5 mt-2 mb-5">
              <PlayingCard
                card={{ rank: "Q", suit: trumpSuit }}
                state={isKing ? "default" : "playable"}
                size="md"
                withTransition={false}
              />
              <PlayingCard
                card={{ rank: "K", suit: trumpSuit }}
                state={isKing ? "playable" : "default"}
                size="md"
                withTransition={false}
              />
            </div>

            {/* Brass-bordered accent strip — small felt callout reminding the
                player of the +20 swing if they announce. Sits in the same
                family as the panel chrome. */}
            <div
              className="mb-3.5 px-3 py-2 rounded-lg flex items-center gap-2 font-body"
              style={{
                border: "1px solid rgba(201,168,118,0.32)",
                background: "linear-gradient(90deg, rgba(20,46,28,0.55), rgba(14,40,24,0.35))",
                color: "#e8dfc8",
                fontSize: 12,
                boxShadow: "inset 0 1px 0 rgba(201,168,118,0.12)",
              }}
            >
              <span
                aria-hidden
                className="rounded-full"
                style={{
                  width: 7,
                  height: 7,
                  background: "var(--brass, #c9a876)",
                  boxShadow: "0 0 6px rgba(201,168,118,0.55)",
                }}
              />
              {t(accentStripKey)}
            </div>

            <div className="flex justify-end items-center gap-3.5">
              {showRing ? (
                // Visual countdown only — the server runs its own per-move
                // timer and auto-skips the belot announcement on expiry. The
                // resulting state push dismisses this prompt, so a client-side
                // onExpire would race the server and surface a wrong-phase
                // error toast for an action the user did not take.
                <ButtonTimerRing
                  turnExpiresAt={turnExpiresAt}
                  totalDuration={timerDurationSec ?? 0}
                >
                  <ClassicButton onClick={onDecline} data-testid="belot-prompt-decline">
                    {t("match.belot.decline")}
                  </ClassicButton>
                </ButtonTimerRing>
              ) : (
                <ClassicButton onClick={onDecline} data-testid="belot-prompt-decline">
                  {t("match.belot.decline")}
                </ClassicButton>
              )}
              <ClassicButton
                variant="primary"
                onClick={onAnnounce}
                data-testid="belot-prompt-announce"
              >
                {t(announceKey)}
              </ClassicButton>
            </div>
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );
}
