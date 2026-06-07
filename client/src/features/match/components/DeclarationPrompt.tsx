import { useTranslation } from "react-i18next";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { Z } from "@/shared/lib/zLayers";
import type { Declaration } from "@/shared/types/matchTypes";

import { declarationLabelKey } from "../lib/declarations";
import { ButtonTimerRing } from "./overlay/ButtonTimerRing";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";
import { PlayingCard } from "./PlayingCard";

interface DeclarationPromptProps {
  declarations: Declaration[];
  onDeclare: () => void;
  onSkip: () => void;
  turnExpiresAt?: string | null;
  timerDurationSec?: number;
  // Defensive component-level invariant: ring renders only when the viewer is
  // the active player. Caller (MatchPage) already gates this prompt on
  // activePlayerSeat === myPlayerSeat, so this defaults true.
  isActivePlayer?: boolean;
}

function declarationLabel(
  decl: Declaration,
  t: (key: string, opts?: Record<string, unknown>) => string,
): string {
  // Rules name only (Tierce / Quarte / Quint / Carré) — the points show on the
  // right of the row and in the total, so they're intentionally omitted here.
  return t(`match.declaration.${declarationLabelKey(decl.type, decl.cards.length)}`);
}

export function DeclarationPrompt({
  declarations,
  onDeclare,
  onSkip,
  turnExpiresAt,
  timerDurationSec,
  isActivePlayer = true,
}: DeclarationPromptProps) {
  const { t } = useTranslation();
  const promptRef = useFocusTrap<HTMLDivElement>();
  const total = declarations.reduce((sum, d) => sum + d.value, 0);
  const showRing = isActivePlayer && Boolean(turnExpiresAt) && (timerDurationSec ?? 0) > 0;

  return (
    <div className="fixed inset-0" style={{ zIndex: Z.PROMPT }} data-testid="declaration-prompt">
      <OverlayBackdrop dim={0.5}>
        <div
          ref={promptRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="declaration-prompt-title"
          className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            width={500}
            title={<span id="declaration-prompt-title">{t("match.declaration.title")}</span>}
          >
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

            <div className="flex justify-end items-center gap-3">
              {showRing ? (
                // Visual countdown only — server-authoritative auto-skip on
                // expiry. Same reasoning as BelotPrompt: a client onExpire
                // racing the server's auto-action surfaces a wrong-phase toast.
                <ButtonTimerRing
                  turnExpiresAt={turnExpiresAt}
                  totalDuration={timerDurationSec ?? 0}
                >
                  <ClassicButton onClick={onSkip} data-testid="declaration-prompt-skip">
                    {t("match.declaration.skip")}
                  </ClassicButton>
                </ButtonTimerRing>
              ) : (
                <ClassicButton onClick={onSkip} data-testid="declaration-prompt-skip">
                  {t("match.declaration.skip")}
                </ClassicButton>
              )}
              <ClassicButton
                variant="primary"
                onClick={onDeclare}
                data-testid="declaration-prompt-declare"
              >
                {t("match.declaration.declare")}
              </ClassicButton>
            </div>
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );
}
