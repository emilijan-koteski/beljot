import { Flag } from "lucide-react";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";

import { HUDButton } from "./HUDButton";
import { SurrenderDialog } from "./SurrenderDialog";

interface SurrenderButtonProps {
  canRequest: boolean;
  isExhausted: boolean;
  isPending: boolean;
  onConfirm: () => void;
}

export function SurrenderButton({
  canRequest,
  isExhausted,
  isPending,
  onConfirm,
}: SurrenderButtonProps) {
  const { t } = useTranslation();
  const [confirmOpen, setConfirmOpen] = useState(false);

  useEffect(() => {
    if (isPending || isExhausted) setConfirmOpen(false);
  }, [isPending, isExhausted]);

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
 * Surrender confirmation modal, exported for surfaces that cannot keep SurrenderButton mounted.
 */
export function SurrenderConfirmDialog({ onCancel, onConfirm }: SurrenderConfirmDialogProps) {
  const { t } = useTranslation();

  return createPortal(
    <SurrenderDialog
      titleId="surrender-confirm-title"
      title={t("match.surrender.confirm.title")}
      body={t("match.surrender.confirm.body")}
      cost={t("match.surrender.confirm.consumeWarning")}
      confirmLabel={t("match.surrender.confirm.confirm")}
      cancelLabel={t("match.surrender.confirm.cancel")}
      onConfirm={onConfirm}
      onCancel={onCancel}
      testIds={{
        overlay: "surrender-confirm-overlay",
        confirm: "surrender-confirm",
        cancel: "surrender-cancel",
      }}
    />,
    document.body,
  );
}
