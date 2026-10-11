import { useEffect } from "react";

type Props = {
  message: string | null;
  onClear: () => void;
  ttlMs?: number;
};

/**
 * Accent-tinted pill at the bottom-center of the viewport. Used for the
 * lobby's ephemeral "Joining room…" / "Opened Create Room dialog" feedback.
 */
export function Toast({ message, onClear, ttlMs = 2200 }: Props) {
  useEffect(() => {
    if (!message) return;
    const handle = window.setTimeout(onClear, ttlMs);
    return () => window.clearTimeout(handle);
  }, [message, onClear, ttlMs]);

  if (!message) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      data-testid="lobby-toast"
      className="bg-surface-elevated text-ink fixed bottom-6 left-1/2 z-60 -translate-x-1/2 rounded-full border border-accent/30 px-4.5 py-2.5 text-sm font-medium shadow-[0_10px_30px_-10px_rgba(25,101,54,0.40)] animate-[card-in_.25s_ease_both]"
    >
      {message}
    </div>
  );
}
