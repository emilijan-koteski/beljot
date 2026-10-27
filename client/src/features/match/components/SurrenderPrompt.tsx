import { Flag } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { Z } from "@/shared/lib/zLayers";

import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { ExplainerStrip } from "./overlay/ExplainerStrip";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface SurrenderPromptProps {
  proposerUsername: string;
  onAccept: () => void;
  onDecline: () => void;
}

const DANGER = "var(--danger-light)";

export function SurrenderPrompt({ proposerUsername, onAccept, onDecline }: SurrenderPromptProps) {
  const { t } = useTranslation();
  const promptRef = useFocusTrap<HTMLDivElement>({ onEscape: onDecline });

  return (
    <div className="fixed inset-0" style={{ zIndex: Z.SURRENDER }} data-testid="surrender-prompt">
      <OverlayBackdrop dim="prompt">
        <div
          ref={promptRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="surrender-prompt-title"
          className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            size="dialog"
            glowColor={DANGER}
            title={
              <span id="surrender-prompt-title" className="inline-flex items-center gap-2.5">
                <Flag size={20} style={{ color: DANGER }} aria-hidden="true" />
                {t("match.surrender.prompt.title")}
              </span>
            }
          >
            <p
              className="text-sm mb-4"
              style={{ color: "var(--ink-light)", opacity: 0.82, lineHeight: 1.6 }}
            >
              {t("match.surrender.prompt.body", { username: proposerUsername })}
            </p>

            <ExplainerStrip tone="danger" className="mb-4">
              {t("match.surrender.prompt.consequence")}
            </ExplainerStrip>

            <div className="flex justify-end items-center gap-3.5">
              <ClassicButton
                variant="danger"
                onClick={onAccept}
                data-testid="surrender-prompt-accept"
              >
                {t("match.surrender.prompt.accept")}
              </ClassicButton>
              <ClassicButton onClick={onDecline} data-testid="surrender-prompt-decline">
                {t("match.surrender.prompt.decline")}
              </ClassicButton>
            </div>
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );
}
