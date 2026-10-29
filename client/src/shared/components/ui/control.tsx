import { cva } from "class-variance-authority";

export const controlVariants = cva(
  "border-border bg-surface text-ink-dim hover:bg-surface-sunken hover:text-ink data-popup-open:bg-surface-sunken aria-expanded:bg-surface-sunken inline-flex shrink-0 cursor-pointer items-center justify-center rounded-full border transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/70 [&_svg]:size-3.5 [&_svg]:shrink-0",
  {
    variants: {
      shape: {
        round: "size-8",
        labelled: "h-8 gap-1.5 px-3 text-xs font-medium",
      },
    },
    defaultVariants: { shape: "round" },
  },
);
