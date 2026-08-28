import { useState } from "react";
import { Trans } from "react-i18next";

import { SupportDialog } from "@/shared/components/support/SupportDialog";
import { BMC_MARK_SRC } from "@/shared/components/support/supportLinks";
import { cn } from "@/shared/lib/utils";

/**
 * One quiet line of text whose only interactive part opens <SupportDialog />.
 *
 * The deliberate shape of the whole feature: a sentence the eye can skip, never
 * a banner, never dismissible — there is nothing to dismiss. Owns its own
 * dialog state so mounting it stays a one-line change at each call site.
 *
 * `auth` sits under the credit line in AuthLayout's footer (so it covers login,
 * register, forgot-password and reset-password at once); `lobby` sits under the
 * room-count footnote at the very bottom of the lobby. Both render the SAME
 * `support.note` string — `variant` only picks the type scale and spacing, so
 * the two placements cannot drift apart in wording (they once did).
 */

type SupportNoteProps = {
  variant: "auth" | "lobby";
  className?: string;
};

const VARIANT_CLS: Record<SupportNoteProps["variant"], string> = {
  auth: "mt-1.5 text-[11.5px]",
  lobby: "mt-2 text-xs",
};

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
            // Named <action> rather than <link>: Trans parses the string as
            // HTML, and `link` is a real VOID element there — it would close
            // immediately and spill the label into the surrounding sentence.
            //
            // A button, not an anchor: it opens the in-app explainer rather
            // than leaving for the external page. The dotted underline reads as
            // "this tells you more" instead of "this navigates".
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
        {/* Closes the sentence with the cup. Sized in `em` so it tracks the
            note's font size at both variants, and 1.35em rather than 1em
            because a disc reads smaller than a glyph in the same box. Static,
            never the animated sticker: a looping GIF in a footer line is the
            nagging this feature exists to avoid. */}
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
