/** The table's waiting state before the first deal: a brass ring spinner over one status line. */
export function MatchStartSplash({ message }: { message: string }) {
  return (
    <div
      role="status"
      className="flex flex-col items-center gap-4"
      data-testid="match-starting-splash"
    >
      <span
        aria-hidden
        className="inline-block size-9 rounded-full motion-safe:animate-spin"
        style={{
          border: "2px solid var(--brass-soft)",
          borderTopColor: "var(--brass)",
          animationDuration: "0.9s",
        }}
      />
      <span
        className="text-base"
        style={{ color: "var(--ink-light)", opacity: 0.85 }}
        data-testid="splash-text"
      >
        {message}
      </span>
    </div>
  );
}
