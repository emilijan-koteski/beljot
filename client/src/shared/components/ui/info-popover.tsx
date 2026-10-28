"use client";

import { Popover as PopoverPrimitive } from "@base-ui/react/popover";
import * as React from "react";

import { POPUP_SURFACE } from "@/shared/lib/popupSurface";
import { cn } from "@/shared/lib/utils";

type InfoPopoverProps = {
  text: React.ReactNode;
  ariaLabel: string;
  variant?: "help" | "diff";
  tone?: "light" | "dark";
  zIndex?: number;
  testId?: string;
  className?: string;
};

const TRIGGER = {
  help: {
    light:
      "border-border text-ink-mute hover:border-brass hover:text-brass-deep data-popup-open:border-brass data-popup-open:bg-brass-soft data-popup-open:text-brass-deep",
    dark: "border-brass/40 text-brass hover:bg-brass/10 data-popup-open:bg-brass/10",
  },
  diff: {
    light: "border-brass bg-brass-soft text-brass-deep",
    dark: "border-brass/40 bg-brass/10 text-brass",
  },
} as const;

const POPUP = {
  light: cn(POPUP_SURFACE, "text-ink-dim"),
  dark: "bg-ink text-background border-ink rounded-lg border shadow-lg",
} as const;

/**
 * The one inline explainer: a round "?" (help) or "!" (differs between variants) that opens a short note on
 * hover or tap. Portalled through Base UI's Positioner so clipping ancestors and viewport edges never cut it off.
 */
export function InfoPopover({
  text,
  ariaLabel,
  variant = "help",
  tone = "light",
  zIndex,
  testId,
  className,
}: InfoPopoverProps) {
  return (
    <PopoverPrimitive.Root>
      <PopoverPrimitive.Trigger
        aria-label={ariaLabel}
        data-testid={testId}
        openOnHover
        delay={0}
        closeDelay={0}
        className={cn(
          "inline-flex size-4 shrink-0 cursor-pointer items-center justify-center rounded-full border align-middle text-2xs leading-none font-bold transition-[color,background-color,border-color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/70",
          TRIGGER[variant][tone],
          className,
        )}
      >
        {variant === "diff" ? "!" : "?"}
      </PopoverPrimitive.Trigger>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Positioner
          side="bottom"
          align="start"
          sideOffset={8}
          collisionPadding={16}
          data-slot="info-popover-positioner"
          className="z-50"
          style={zIndex === undefined ? undefined : { zIndex }}
        >
          <PopoverPrimitive.Popup
            data-testid={testId ? `${testId}-content` : undefined}
            className={cn(
              POPUP[tone],
              "w-66 max-w-[calc(100vw-32px)] p-2.5 text-xs leading-normal font-normal normal-case",
            )}
          >
            {text}
          </PopoverPrimitive.Popup>
        </PopoverPrimitive.Positioner>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  );
}
