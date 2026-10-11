import { useState } from "react";
import { Trans } from "react-i18next";

import { SupportDialog } from "@/shared/components/support/SupportDialog";
import { BMC_MARK_SRC } from "@/shared/components/support/supportLinks";
import { cn } from "@/shared/lib/utils";

type SupportNoteProps = {
  variant: "auth" | "lobby";
  className?: string;
};

const VARIANT_CLS: Record<SupportNoteProps["variant"], string> = {
  auth: "mt-1.5 text-[11.5px]",
  lobby: "mt-2 text-xs",
};

/** One quiet line of text whose only interactive part opens the support dialog. */
export function SupportNote({ variant, className }: SupportNoteProps) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <p
        className={cn("text-ink-mute text-center", VARIANT_CLS[variant], className)}
        data-testid="support-note"
      >
        <Trans
          i18nKey="support.note"
          components={{
            action: (
              <button
                type="button"
                onClick={() => setOpen(true)}
                data-testid="support-note-link"
                className="text-ink-dim hover:text-accent focus-visible:ring-accent/50 cursor-pointer rounded underline decoration-dotted underline-offset-2 transition-colors outline-none focus-visible:ring-2"
              />
            ),
          }}
        />{" "}
        <img
          src={BMC_MARK_SRC}
          alt=""
          aria-hidden="true"
          className="inline-block h-[1.35em] w-[1.35em] align-[-0.3em]"
          data-testid="support-note-icon"
        />
      </p>

      <SupportDialog open={open} onOpenChange={setOpen} />
    </>
  );
}
