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
          "border-border text-ink-mute hover:border-brass hover:text-brass-deep data-popup-open:border-brass data-popup-open:bg-brass-soft data-popup-open:text-brass-deep inline-flex size-4 items-center justify-center rounded-full border align-middle text-[10px] leading-none font-bold transition-colors",
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
            className="border-border-2 bg-surface-elevated text-ink-dim w-67 max-w-[calc(100vw-32px)] rounded-[10px] border p-2.5 text-[12.5px] leading-normal font-normal normal-case shadow-[0_18px_40px_-20px_rgba(14,58,36,0.45)]"
          >
            {text}
          </PopoverPrimitive.Popup>
        </PopoverPrimitive.Positioner>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  );
}
