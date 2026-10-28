import type { ReactNode } from "react";

/** The one "Loading…" line for lists, menus and dialogs, announced politely to assistive tech. */
export function LoadingLine({ children, testId }: { children: ReactNode; testId?: string }) {
  return (
    <p role="status" className="text-ink-mute m-0 px-1 py-2 text-xs" data-testid={testId}>
      {children}
    </p>
  );
}
