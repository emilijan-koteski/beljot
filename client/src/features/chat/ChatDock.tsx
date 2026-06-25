import { MessageSquare, Send, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/shared/lib/utils";
import { useWsConnectionState, useWsSendMessage } from "@/shared/providers/WebSocketContext";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import type { ChatMessagePayload, ChatMessageRequest } from "@/shared/types/wsEvents";
import { ACTION_CHAT_MESSAGE } from "@/shared/types/wsEvents";

const PEEK_MS = 2000;
const PEEK_MAX_CHARS = 90;
const MAX_MESSAGE_LENGTH = 500;

type Variant = "lobby" | "room" | "match";

interface ChatDockBaseProps {
  /** Extra class(es) applied to the dock root(s) — used by the in-game wrapper
   *  to attach the `.chat-dock-match` skin. */
  className?: string;
  /** Resolve a sender's username color (team tinting). When omitted, usernames
   *  render in the default muted style (lobby/global chat has no teams). */
  resolveNameColor?: (userId: number) => string | undefined;
  /** Controlled open state. When `onOpenChange` is provided the dock is
   *  controlled (the in-game dock lifts this so the HUD can react); otherwise
   *  it manages its own open state internally. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

type ChatDockProps = ChatDockBaseProps &
  (
    | { variant: "lobby"; roomId?: never }
    | { variant: "room"; roomId: number }
    | { variant: "match"; roomId: number }
  );

/**
 * Bottom-right floating chat dock, shared across lobby (global channel), room
 * (room-scoped channel), and the in-game table (match channel). Closed state is
 * a 56px FAB with an unread badge + 2-second "peek" bubble for each new incoming
 * message. Open state is a 340×480 panel docked to the same corner.
 *
 * Lifecycle:
 *   - The unread counter increments for every incoming message that isn't
 *     mine while the dock is closed; opening the dock clears it.
 *   - The peek auto-dismisses PEEK_MS after its last arrival; new arrivals
 *     reset the timer so the latest message always gets its full window.
 *   - Pre-existing messages (received before this component mounted in the
 *     same session) are NOT replayed as peek bubbles — only messages that
 *     arrive while the dock is mounted + closed trigger the peek.
 *
 * The variant selects which chat-store slice to read/write and which i18n key
 * namespace to use, so all three docks share visual chrome, lifecycle logic,
 * and data-testids while staying scoped to their own channel. Per-channel
 * sender coloring is supplied by the wrapper via `resolveNameColor`, keeping
 * this component free of any lobby- or game-specific store coupling. The felt
 * theme is purely a CSS re-skin (`.chat-dock-match`), not a code branch.
 */
export function ChatDock(props: ChatDockProps) {
  const { variant, resolveNameColor, className } = props;
  const { t } = useTranslation();

  const messages = useChatStore((s) =>
    variant === "match" ? s.matchMessages : variant === "room" ? s.roomMessages : s.lobbyMessages,
  );
  const markSent = useChatStore((s) =>
    variant === "match" ? s.markSentMatch : variant === "room" ? s.markSentRoom : s.markSentLobby,
  );
  const me = useAuthStore((s) => s.user);
  const sendWs = useWsSendMessage();
  const connectionState = useWsConnectionState();
  const isConnected = connectionState === "connected";

  // Open state: controlled when the parent wires `onOpenChange` (in-game dock),
  // otherwise self-managed (lobby/room).
  const [internalOpen, setInternalOpen] = useState(false);
  const isControlled = props.onOpenChange != null;
  const open = isControlled ? props.open === true : internalOpen;
  const setOpen = (next: boolean) => {
    if (isControlled) props.onOpenChange?.(next);
    else setInternalOpen(next);
  };

  const [draft, setDraft] = useState("");
  const [unread, setUnread] = useState(0);
  const [peekVisible, setPeekVisible] = useState(false);
  const seenCountRef = useRef(messages.length);

  // Listen for new incoming messages — increment unread + show peek (closed),
  // or clear (open). Skip messages from the local user.
  useEffect(() => {
    if (messages.length < seenCountRef.current) {
      // Buffer shrank (channel cleared / match teardown) — drop any stale
      // unread + peek and resync, so the badge never lingers over an empty
      // history.
      seenCountRef.current = messages.length;
      setUnread(0);
      setPeekVisible(false);
      return;
    }
    if (messages.length === seenCountRef.current) return;
    const newMsgs = messages.slice(seenCountRef.current);
    seenCountRef.current = messages.length;
    const incomingFromOthers = newMsgs.filter((m) => m.userId !== me?.id);
    if (incomingFromOthers.length === 0) return;
    if (open) return;
    setUnread((u) => u + incomingFromOthers.length);
    setPeekVisible(true);
  }, [messages, me?.id, open]);

  // Auto-dismiss peek after PEEK_MS; resets on every new arrival.
  useEffect(() => {
    if (!peekVisible) return;
    const handle = window.setTimeout(() => setPeekVisible(false), PEEK_MS);
    return () => window.clearTimeout(handle);
  }, [peekVisible, messages.length]);

  // Opening the dock clears unread + hides peek immediately.
  useEffect(() => {
    if (open) {
      setUnread(0);
      setPeekVisible(false);
    }
  }, [open]);

  // Auto-scroll to the bottom when new messages arrive (open state only).
  const listEndRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (open) listEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [open, messages.length]);

  function send() {
    const text = draft.trim();
    if (!text || !isConnected) return;
    const trimmed = text.slice(0, MAX_MESSAGE_LENGTH);
    const payload: ChatMessageRequest =
      props.variant === "match"
        ? { channel: "match", roomId: props.roomId, text: trimmed }
        : props.variant === "room"
          ? { channel: "room", roomId: props.roomId, text: trimmed }
          : { channel: "lobby", text: trimmed };
    sendWs(ACTION_CHAT_MESSAGE, payload);
    markSent();
    setDraft("");
  }

  const peek = useMemo(() => peekPayload(messages, me?.id), [messages, me?.id]);
  const keys = useMemo(() => i18nKeys(variant), [variant]);
  const testIdRoot =
    variant === "match" ? "match-chat" : variant === "room" ? "room-chat" : "lobby-chat";

  // Felt theme is a pure CSS re-skin: `.chat-dock-match` re-points the accent +
  // surface tokens, and `backdrop-blur` frosts the translucent panel/FAB/peek.
  const skin = variant === "match" ? "chat-dock-match" : "";
  const frosted = variant === "match" ? "backdrop-blur-md" : "";

  // ── Closed state ──────────────────────────────────────────────────────
  if (!open) {
    return (
      <div
        data-testid={`${testIdRoot}-dock`}
        className={cn(
          "fixed right-4.5 bottom-4.5 z-40 flex flex-col items-end gap-2.5",
          skin,
          className,
        )}
      >
        {peek && peekVisible && (
          <div
            data-testid={`${testIdRoot}-peek`}
            className={cn(
              "bg-surface-elevated max-w-65 rounded-2xl border border-border px-3 py-2 shadow-(--chat-shadow-fab) animate-[card-in_.2s_ease_both]",
              frosted,
            )}
          >
            <div
              className="text-brass-deep text-[10px] font-bold uppercase tracking-[0.8px]"
              style={{ color: resolveNameColor?.(peek.userId) }}
            >
              {peek.username}
            </div>
            <div className="text-ink mt-0.5 text-xs leading-snug">{peek.text}</div>
          </div>
        )}
        <button
          onClick={() => setOpen(true)}
          aria-label={t(keys.openLabel)}
          data-testid={`${testIdRoot}-fab`}
          className={cn(
            "bg-surface text-ink relative inline-flex size-14 items-center justify-center rounded-full transition-transform hover:-translate-y-0.5",
            frosted,
            unread > 0
              ? "border border-brass shadow-[0_0_0_3px_var(--brass-soft),0_10px_28px_-10px_rgba(14,58,36,0.35)]"
              : "border-border-2 border shadow-(--chat-shadow-fab)",
          )}
        >
          <MessageSquare className="size-5.5" strokeWidth={1.8} />
          {unread > 0 && (
            <span
              data-testid={`${testIdRoot}-unread`}
              className="bg-brass text-brass-ink border-surface absolute -top-0.5 -right-0.5 inline-flex h-5 min-w-5 items-center justify-center rounded-full border-2 px-1.5 text-[11px] font-bold leading-none tabular-nums shadow-[0_0_10px_rgba(201,168,118,0.55)]"
            >
              {unread > 99 ? "99+" : unread}
            </span>
          )}
        </button>
      </div>
    );
  }

  // ── Open state ────────────────────────────────────────────────────────
  return (
    <aside
      data-testid={`${testIdRoot}-dock`}
      className={cn(
        // Full-screen overlay on phones so chat is comfortable to read/type;
        // the docked 340×480 corner panel returns at sm+ (desktop treatment).
        "bg-surface fixed inset-0 z-50 flex flex-col overflow-hidden sm:inset-auto sm:right-4.5 sm:bottom-4.5 sm:h-120 sm:w-85 sm:animate-[card-in_.18s_ease_both] sm:rounded-lg sm:border sm:border-border sm:shadow-(--chat-shadow-panel)",
        frosted,
        skin,
        className,
      )}
    >
      <div className="flex items-center gap-2 border-b border-border px-3.5 py-3">
        <MessageSquare className="text-accent size-3.5" />
        <span className="font-display text-ink text-sm font-semibold">{t(keys.title)}</span>
        <button
          onClick={() => setOpen(false)}
          aria-label={t(keys.closeLabel)}
          className="text-ink-dim ml-auto p-1"
          data-testid={`${testIdRoot}-close`}
        >
          <X className="size-3.5" />
        </button>
      </div>

      <div
        className="flex flex-1 flex-col gap-2.5 overflow-y-auto px-3.5 py-3"
        data-testid={`${testIdRoot}-list`}
      >
        {messages.map((m) => (
          <ChatLine
            key={`${m.userId}-${m.timestamp}-${m.message}`}
            m={m}
            mine={m.userId === me?.id}
            nameColor={resolveNameColor?.(m.userId)}
          />
        ))}
        <div ref={listEndRef} />
      </div>

      <div className="flex items-center gap-2 border-t border-border px-3 py-2.5">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && send()}
          placeholder={t(isConnected ? keys.placeholder : "chat.placeholderDisabled")}
          disabled={!isConnected}
          maxLength={MAX_MESSAGE_LENGTH}
          data-testid={`${testIdRoot}-input`}
          className="bg-surface-elevated text-ink flex-1 rounded-lg border border-border px-2.5 py-2 text-xs outline-none placeholder:text-ink-off disabled:cursor-not-allowed disabled:opacity-50"
        />
        <button
          onClick={send}
          disabled={!draft.trim() || !isConnected}
          aria-label={t(keys.sendLabel)}
          data-testid={`${testIdRoot}-send`}
          className="bg-accent text-accent-ink inline-flex size-8.5 items-center justify-center rounded-lg disabled:opacity-50"
        >
          <Send className="size-3.5" strokeWidth={2} />
        </button>
      </div>
    </aside>
  );
}

function i18nKeys(variant: Variant) {
  if (variant === "match") {
    return {
      openLabel: "match.chat.toggleOpen",
      closeLabel: "match.chat.toggleClose",
      sendLabel: "match.chat.sendLabel",
      title: "match.chat.title",
      placeholder: "match.chat.placeholder",
    };
  }
  if (variant === "room") {
    return {
      openLabel: "room.chat.openLabel",
      closeLabel: "room.chat.closeLabel",
      sendLabel: "room.chat.sendLabel",
      title: "room.chat.title",
      placeholder: "room.chat.placeholder",
    };
  }
  return {
    openLabel: "lobby.chat.openLabel",
    closeLabel: "lobby.chat.closeLabel",
    sendLabel: "lobby.chat.sendLabel",
    title: "lobby.chat.title",
    placeholder: "lobby.chat.placeholder",
  };
}

function peekPayload(messages: ChatMessagePayload[], myId: number | undefined) {
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i];
    if (!m || m.userId === myId) continue;
    const text =
      m.message.length > PEEK_MAX_CHARS ? `${m.message.slice(0, PEEK_MAX_CHARS - 1)}…` : m.message;
    return { userId: m.userId, username: m.username, text };
  }
  return null;
}

function ChatLine({
  m,
  mine,
  nameColor,
}: {
  m: ChatMessagePayload;
  mine: boolean;
  nameColor?: string;
}) {
  return (
    <div className={cn("flex flex-col gap-0.5", mine ? "items-end" : "items-start")}>
      {!mine && (
        <span className="text-ink-mute pl-0.5 text-[11px]">
          {nameColor ? (
            <strong className="font-semibold" style={{ color: nameColor }}>
              {m.username}
            </strong>
          ) : (
            m.username
          )}{" "}
          · <RelativeTime iso={m.timestamp} variant="compact" />
        </span>
      )}
      <span
        className={cn(
          "max-w-[85%] rounded-2xl border px-2.5 py-1.5 text-xs",
          mine
            ? "bg-accent-soft border-accent/40 text-accent"
            : "bg-surface-elevated border-border text-ink",
        )}
      >
        {m.message}
      </span>
    </div>
  );
}
