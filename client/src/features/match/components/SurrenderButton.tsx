import { Flag } from "lucide-react";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";

import { HUDButton } from "./HUDButton";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface SurrenderButtonProps {
  canRequest: boolean;
  isExhausted: boolean;
  isPending: boolean;
  onConfirm: () => void;
}

const DANGER = "#ff8585";

export function SurrenderButton({
  canRequest,
  isExhausted,
  isPending,
  onConfirm,
}: SurrenderButtonProps) {
  const { t } = useTranslation();
  const [confirmOpen, setConfirmOpen] = useState(false);

  // If a partner's proposal arrives or the player's attempt becomes exhausted
  // while the confirm dialog is open, close it — the action would be doomed.
  useEffect(() => {
    if (isPending || isExhausted) setConfirmOpen(false);
  }, [isPending, isExhausted]);

  // Exhausted state takes priority — once consumed, that's the durable
  // condition; pending only describes a transient partner-side proposal.
  // The aria-label carries the verbose status; the button itself is icon-only.
  const ariaLabel = isExhausted
    ? t("match.surrender.exhausted")
    : isPending
      ? t("match.surrender.pending")
      : t("match.surrender.requestButton");

  const isDisabled = !canRequest;

  const handleConfirm = () => {
    setConfirmOpen(false);
    onConfirm();
  };

  return (
    <>
      <HUDButton
        variant="danger"
        icon={<Flag className="h-4 w-4" aria-hidden="true" />}
        aria-label={ariaLabel}
        title={ariaLabel}
        onClick={() => setConfirmOpen(true)}
        disabled={isDisabled}
        data-testid="surrender-button"
      />

      {confirmOpen && (
        <SurrenderConfirmDialog onCancel={() => setConfirmOpen(false)} onConfirm={handleConfirm} />
      )}
    </>
  );
}

interface SurrenderConfirmDialogProps {
  onCancel: () => void;
  onConfirm: () => void;
}

/**
 * The surrender confirmation modal. Exported so surfaces that can't keep the
 * SurrenderButton mounted (e.g. the mobile HUD menu, which closes on tap) can
 * drive the confirm dialog directly from their own state.
 */
export function SurrenderConfirmDialog({ onCancel, onConfirm }: SurrenderConfirmDialogProps) {
  const { t } = useTranslation();
  // Escape cancels the proposal — matches BelotPrompt's onEscape→decline
  // convention so destructive overlays bail out safely on a stray keystroke.
  const dialogRef = useFocusTrap<HTMLDivElement>({ onEscape: onCancel });

  // Portal to document.body — the SurrenderButton sits inside the bottom-left
  // HUD cluster (z-10), which would otherwise cap the overlay's stacking
  // context and hide the dialog behind table elements like the bidder banner
  // (z-20). Rendering at the body level lets the OverlayBackdrop's z-50
  // float above the entire game-table.
  const dialog = (
    <div className="fixed inset-0 z-50" data-testid="surrender-confirm-overlay">
      <OverlayBackdrop dim={0.5}>
        <div
          ref={dialogRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="surrender-confirm-title"
          className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            width={460}
            glowColor={DANGER}
            title={
              <span id="surrender-confirm-title" className="inline-flex items-center gap-2.5">
                <Flag size={18} style={{ color: DANGER }} aria-hidden="true" />
                {t("match.surrender.confirm.title")}
              </span>
            }
          >
            <p
              className="font-body text-sm mb-4"
              style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.78, lineHeight: 1.55 }}
            >
              {t("match.surrender.confirm.body")}
            </p>

            {/* Danger accent strip — red counterpart to the brass strip used
                for celebratory announcements (BelotPrompt). Reinforces that
                this is a one-shot, destructive action. */}
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
              {t("match.surrender.confirm.consumeWarning")}
            </div>

            <div className="flex justify-end items-center gap-3.5">
              <ClassicButton onClick={onCancel} data-testid="surrender-cancel">
                {t("match.surrender.confirm.cancel")}
              </ClassicButton>
              <ClassicButton
                variant="primary"
                onClick={onConfirm}
                data-testid="surrender-confirm"
                style={{
                  background: "linear-gradient(180deg, #d96a5a 0%, #8a3024 100%)",
                  borderColor: "rgba(255,133,133,0.7)",
                  color: "#fff8f3",
                  boxShadow:
                    "0 4px 10px rgba(217,106,90,0.4), inset 0 1px 0 rgba(255,255,255,0.25)",
                }}
              >
                {t("match.surrender.confirm.confirm")}
              </ClassicButton>
            </div>
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );

  return createPortal(dialog, document.body);
}
