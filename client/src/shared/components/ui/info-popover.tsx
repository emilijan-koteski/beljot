"use client";

import { Popover as PopoverPrimitive } from "@base-ui/react/popover";
import * as React from "react";

import { cn } from "@/shared/lib/utils";

type InfoPopoverProps = {
  text: React.ReactNode;
  ariaLabel: string;
  testId?: string;
  className?: string;
};

/**
 * "(?)" button that opens an explanation popup on hover, or on tap for touch devices.
 * Portalled via Base UI's Positioner so overflow-clipping ancestors and viewport edges never cut it off.
 */
export function InfoPopover({ text, ariaLabel, testId, className }: InfoPopoverProps) {
  return (
    <PopoverPrimitive.Root>
      <PopoverPrimitive.Trigger
        aria-label={ariaLabel}
        data-testid={testId}
        openOnHover
        delay={0}
        closeDelay={0}
        className={cn(
          "border-border text-ink-mute hover:border-brass hover:text-brass-deep data-popup-open:border-brass data-popup-open:bg-brass-soft data-popup-open:text-brass-deep inline-flex size-4 items-center justify-center rounded-full border align-middle text-2xs leading-none font-bold transition-[color,background-color,border-color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/70",
          className,
        )}
      >
        ?
      </PopoverPrimitive.Trigger>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Positioner
          side="bottom"
          align="start"
          sideOffset={8}
          collisionPadding={16}
          className="z-50"
        >
          <PopoverPrimitive.Popup
            data-testid={testId ? `${testId}-content` : undefined}
            className="border-brass bg-surface-elevated text-ink-dim w-66 max-w-[calc(100vw-32px)] rounded-md border p-2.5 text-xs leading-normal font-normal normal-case shadow-lg"
          >
            {text}
          </PopoverPrimitive.Popup>
        </PopoverPrimitive.Positioner>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  );
}
