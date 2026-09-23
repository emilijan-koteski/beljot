import { Slider as SliderPrimitive } from "@base-ui/react/slider";

import { cn } from "@/shared/lib/utils";

type SliderProps = Omit<
  SliderPrimitive.Root.Props<number>,
  "className" | "children" | "aria-label"
> & {
  /** Accessible name. Forwarded to the thumb's range input, which is what a screen reader lands on. */
  "aria-label"?: string;
  /** Screen-reader text for a value (e.g. `30%`); defaults to the bare number. */
  getAriaValueText?: (value: number) => string;
  className?: string;
  trackClassName?: string;
  indicatorClassName?: string;
  thumbClassName?: string;
};

/**
 * Single-thumb slider on the Base UI primitive, which renders a real
 * `<input type="range">` inside the thumb for keyboard and screen-reader
 * support. `onValueChange` fires live while dragging; `onValueCommitted` fires
 * once on release (and once per keyboard step).
 *
 * The default look follows the app tokens; every part takes its own className
 * so a surface with its own palette (the brass in-match dialog) restyles the
 * track, fill and thumb without forking the component. Disabled state is
 * exposed as `data-disabled` on every part.
 *
 * The track is outlined with an inset shadow, not a border: the primitive
 * sizes the fill with `height: inherit`, so a border would push the fill out
 * past the bottom of the track.
 *
 * The control, which takes the presses, is 24 px tall — the minimum target
 * size — around the thin visual track.
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
