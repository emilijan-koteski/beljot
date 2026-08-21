import { useState } from "react";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/components/ui/tooltip";
import { Z } from "@/shared/lib/zLayers";

type Props = {
  /** Authored prose: what the OTHER variant does. */
  note: string;
  /** Accessible name of the marker itself, from `ui.diffLabel`. */
  label: string;
  /** `dark` is the in-match felt overlay; `light` is the standalone page. */
  tone?: "light" | "dark";
};

const LIGHT = {
  color: "var(--brass-deep)",
  background: "var(--brass-soft)",
  border: "1px solid var(--border-2)",
};

const DARK = {
  color: "#c9a876",
  background: "rgba(201,168,118,0.14)",
  border: "1px solid rgba(201,168,118,0.42)",
};

/**
 * The per-block "this rule differs" marker: an inline button whose tooltip says
 * what the OTHER variant does.
 *
 * The tooltip is CONTROLLED, because Base UI's own trigger interaction is
 * hover- and focus-only (its hover is deliberately `mouseOnly`). On a phone
 * there is no hover, so an uncontrolled tooltip here would pass every unit test
 * and still be unreachable for the players most likely to be reading the rules
 * mid-match on a handset. Open state therefore comes from two inputs:
 *
 *   - hover / keyboard focus, which the root reports through `onOpenChange`;
 *   - a press, which PINS the tooltip open until it is pressed again, tapped
 *     away from, or dismissed with Escape.
 *
 * `closeOnClick` is off for the same reason: Base UI's reflex is to close a
 * tooltip when its trigger is pressed, and here a press is what opens it.
 *
 * Expects a `TooltipProvider` above it — one per surface, not one per marker, so
 * that the dozen markers on a page share Base UI's delay grouping instead of
 * each running its own provider root.
 */
export function DiffMarker({ note, label, tone = "light" }: Props) {
  const [hoverOpen, setHoverOpen] = useState(false);
  const [pinned, setPinned] = useState(false);
  const open = hoverOpen || pinned;
  const skin = tone === "dark" ? DARK : LIGHT;

  return (
    <>
      <Tooltip
        open={open}
        onOpenChange={(next) => {
          setHoverOpen(next);
          // Escape, an outside press, and the pointer leaving all unpin it too —
          // otherwise a pinned marker could only ever be closed by finding it
          // again.
          if (!next) setPinned(false);
        }}
      >
        <TooltipTrigger
          closeOnClick={false}
          type="button"
          aria-label={label}
          data-testid="rules-diff-marker"
          onClick={() => {
            // A tap focuses the button first, and Base UI opens on focus — so
            // the press has to clear that too, or the second tap would toggle
            // the pin off and the focus-open would keep the tooltip on screen.
            setHoverOpen(false);
            setPinned((p) => !p);
          }}
          style={{
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
            width: 18,
            height: 18,
            marginLeft: 6,
            // A circle, not a rounded square: with the bare "!" glyph below it
            // reads as the familiar (!) "there is something to know here" mark.
            borderRadius: "50%",
            cursor: "pointer",
            verticalAlign: "middle",
            flexShrink: 0,
            ...skin,
          }}
        >
          <span
            aria-hidden="true"
            style={{
              fontFamily: "var(--font-body)",
              fontSize: 12,
              fontWeight: 700,
              // lineHeight 1 keeps the glyph optically centred in the circle —
              // an inherited body line-height pushes it visibly low.
              lineHeight: 1,
            }}
          >
            !
          </span>
        </TooltipTrigger>
        {/* Base UI portals the popup to document.body, so inside the in-match
            overlay it is a SIBLING of a panel painting at Z.UTIL — at the
            primitive's default z-50 the note would be completely hidden behind
            it. The light page has no such competitor and keeps the default. */}
        <TooltipContent zIndex={tone === "dark" ? Z.UTIL_POPOVER : undefined}>
          {note}
        </TooltipContent>
      </Tooltip>
    </>
  );
}
