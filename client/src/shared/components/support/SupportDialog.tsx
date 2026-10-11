import { ChevronRight, Coffee } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { BMC_QR_SRC, BMC_URL, supportMarkSrc } from "@/shared/components/support/supportLinks";
import { buttonVariants } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { SuitRule } from "@/shared/components/ui/suit-rule";
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

  const markSrc = supportMarkSrc(prefersReducedMotion);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="gap-0 p-5 sm:max-w-md" data-testid="support-dialog">
        <div className="flex flex-col items-center gap-3 text-center">
          <img
            src={markSrc}
            alt=""
            aria-hidden="true"
            className="size-20"
            data-testid="support-mark"
          />
          <Eyebrow>{t("support.dialog.eyebrow")}</Eyebrow>
          <DialogTitle className="font-display text-ink max-w-70 text-[19px] leading-snug font-semibold tracking-[-0.3px]">
            {t("support.dialog.title")}
          </DialogTitle>
        </div>

        <DialogDescription className="text-ink-dim mt-3.5 text-center text-[13px] leading-relaxed">
          {t("support.dialog.body")}
        </DialogDescription>

        <div className="mt-5 mb-4 flex flex-col [&>div]:my-0">
          <SuitRule />
        </div>

        <a
          href={BMC_URL}
          target="_blank"
          rel="noopener noreferrer"
          className={cn(buttonVariants({ size: "cta" }), "w-full")}
          data-testid="support-cta"
        >
          <Coffee className="size-4.5" />
          {t("support.dialog.cta")}
        </a>

        <div className="mt-3 flex flex-col items-center">
          <button
            type="button"
            onClick={() => setShowQr((v) => !v)}
            aria-expanded={showQr}
            data-testid="support-qr-toggle"
            className="text-ink-mute hover:text-ink focus-visible:ring-accent/50 inline-flex cursor-pointer items-center gap-1 rounded text-[11.5px] transition-colors outline-none focus-visible:ring-2"
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
        </div>
      </DialogContent>
    </Dialog>
  );
}
