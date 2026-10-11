import { Slider as SliderPrimitive } from "@base-ui/react/slider";

import { cn } from "@/shared/lib/utils";

type SliderProps = Omit<
  SliderPrimitive.Root.Props<number>,
  "className" | "children" | "aria-label"
> & {
  "aria-label"?: string;
  getAriaValueText?: (value: number) => string;
  className?: string;
  trackClassName?: string;
  indicatorClassName?: string;
  thumbClassName?: string;
};

/**
 * Single-thumb Base UI slider whose track, fill and thumb each accept their own className.
 * The track uses an inset shadow, not a border, because a border would push the inherited-height fill out of it.
 */
function Slider({
  "aria-label": ariaLabel,
  getAriaValueText,
  className,
  trackClassName,
  indicatorClassName,
  thumbClassName,
  ...props
}: SliderProps) {
  return (
    <SliderPrimitive.Root
      data-slot="slider"
      className={cn("relative w-full data-disabled:opacity-50", className)}
      {...props}
    >
      <SliderPrimitive.Control
        data-slot="slider-control"
        className="flex h-6 w-full cursor-pointer touch-none items-center select-none data-disabled:cursor-not-allowed"
      >
        <SliderPrimitive.Track
          data-slot="slider-track"
          className={cn(
            "bg-surface-sunken h-1.5 w-full rounded-full shadow-[inset_0_0_0_1px_var(--border)] select-none",
            trackClassName,
          )}
        >
          <SliderPrimitive.Indicator
            data-slot="slider-indicator"
            className={cn("bg-accent rounded-full select-none", indicatorClassName)}
          />
          <SliderPrimitive.Thumb
            data-slot="slider-thumb"
            aria-label={ariaLabel}
            getAriaValueText={
              getAriaValueText ? (_formatted, value) => getAriaValueText(value) : undefined
            }
            className={cn(
              "border-accent bg-surface size-4 rounded-full border-2 shadow-sm outline-none select-none",
              "has-[:focus-visible]:ring-ring/50 has-[:focus-visible]:ring-[3px]",
              thumbClassName,
            )}
          />
        </SliderPrimitive.Track>
      </SliderPrimitive.Control>
    </SliderPrimitive.Root>
  );
}

export { Slider };
