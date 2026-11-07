import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

type BrowserFrameProps = {
  url: string;
  tone?: "felt" | "page";
  className?: string;
  children: ReactNode;
};

/** A browser window around a real app screenshot: dark on the felt, theme-coloured on the page. */
export function BrowserFrame({ url, tone = "page", className, children }: BrowserFrameProps) {
  const felt = tone === "felt";
  return (
    <div
      className={cn(
        "overflow-hidden rounded-[14px] ring-1",
        felt ? "bg-background shadow-felt-xl ring-brass/30" : "bg-surface shadow-xl ring-border",
        className,
      )}
    >
      <div
        className={cn(
          "flex h-7.5 items-center gap-1.5 px-3",
          felt ? "bg-surface" : "bg-surface-sunken",
        )}
        aria-hidden="true"
      >
        <span className="bg-brass/40 size-2.25 rounded-full" />
        <span className="bg-brass/40 size-2.25 rounded-full" />
        <span className="bg-brass/40 size-2.25 rounded-full" />
        <span className="text-ink-mute bg-background/60 mx-auto -translate-x-4.5 rounded-md px-3.5 py-1 font-mono text-[11px] leading-none">
          {url}
        </span>
      </div>
      {children}
    </div>
  );
}
