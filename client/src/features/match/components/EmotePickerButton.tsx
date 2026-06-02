import { Smile } from "lucide-react";
import {
  type KeyboardEvent as ReactKeyboardEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { useTranslation } from "react-i18next";

import { MOTION } from "@/shared/lib/motion";
import { useMatchStore } from "@/shared/stores/matchStore";
import { EMOTE_IDS, type EmoteID } from "@/shared/types/wsEvents";

const EMOTE_GLYPHS: Record<EmoteID, string> = {
  thumbs_up: "👍",
  clap: "👏",
  laugh: "😂",
  thinking: "🤔",
  facepalm: "🤦",
  heart: "❤️",
};

// snake_case wire ID → camelCase i18n leaf for `match.emote.names.*`.
const EMOTE_I18N_KEYS: Record<EmoteID, string> = {
  thumbs_up: "thumbsUp",
  clap: "clap",
  laugh: "laugh",
  thinking: "thinking",
  facepalm: "facepalm",
  heart: "heart",
};

const COOLDOWN_MS = MOTION.EMOTE_COOLDOWN;

interface EmotePickerButtonProps {
  onSend: (emote: EmoteID) => void;
  disabled?: boolean;
  /** When set, the trigger renders as a labeled row (mobile HUD menu). */
  label?: ReactNode;
  /** Picker opens upward by default (bottom HUD); "down" opens below the
   *  trigger (top-right menu). */
  openDirection?: "up" | "down";
  /** Extra classes for the trigger button (e.g. full-width row). */
  className?: string;
  /** Fired after an emote is sent — lets the host (mobile menu) close itself. */
  onSent?: () => void;
}

export function EmotePickerButton({
  onSend,
  disabled = false,
  label,
  openDirection = "up",
  className = "",
  onSent,
}: EmotePickerButtonProps) {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);
  // lastSentAt lives on matchStore so the cooldown survives the picker's
  // unmount/remount across phase transitions (D107). performance.now() is
  // monotonic so an OS clock backstep cannot lock the picker (D108).
  const lastSentAt = useMatchStore((s) => s.lastEmoteSentAt);
  const setLastSentAt = useMatchStore((s) => s.setLastEmoteSentAt);
  // `now` re-renders the picker exactly when the cooldown expires so disabled
  // tiles re-enable instantly without forcing parents to re-render every tick.
  const [now, setNow] = useState<number>(() => performance.now());
  const popoverRef = useRef<HTMLDivElement | null>(null);
  const buttonRef = useRef<HTMLButtonElement | null>(null);
  const tileRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const cooldownRemaining = lastSentAt === 0 ? 0 : Math.max(0, COOLDOWN_MS - (now - lastSentAt));

  // Single timer keyed on lastSentAt — fires once at the cooldown end so the
  // disabled tiles flip back to enabled. No per-tile timer storms.
  useEffect(() => {
    if (cooldownRemaining <= 0) return;
    const handle = window.setTimeout(() => {
      setNow(performance.now());
    }, cooldownRemaining);
    return () => window.clearTimeout(handle);
  }, [lastSentAt, cooldownRemaining]);

  // Close on outside click + Escape. Mirrors the lightweight popover pattern;
  // useFocusTrap is intentionally NOT used — outside click dismissal would be
  // swallowed by a focus trap (see Dev Notes anti-patterns).
  useEffect(() => {
    if (!isOpen) return;

    const handlePointer = (event: MouseEvent | TouchEvent) => {
      const target = event.target as Node | null;
      if (!target) return;
      if (popoverRef.current?.contains(target)) return;
      if (buttonRef.current?.contains(target)) return;
      setIsOpen(false);
    };
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
        // Restore focus to the toggle so keyboard users land back on it.
        buttonRef.current?.focus();
      }
    };

    document.addEventListener("mousedown", handlePointer);
    document.addEventListener("touchstart", handlePointer);
    document.addEventListener("keydown", handleKey);
    return () => {
      document.removeEventListener("mousedown", handlePointer);
      document.removeEventListener("touchstart", handlePointer);
      document.removeEventListener("keydown", handleKey);
    };
  }, [isOpen]);

  const handleToggle = useCallback(() => {
    setIsOpen((open) => !open);
  }, []);

  // When the picker opens, focus the first tile so keyboard users can navigate
  // the grid with arrow keys immediately (AC #1). Mouse users are unaffected
  // because the focus-visible ring only renders for keyboard activation.
  useEffect(() => {
    if (isOpen) {
      tileRefs.current[0]?.focus();
    }
  }, [isOpen]);

  // Roving-tabindex-style arrow navigation across the 3x2 tile grid.
  // Layout (zero-indexed):  0 1 2
  //                         3 4 5
  // Right/Left wrap horizontally; Up/Down move between rows (and wrap).
  const handleGridKeyDown = useCallback((event: ReactKeyboardEvent<HTMLDivElement>) => {
    const focused = document.activeElement;
    const currentIdx = tileRefs.current.findIndex((ref) => ref === focused);
    if (currentIdx === -1) return;
    let nextIdx: number | null = null;
    switch (event.key) {
      case "ArrowRight":
        nextIdx = (currentIdx + 1) % EMOTE_IDS.length;
        break;
      case "ArrowLeft":
        nextIdx = (currentIdx + EMOTE_IDS.length - 1) % EMOTE_IDS.length;
        break;
      case "ArrowDown":
      case "ArrowUp":
        nextIdx = (currentIdx + 3) % EMOTE_IDS.length;
        break;
    }
    if (nextIdx !== null) {
      event.preventDefault();
      tileRefs.current[nextIdx]?.focus();
    }
  }, []);

  const handleTileClick = useCallback(
    (emote: EmoteID) => {
      const stamp = performance.now();
      const remaining = lastSentAt === 0 ? 0 : COOLDOWN_MS - (stamp - lastSentAt);
      if (remaining > 0) return; // defence in depth — tiles already disabled
      setLastSentAt(stamp);
      setNow(stamp);
      onSend(emote);
      setIsOpen(false);
      onSent?.();
    },
    [lastSentAt, setLastSentAt, onSend, onSent],
  );

  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        onClick={handleToggle}
        disabled={disabled}
        aria-label={t("match.emote.button")}
        aria-pressed={isOpen}
        className={`inline-flex h-9 cursor-pointer items-center rounded-[10px] transition-colors hover:not-disabled:brightness-110 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brass disabled:cursor-not-allowed disabled:opacity-40 ${label ? "w-full justify-start gap-2 px-3.5" : "w-9 justify-center"} ${className}`}
        style={{
          background: "var(--panel-hud, rgba(18,32,22,0.85))",
          border: "1px solid rgba(201,168,118,0.4)",
          color: "var(--ink-light, #f5f2e8)",
          backdropFilter: "blur(6px)",
          WebkitBackdropFilter: "blur(6px)",
        }}
        data-testid="emote-toggle"
      >
        <Smile className="h-5 w-5 shrink-0" aria-hidden="true" />
        {label && <span className="font-body text-xs font-semibold">{label}</span>}
      </button>

      {isOpen && (
        <div
          ref={popoverRef}
          role="group"
          aria-label={t("match.emote.picker.title")}
          className={`absolute right-0 z-30 w-44 rounded-lg p-2 shadow-xl ${openDirection === "down" ? "top-full mt-1" : "bottom-12"}`}
          style={{
            background: "var(--panel-deeper, rgba(18,32,22,0.94))",
            border: "1px solid rgba(201,168,118,0.4)",
            backdropFilter: "blur(10px)",
            WebkitBackdropFilter: "blur(10px)",
          }}
          data-testid="emote-picker"
        >
          <div className="grid grid-cols-3 gap-1" onKeyDown={handleGridKeyDown}>
            {EMOTE_IDS.map((id, idx) => {
              const tileDisabled = cooldownRemaining > 0;
              return (
                <button
                  key={id}
                  ref={(el) => {
                    tileRefs.current[idx] = el;
                  }}
                  type="button"
                  onClick={() => handleTileClick(id)}
                  disabled={tileDisabled}
                  aria-disabled={tileDisabled}
                  aria-label={t(`match.emote.names.${EMOTE_I18N_KEYS[id]}`)}
                  data-testid={`emote-tile-${id}`}
                  className="aspect-square cursor-pointer rounded-md text-2xl leading-none transition-colors hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-40"
                >
                  <span aria-hidden="true">{EMOTE_GLYPHS[id]}</span>
                </button>
              );
            })}
          </div>
        </div>
      )}
    </>
  );
}
