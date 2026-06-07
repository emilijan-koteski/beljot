import { Flag } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { Z } from "@/shared/lib/zLayers";

import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface SurrenderPromptProps {
  proposerUsername: string;
  onAccept: () => void;
  onDecline: () => void;
}

const DANGER = "#ff8585";

export function SurrenderPrompt({ proposerUsername, onAccept, onDecline }: SurrenderPromptProps) {
  const { t } = useTranslation();
  // Escape declines the partner-accept (D102 / D103). fixed inset-0 prevents
  // the prompt from leaking through scrolled/zoomed parent stacking contexts.
  const promptRef = useFocusTrap<HTMLDivElement>({ onEscape: onDecline });

  return (
    <div className="fixed inset-0" style={{ zIndex: Z.SURRENDER }} data-testid="surrender-prompt">
      <OverlayBackdrop dim={0.5}>
        <div
          ref={promptRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="surrender-prompt-title"
          className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            width={480}
            glowColor={DANGER}
            title={
              <span id="surrender-prompt-title" className="inline-flex items-center gap-2.5">
                <Flag size={18} style={{ color: DANGER }} aria-hidden="true" />
                {t("match.surrender.prompt.title")}
              </span>
            }
          >
            <p
              className="font-body text-sm mb-4"
              style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.82, lineHeight: 1.55 }}
            >
              {t("match.surrender.prompt.body", { username: proposerUsername })}
            </p>

            {/* Danger accent strip — mirrors the proposer's confirm dialog so
                both sides of the surrender flow share visual language. */}
            <div
              className="mb-4 px-3 py-2 rounded-lg flex items-center gap-2 font-body"
              style={{
                border: "1px solid rgba(255,133,133,0.35)",
                background: "linear-gradient(90deg, rgba(80,28,28,0.55), rgba(50,18,18,0.35))",
                color: "#ffe1d6",
                fontSize: 12,
                boxShadow: "inset 0 1px 0 rgba(255,133,133,0.14)",
              }}
            >
              <span
                aria-hidden
                className="rounded-full"
                style={{
                  width: 7,
                  height: 7,
                  background: DANGER,
                  boxShadow: "0 0 6px rgba(255,133,133,0.6)",
                  flexShrink: 0,
                }}
              />
              {t("match.surrender.prompt.consequence")}
            </div>

            <div className="flex justify-end items-center gap-3.5">
              <ClassicButton onClick={onDecline} data-testid="surrender-prompt-decline">
                {t("match.surrender.prompt.decline")}
              </ClassicButton>
              <ClassicButton
                variant="primary"
                onClick={onAccept}
                data-testid="surrender-prompt-accept"
                style={{
                  background: "linear-gradient(180deg, #d96a5a 0%, #8a3024 100%)",
                  borderColor: "rgba(255,133,133,0.7)",
                  color: "#fff8f3",
                  boxShadow:
                    "0 4px 10px rgba(217,106,90,0.4), inset 0 1px 0 rgba(255,255,255,0.25)",
                }}
              >
                {t("match.surrender.prompt.accept")}
              </ClassicButton>
            </div>
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );
}
