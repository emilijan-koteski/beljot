import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import type { Declaration } from "@/shared/types/matchTypes";

import { declarationLabelKey } from "../lib/declarations";
import { ButtonTimerRing } from "./overlay/ButtonTimerRing";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { MeldRow } from "./overlay/MeldRow";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";
import { SummaryStrip } from "./overlay/SummaryStrip";

const AUTO_SKIP_SEC = MOTION.DECLARATION_PHASE_AUTO_SKIP / 1000;

interface DeclarationPromptProps {
  declarations: Declaration[];
  onDeclare: () => void;
  onSkip: () => void;
  turnExpiresAt?: string | null;
  timerDurationSec?: number;
  isActivePlayer?: boolean;
  simultaneous?: boolean;
  answered?: boolean;
  answeredCount?: number;
}

function declarationLabel(
  decl: Declaration,
  t: (key: string, opts?: Record<string, unknown>) => string,
): string {
  return t(`match.declaration.${declarationLabelKey(decl.type, decl.cards.length)}`);
}

/**
 * "Declare or skip" dialog, for the prompted seat in Bitola or for all four seats at once in the
 * Croatian simultaneous phase, where every seat sees the same panel so answering reveals nothing.
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

  const autoSkip = simultaneous && !hasDeclarations && !answered;
  useEffect(() => {
    if (autoSkip) sendOnce(onSkip);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoSkip]);

  const waiting = simultaneous && (answered || sent);
  const shownAnsweredCount = Math.min(4, answeredCount + (sent && !answered ? 1 : 0));

  const showRing = simultaneous
    ? !waiting
    : isActivePlayer && Boolean(turnExpiresAt) && (timerDurationSec ?? 0) > 0;

  const skipButton = (
    <ClassicButton onClick={handleSkip} disabled={waiting} data-testid="declaration-prompt-skip">
      {waiting
        ? t("match.declaration.waitingOthers", { answered: shownAnsweredCount })
        : t("match.declaration.skip")}
    </ClassicButton>
  );

  return (
    <div className="fixed inset-0" style={{ zIndex: Z.PROMPT }} data-testid="declaration-prompt">
      <OverlayBackdrop dim="prompt">
        <div
          ref={promptRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="declaration-prompt-title"
          tabIndex={-1}
          className="outline-none motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            size="dialog"
            title={
              <span id="declaration-prompt-title">
                {hasDeclarations ? t("match.declaration.title") : t("match.declaration.noneTitle")}
              </span>
            }
          >
            {hasDeclarations ? (
              <>
                <div className="flex flex-col gap-2 mb-3">
                  {declarations.map((decl, i) => (
                    <MeldRow
                      key={i}
                      label={declarationLabel(decl, t)}
                      cards={decl.cards}
                      value={decl.value}
                      testId="declaration-prompt-meld"
                    />
                  ))}
                </div>
                <SummaryStrip
                  label={t("match.declaration.total")}
                  value={`+${total}`}
                  testId="declaration-prompt-total"
                  className="mb-4"
                />
              </>
            ) : (
              <p
                className="text-sm leading-snug mb-4 py-6 text-center"
                style={{ color: "var(--ink-light)", opacity: 0.75 }}
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
