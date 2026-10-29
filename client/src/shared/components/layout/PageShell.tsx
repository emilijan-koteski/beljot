import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

const FRAME = "mx-auto w-full max-w-330 px-4 pt-4 pb-10 md:px-6 md:pt-5 md:pb-16 lg:px-7 lg:pt-6";
const WITH_SIDEBAR = "lg:grid lg:grid-cols-[minmax(0,1fr)_18.75rem] lg:items-start lg:gap-6";
const CLEAR_BOTTOM_BAR = "max-md:pb-28";
const MAIN = "flex min-w-0 flex-col gap-3.5 md:gap-4";
const SIDEBAR = "sticky top-21 hidden flex-col gap-4 lg:flex";
const BOTTOM_BAR =
  "from-background via-background/95 pointer-events-none fixed inset-x-0 bottom-0 z-30 flex flex-col gap-2 bg-linear-to-t to-transparent px-4 pt-8 pb-4 md:hidden *:pointer-events-auto";

interface PageShellProps {
  children: ReactNode;
  sidebar?: ReactNode;
  bottomBar?: ReactNode;
  testId?: string;
}

/**
 * The frame every app page sits in: a 1320px cap with 16, 24 and 28px gutters on a phone, tablet and desktop.
 * A desktop adds an optional 300px sidebar that stays under the top bar; a phone pins an optional bar to the bottom.
 */
export function PageShell({ children, sidebar, bottomBar, testId }: PageShellProps) {
  return (
    <>
      <div
        data-testid={testId}
        className={cn(
          FRAME,
          sidebar !== undefined && WITH_SIDEBAR,
          bottomBar !== undefined && CLEAR_BOTTOM_BAR,
        )}
      >
        <div className={MAIN}>{children}</div>
        {sidebar !== undefined && <aside className={SIDEBAR}>{sidebar}</aside>}
      </div>
      {bottomBar !== undefined && (
        <div className={BOTTOM_BAR} data-testid="page-bottom-bar">
          {bottomBar}
        </div>
      )}
    </>
  );
}
