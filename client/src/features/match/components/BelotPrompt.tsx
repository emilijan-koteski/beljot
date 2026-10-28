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
  isRebelote: boolean;
  trumpSuit?: Suit;
  onAnnounce: () => void;
  onDecline: () => void;
  turnExpiresAt?: string | null;
  timerDurationSec?: number;
  isActivePlayer?: boolean;
}

export function BelotPrompt({
  isKing,
  isRebelote,
  trumpSuit = "S",
  onAnnounce,
  onDecline,
  turnExpiresAt,
  timerDurationSec,
  isActivePlayer = true,
}: BelotPromptProps) {
  const { t } = useTranslation();
  const promptRef = useFocusTrap<HTMLDivElement>({ onEscape: onDecline });
  const showRing = isActivePlayer && Boolean(turnExpiresAt) && (timerDurationSec ?? 0) > 0;

  const titleKey = isRebelote ? "match.belot.titleRebelot" : "match.belot.titleBelot";

  return (
    <div className="fixed inset-0" style={{ zIndex: Z.PROMPT }} data-testid="belot-prompt">
      <OverlayBackdrop dim="prompt">
        <div
          ref={promptRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="belot-prompt-title"
          className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            size="dialog"
            title={<span id="belot-prompt-title">{t(titleKey)}</span>}
            subtitle={t("match.belot.subtitle")}
          >
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

            <div className="flex justify-end items-center gap-3.5">
              <ClassicButton
                variant="primary"
                onClick={onAnnounce}
                data-testid="belot-prompt-announce"
              >
                {t("match.belot.announce")}
              </ClassicButton>
              {showRing ? (
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
            </div>
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );
}
