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

/**
 * The one place that explains WHY support is being asked for. Every support
 * surface in the app is a link or a menu row that opens this — nothing else
 * carries the pitch, so nothing else has to be dismissible.
 *
 * Shaped as a parchment card rather than a donation modal: mono eyebrow, then
 * the claim, then the reason, then the app's own ♥♠♦♣ rule dividing the
 * situation from the action — the same divider AuthCard and Create Room use, so
 * this reads as a Beljot surface instead of a bolted-on payment widget. The
 * yellow stays inside the cup artwork and never touches the UI.
 *
 * Controlled only (no DialogTrigger): one of its callers is a
 * DropdownMenuItem, and the dropdown unmounts its own subtree on select, which
 * would close the dialog in the same tick it opened.
 */

type SupportDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function SupportDialog({ open, onOpenChange }: SupportDialogProps) {
  const { t } = useTranslation();
  const prefersReducedMotion = useReducedMotion();
  const [showQr, setShowQr] = useState(false);

  // The animated sticker is only used when motion is welcome; the still frame
  // is the fallback (see supportLinks).
  const markSrc = supportMarkSrc(prefersReducedMotion);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="gap-0 p-5 sm:max-w-md" data-testid="support-dialog">
        {/* Cup, eyebrow and claim read as one block, so they sit closer to each
            other than to the paragraph that follows. */}
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

        {/* Divides the situation above from the action below. SuitRule carries
            its own my-* for the auth card, so zero it and let this wrapper own
            both gaps — flex, because a plain div lets those margins collapse
            straight through it (they did, and ate the gap under the rule). */}
        <div className="mt-5 mb-4 flex flex-col [&>div]:my-0">
          <SuitRule />
        </div>

        {/* Styled as a Button but rendered as a plain anchor: it leaves the app
            for an external domain, so it must be a real link (middle-click,
            open-in-new-tab, "copy link address" all keep working). */}
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

        {/* Collapsed by default: the QR only helps someone reading this on a
            desktop with a phone in hand, and expanded it would make the dialog
            taller for everyone else. */}
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
            /* White plate under the code: the QR's own quiet zone is white, and
               parchment behind it would eat the contrast scanners need. */
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
