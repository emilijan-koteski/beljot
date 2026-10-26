import { useState } from "react";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/components/ui/tooltip";
import { Z } from "@/shared/lib/zLayers";

type Props = {
  note: string;
  label: string;
  tone?: "light" | "dark";
};

const LIGHT = {
  color: "var(--brass-deep)",
  background: "var(--brass-soft)",
  border: "1px solid var(--brass)",
};

const DARK = {
  color: "var(--brass)",
  background: "var(--brass-10)",
  border: "1px solid var(--brass-40)",
};

/**
 * Inline marker whose tooltip says what the other variant does, pinned open on press so touch screens can reach it.
 * Expects a shared `TooltipProvider` above it.
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
          if (!next) setPinned(false);
        }}
      >
        <TooltipTrigger
          closeOnClick={false}
          type="button"
          aria-label={label}
          data-testid="rules-diff-marker"
          onClick={() => {
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
              fontSize: 12,
              fontWeight: 700,
              lineHeight: 1,
            }}
          >
            !
          </span>
        </TooltipTrigger>
        <TooltipContent zIndex={tone === "dark" ? Z.UTIL_POPOVER : undefined}>
          {note}
        </TooltipContent>
      </Tooltip>
    </>
  );
}
