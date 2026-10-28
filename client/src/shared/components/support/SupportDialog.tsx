import { ChevronRight, Coffee } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { BMC_QR_SRC, BMC_URL, supportMarkSrc } from "@/shared/components/support/supportLinks";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { cn } from "@/shared/lib/utils";

type SupportDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/** Controlled dialog explaining why to support the app, with the Buy Me a Coffee link and an optional QR. */
export function SupportDialog({ open, onOpenChange }: SupportDialogProps) {
  const { t } = useTranslation();
  const prefersReducedMotion = useReducedMotion();
  const [showQr, setShowQr] = useState(false);

  return (
    <AppDialog
      open={open}
      onClose={() => onOpenChange(false)}
      tone="celebration"
      art={
        <img
          src={supportMarkSrc(prefersReducedMotion)}
          alt=""
          aria-hidden="true"
          className="size-20"
          data-testid="support-mark"
        />
      }
      title={t("support.dialog.title")}
      description={t("support.dialog.body")}
      primary={{
        label: (
          <>
            <Coffee className="size-5" />
            {t("support.dialog.cta")}
          </>
        ),
        href: BMC_URL,
        testId: "support-cta",
      }}
      testId="support-dialog"
    >
      <button
        type="button"
        onClick={() => setShowQr((v) => !v)}
        aria-expanded={showQr}
        data-testid="support-qr-toggle"
        className="text-ink-mute hover:text-ink focus-visible:ring-ring/70 inline-flex cursor-pointer items-center gap-1 rounded text-xs transition-colors outline-none focus-visible:ring-3"
      >
        <ChevronRight
          className={cn("size-3.5 transition-transform", showQr && "rotate-90")}
          aria-hidden="true"
        />
        {t("support.dialog.qrToggle")}
      </button>

      {showQr && (
        <img
          src={BMC_QR_SRC}
          alt={t("support.dialog.qrAlt")}
          width={168}
          height={168}
          className="ring-border mt-3 size-42 rounded-xl bg-white p-2.5 ring-1"
          data-testid="support-qr"
        />
      )}
    </AppDialog>
  );
}
