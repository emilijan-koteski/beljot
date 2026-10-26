import { cn } from "@/shared/lib/utils";

/** One mark on the track. `label` renders under it; `emphasis` draws it taller. */
type SliderTick = { value: number; label?: string; emphasis?: boolean };

/** A second, read-only mark — e.g. "your own score" against a threshold. */
type SliderMarker = { value: number; label?: string };

type DurationSliderProps = {
  value: number;
  onChange: (next: number) => void;
  min?: number;
  max?: number;
  step?: number;
  unitLabel: string;
  rangeLabel?: string;
  presets?: number[];
  ticks?: SliderTick[];
  fillStyle?: string;
  valueText?: string;
  marker?: SliderMarker;
  className?: string;
  testId?: string;
};

const DEFAULT_PRESETS = [15, 30, 60, 120];

/**
 * Discrete range slider with a native range input layered over a painted track.
 * Ticks, the marker and the thumb anchor on the track box, not the asymmetrically padded shell.
 */
export function DurationSlider({
  value,
  onChange,
  min = 10,
  max = 120,
  step = 5,
  unitLabel,
  rangeLabel,
  presets = DEFAULT_PRESETS,
  ticks,
  fillStyle,
  valueText,
  marker,
  className,
  testId,
}: DurationSliderProps) {
  const span = max - min || 1;
  const pct = ((value - min) / span) * 100;

  const resolvedTicks: SliderTick[] =
    ticks ??
    (() => {
      const out: SliderTick[] = [];
      for (let v = min; v <= max; v += 10) out.push({ value: v, emphasis: presets.includes(v) });
      return out;
    })();

  const hasTickLabels = resolvedTicks.some((tk) => tk.label !== undefined);
  const markerPct =
    marker === undefined ? null : ((Math.min(max, Math.max(min, marker.value)) - min) / span) * 100;

  return (
    <div className={cn("flex flex-col gap-2.5", className)}>
      <div className="flex items-baseline justify-between">
        <div className="flex items-baseline gap-1.5">
          <span
            className="font-display text-ink text-[22px] leading-none font-bold tracking-[-0.6px] tabular-nums"
            data-testid={testId ? `${testId}-value` : undefined}
            data-value={value}
          >
            {valueText ?? value}
          </span>
          <span className="text-brass-deep font-mono text-[11px] font-semibold tracking-[1px] uppercase">
            {unitLabel}
          </span>
        </div>
        {rangeLabel && (
          <span className="text-ink-mute font-mono text-[10.5px] tracking-[0.4px]">
            {rangeLabel}
          </span>
        )}
      </div>

      <div
        className={cn(
          "bg-surface-elevated border-border relative rounded-md border px-3 py-3.5",
          hasTickLabels && "pb-6",
        )}
      >
        <div className="relative">
          <div className="bg-surface-sunken border-border relative h-1.5 overflow-hidden rounded-full border">
            <div
              className="absolute inset-y-0 left-0"
              style={{
                width: `${pct}%`,
                background:
                  fillStyle ?? "linear-gradient(90deg, var(--brass) 0%, var(--accent) 100%)",
              }}
            />
          </div>
          <div className="pointer-events-none absolute inset-x-0 top-0 bottom-0">
            {resolvedTicks.map((tk) => {
              const tickPct = ((tk.value - min) / span) * 100;
              return (
                <span key={tk.value}>
                  <span
                    className={cn(
                      "absolute top-1/2 -translate-x-1/2 -translate-y-1/2 rounded-xs",
                      tk.emphasis ? "bg-brass-deep opacity-70" : "bg-ink-off opacity-45",
                    )}
                    style={{
                      left: `${tickPct}%`,
                      width: tk.emphasis ? 2 : 1,
                      height: tk.emphasis ? 12 : 7,
                    }}
                  />
                  {tk.label !== undefined && (
                    <span
                      className="text-ink-mute absolute -translate-x-1/2 font-mono text-[9.5px] tracking-[0.4px] tabular-nums"
                      style={{ left: `${tickPct}%`, top: "calc(50% + 10px)" }}
                    >
                      {tk.label}
                    </span>
                  )}
                </span>
              );
            })}

            {markerPct !== null && (
              <span
                data-testid={testId ? `${testId}-marker` : undefined}
                data-value={marker?.value}
              >
                <span
                  className="bg-ink absolute top-1/2 h-4 w-0.5 -translate-x-1/2 -translate-y-1/2 rounded-xs"
                  style={{ left: `${markerPct}%` }}
                />
                {marker?.label && (
                  <span
                    className="bg-surface-elevated border-border text-ink absolute -translate-x-1/2 rounded border px-1 py-px font-mono text-[9.5px] font-semibold whitespace-nowrap"
                    style={{ left: `${markerPct}%`, bottom: "calc(100% + 16px)" }}
                  >
                    {marker.label}
                  </span>
                )}
              </span>
            )}
          </div>
          <input
            type="range"
            min={min}
            max={max}
            step={step}
            value={value}
            onChange={(e) => onChange(Number(e.target.value))}
            aria-valuemin={min}
            aria-valuemax={max}
            aria-valuenow={value}
            aria-valuetext={valueText}
            data-testid={testId}
            className="absolute inset-x-0 top-1/2 h-9 w-full -translate-y-1/2 cursor-pointer appearance-none bg-transparent outline-none focus-visible:[&::-webkit-slider-thumb]:shadow-[0_0_0_4px_var(--accent-soft)] [&::-webkit-slider-runnable-track]:bg-transparent [&::-webkit-slider-thumb]:size-5 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:border-2 [&::-webkit-slider-thumb]:border-accent [&::-webkit-slider-thumb]:bg-surface [&::-webkit-slider-thumb]:shadow-[0_4px_12px_-4px_rgba(25,101,54,0.45),inset_0_0_0_3px_var(--accent)] [&::-moz-range-thumb]:size-5 [&::-moz-range-thumb]:appearance-none [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-2 [&::-moz-range-thumb]:border-accent [&::-moz-range-thumb]:bg-surface"
          />
        </div>
      </div>
    </div>
  );
}
