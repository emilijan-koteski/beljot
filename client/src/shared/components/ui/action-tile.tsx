import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

export type ActionTileTone = "primary" | "quiet";

const BASE =
  "flex h-14 min-w-0 items-center gap-2.5 rounded-xl px-3 text-left transition-[background-color,border-color,box-shadow] outline-none focus-visible:ring-3 focus-visible:ring-ring/70 [&>svg]:size-5 [&>svg]:shrink-0";

const TONE: Record<ActionTileTone, string> = {
  primary: "bg-accent text-accent-ink shadow-glow hover:bg-accent-deep",
  quiet: "bg-surface border-border text-ink border hover:border-brass",
};

/** The classes of an action tile, for a tile that is not a plain button (a label wrapping a field). */
export function actionTileClasses(tone: ActionTileTone = "quiet"): string {
  return cn(BASE, TONE[tone]);
}

type ActionTileProps = Omit<ComponentProps<"button">, "title"> & {
  tone?: ActionTileTone;
  icon: ReactNode;
  title: ReactNode;
  subtitle: ReactNode;
};

/**
 * A 56px page action: an icon, a title and a one-line subtitle. `primary` is the page's one accent action;
 * the icon of a `quiet` tile takes its colour from the caller.
 */
export function ActionTile({
  tone = "quiet",
  icon,
  title,
  subtitle,
  className,
  ...props
}: ActionTileProps) {
  return (
    <button
      type="button"
      className={cn(
        actionTileClasses(tone),
        "cursor-pointer disabled:cursor-default disabled:opacity-60",
        className,
      )}
      {...props}
    >
      {icon}
      <span className="flex min-w-0 flex-1 flex-col leading-tight">
        <span className="truncate text-sm font-semibold">{title}</span>
        <span
          className={cn("truncate text-2xs", tone === "primary" ? "opacity-80" : "text-ink-mute")}
        >
          {subtitle}
        </span>
      </span>
    </button>
  );
}
