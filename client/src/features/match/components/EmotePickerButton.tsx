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
import { Z } from "@/shared/lib/zLayers";
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
  label?: ReactNode;
  openDirection?: "up" | "down";
  className?: string;
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
  const lastSentAt = useMatchStore((s) => s.lastEmoteSentAt);
  const setLastSentAt = useMatchStore((s) => s.setLastEmoteSentAt);
  const [now, setNow] = useState<number>(() => performance.now());
  const popoverRef = useRef<HTMLDivElement | null>(null);
  const buttonRef = useRef<HTMLButtonElement | null>(null);
  const tileRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const cooldownRemaining = lastSentAt === 0 ? 0 : Math.max(0, COOLDOWN_MS - (now - lastSentAt));

  useEffect(() => {
    if (cooldownRemaining <= 0) return;
    const handle = window.setTimeout(() => {
      setNow(performance.now());
    }, cooldownRemaining);
    return () => window.clearTimeout(handle);
  }, [lastSentAt, cooldownRemaining]);

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

  useEffect(() => {
    if (isOpen) {
      tileRefs.current[0]?.focus();
    }
  }, [isOpen]);

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
      if (remaining > 0) return;
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
        className={`inline-flex h-9 cursor-pointer items-center rounded-md transition-colors hover:not-disabled:brightness-110 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brass disabled:cursor-not-allowed disabled:opacity-40 ${label ? "w-full justify-start gap-2 px-3.5" : "w-9 justify-center"} ${className}`}
        style={{
          background: "var(--panel-deeper)",
          border: "1px solid var(--brass-40)",
          color: "var(--ink-light)",
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
          className={`absolute right-0 w-44 rounded-lg p-2 shadow-xl ${openDirection === "down" ? "top-full mt-1" : "bottom-12"}`}
          style={{
            zIndex: Z.UTIL,
            background: "var(--panel-deeper)",
            border: "1px solid var(--brass-40)",
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
