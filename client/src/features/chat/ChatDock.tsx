import { MessageSquare, Send, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { RelativeTime } from "@/shared/components/RelativeTime";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { useVisualViewport } from "@/shared/hooks/useVisualViewport";
import { cn } from "@/shared/lib/utils";
import { useWsConnectionState, useWsSendMessage } from "@/shared/providers/WebSocketContext";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore, whisperChannel } from "@/shared/stores/chatStore";
import type {
  ChatMessagePayload,
  ChatMessageRequest,
  WhisperRequest,
} from "@/shared/types/wsEvents";
import { ACTION_CHAT_MESSAGE, ACTION_WHISPER } from "@/shared/types/wsEvents";

// A single rendered line, normalized across the primary channel
// (ChatMessagePayload) and whisper threads (WhisperPayload) so ChatLine can
// render either. `whisper` toggles the pink skin.
interface DisplayLine {
  key: string;
  userId: number;
  username: string;
  message: string;
  timestamp: string;
}

// Parse a `/w <username> <message>` command. Returns null when the text is not a
// `/w` command at all. Returns { complete: false } for an in-progress command
// ("/w" or "/w bob" with no message yet) so the caller can swallow it rather
// than leak a partial command to the public channel.
function parseWhisperCommand(
  raw: string,
): { complete: true; toUsername: string; text: string } | { complete: false } | null {
  const trimmed = raw.trimStart();
  if (!/^\/w(\s|$)/i.test(trimmed)) return null;
  const m = /^\/w\s+(\S+)\s+([\s\S]+)$/i.exec(trimmed.trim());
  if (!m) return { complete: false };
  const body = m[2]!.trim();
  if (!body) return { complete: false };
  return { complete: true, toUsername: m[1]!, text: body };
}

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

  // Whisper channel state (Story 11.4). Threads are global (keyed by the other
  // participant's username); only one dock mounts at a time, so a single active
  // channel + tab strip is sufficient.
  const whisperThreads = useChatStore((s) => s.whisperThreads);
  const whisperUnread = useChatStore((s) => s.whisperUnread);
  const activeChannel = useChatStore((s) => s.activeChannel);
  const setActiveChannel = useChatStore((s) => s.setActiveChannel);
  const setDockOpen = useChatStore((s) => s.setDockOpen);

  const threadKeys = useMemo(() => Object.keys(whisperThreads).sort(), [whisperThreads]);
  // Total unread across all whisper threads. Surfaced on the CLOSED FAB badge so an
  // incoming whisper is noticed even when the dock is shut — the per-thread tab
  // badges only render while the dock is open. Numeric only (no content peek), so
  // private message text never renders on the FAB.
  const whisperUnreadTotal = useMemo(
    () => Object.values(whisperUnread).reduce((sum, n) => sum + n, 0),
    [whisperUnread],
  );
  // Effective active thread: the store's activeChannel, but only if it still
  // points at a live thread — otherwise fall back to the primary channel. Keeps
  // the dock resilient to a thread being cleared out from under a stale tab.
  const activeThread =
    activeChannel !== "primary" && whisperThreads[activeChannel.slice("whisper:".length)]
      ? activeChannel.slice("whisper:".length)
      : null;

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

  // Tell the store whether the active channel is actually on screen. A closed
  // dock — or an unmounted one, on a page that has no dock at all — must still
  // count an incoming whisper as unread, even on the last-selected thread;
  // otherwise it lands silently with no tab badge and no FAB badge.
  useEffect(() => {
    setDockOpen(open);
    return () => setDockOpen(false);
  }, [open, setDockOpen]);

  // Phone treatment: the open panel is a full-screen overlay, but `fixed
  // inset-0` sizes to the LAYOUT viewport, which the mobile on-screen keyboard
  // does not shrink on iOS — the browser pans to the focused input and drags
  // the header off-screen. Pin the panel to the VISUAL viewport instead
  // (height + top offset) so header and composer stay visible above the
  // keyboard. Android is handled by `interactive-widget=resizes-content`
  // (index.html); desktop (sm+) keeps the corner panel untouched.
  const isPhone = useMediaQuery("(max-width: 639px)");
  const viewportBox = useVisualViewport(open && isPhone);

  // Keep the newest message in view (open state only). Smooth-scroll ONLY for
  // messages arriving while the panel is open — a short, watchable hop. Every
  // other trigger (opening the panel, keyboard show/hide resizing it) jumps
  // instantly: animating from the top through a long history on open is slow
  // and annoying. `prevCountRef` also tracks while closed, so history that
  // accumulated in the closed state doesn't smooth-scroll on open.
  const listEndRef = useRef<HTMLDivElement | null>(null);
  const prevCountRef = useRef(messages.length);
  useEffect(() => {
    const prevCount = prevCountRef.current;
    prevCountRef.current = messages.length;
    if (!open) return;
    const newMessageWhileOpen = messages.length > prevCount;
    listEndRef.current?.scrollIntoView({ behavior: newMessageWhileOpen ? "smooth" : "instant" });
  }, [open, messages.length, viewportBox?.height]);

  function sendWhisper(toUsername: string, text: string) {
    const req: WhisperRequest = { toUsername, text: text.slice(0, MAX_MESSAGE_LENGTH) };
    sendWs(ACTION_WHISPER, req);
    setDraft("");
  }

  function send() {
    if (!isConnected) return;

    // `/w <username> <message>` sends a whisper from ANY channel. A partial
    // command ("/w" / "/w bob") is swallowed so it never leaks to the public
    // channel; a non-`/w` message falls through to the channel logic below.
    const whisper = parseWhisperCommand(draft);
    if (whisper) {
      if (whisper.complete) sendWhisper(whisper.toUsername, whisper.text);
      return;
    }

    const text = draft.trim();
    if (!text) return;

    // Active whisper thread → the composer whispers that friend directly (no
    // `/w` prefix needed).
    if (activeThread) {
      sendWhisper(activeThread, text);
      return;
    }

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

  // Cycle channels with Tab (Valorant-style): primary → each open thread →
  // primary. Shift+Tab reverses. Only intercepts Tab when at least one whisper
  // thread exists, so accessibility tab-out is preserved in the common case.
  function cycleChannel(direction: 1 | -1) {
    if (threadKeys.length === 0) return;
    const channels = ["primary", ...threadKeys.map(whisperChannel)];
    const current = activeThread ? whisperChannel(activeThread) : "primary";
    const idx = channels.indexOf(current);
    const next = channels[(idx + direction + channels.length) % channels.length]!;
    setActiveChannel(next as "primary" | `whisper:${string}`);
  }

  const peek = useMemo(() => peekPayload(messages, me?.id), [messages, me?.id]);
  const keys = useMemo(() => i18nKeys(variant), [variant]);
  const testIdRoot =
    variant === "match" ? "match-chat" : variant === "room" ? "room-chat" : "lobby-chat";

  // Felt theme is a pure CSS re-skin: `.chat-dock-match` re-points the accent +
  // surface tokens, and `backdrop-blur` frosts the translucent panel/FAB/peek.
  const skin = variant === "match" ? "chat-dock-match" : "";
  const frosted = variant === "match" ? "backdrop-blur-md" : "";

  // The lines rendered in the open panel depend on the active channel: the
  // primary channel's own slice, or the active whisper thread (pink). Both are
  // normalized to DisplayLine so ChatLine renders either.
  const displayLines: DisplayLine[] = useMemo(() => {
    if (activeThread) {
      return (whisperThreads[activeThread] ?? []).map((m) => ({
        key: `w-${m.fromUserId}-${m.timestamp}-${m.message}`,
        userId: m.fromUserId,
        username: m.fromUsername,
        message: m.message,
        timestamp: m.timestamp,
      }));
    }
    return messages.map((m) => ({
      key: `${m.userId}-${m.timestamp}-${m.message}`,
      userId: m.userId,
      username: m.username,
      message: m.message,
      timestamp: m.timestamp,
    }));
  }, [activeThread, whisperThreads, messages]);

  // ── Closed state ──────────────────────────────────────────────────────
  if (!open) {
    // The FAB badge combines primary-channel unread with whisper unread so a
    // private message shut behind the closed dock is still noticed. Content is
    // never previewed here (the peek stays primary-channel only).
    const badgeCount = unread + whisperUnreadTotal;
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
            badgeCount > 0
              ? "border border-brass shadow-[0_0_0_3px_var(--brass-soft),0_10px_28px_-10px_rgba(14,58,36,0.35)]"
              : "border-border-2 border shadow-(--chat-shadow-fab)",
          )}
        >
          <MessageSquare className="size-5.5" strokeWidth={1.8} />
          {badgeCount > 0 && (
            <span
              data-testid={`${testIdRoot}-unread`}
              className="bg-brass text-brass-ink border-surface absolute -top-0.5 -right-0.5 inline-flex h-5 min-w-5 items-center justify-center rounded-full border-2 px-1.5 text-[11px] font-bold leading-none tabular-nums shadow-[0_0_10px_rgba(201,168,118,0.55)]"
            >
              {badgeCount > 99 ? "99+" : badgeCount}
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
      // Present only on phones with the VisualViewport API (see above): sizes
      // the overlay to the visible area, overriding inset-0's top/bottom.
      style={
        viewportBox
          ? { top: viewportBox.offsetTop, height: viewportBox.height, bottom: "auto" }
          : undefined
      }
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

      {threadKeys.length > 0 && (
        <div
          role="tablist"
          aria-label={t("whisper.tablistLabel")}
          className="flex items-center gap-1 overflow-x-auto border-b border-border px-2 py-1.5"
          data-testid={`${testIdRoot}-tabs`}
        >
          <ChannelTab
            label={t("whisper.primaryTab")}
            active={activeThread === null}
            onClick={() => setActiveChannel("primary")}
            testId={`${testIdRoot}-tab-primary`}
          />
          {threadKeys.map((key) => (
            <ChannelTab
              key={key}
              label={key}
              whisper
              active={activeThread === key}
              unread={whisperUnread[key] ?? 0}
              onClick={() => setActiveChannel(whisperChannel(key))}
              testId={`${testIdRoot}-whisper-tab-${key}`}
            />
          ))}
        </div>
      )}

      <div
        className="flex flex-1 flex-col gap-2.5 overflow-y-auto overscroll-contain px-3.5 py-3"
        data-testid={`${testIdRoot}-list`}
      >
        {displayLines.map((line) => (
          <ChatLine
            key={line.key}
            username={line.username}
            message={line.message}
            timestamp={line.timestamp}
            mine={line.userId === me?.id}
            nameColor={activeThread ? undefined : resolveNameColor?.(line.userId)}
            whisper={activeThread !== null}
          />
        ))}
        <div ref={listEndRef} />
      </div>

      <div className="flex items-center gap-2 border-t border-border px-3 py-2.5">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              send();
              return;
            }
            // Tab cycles channels when whisper threads exist (Valorant-style);
            // otherwise it keeps its default focus-navigation behaviour.
            if (e.key === "Tab" && threadKeys.length > 0) {
              e.preventDefault();
              cycleChannel(e.shiftKey ? -1 : 1);
            }
          }}
          placeholder={
            !isConnected
              ? t("chat.placeholderDisabled")
              : activeThread
                ? t("whisper.inputPlaceholder", { username: activeThread })
                : t(keys.placeholder)
          }
          disabled={!isConnected}
          maxLength={MAX_MESSAGE_LENGTH}
          title={t("whisper.hint")}
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
  username,
  message,
  timestamp,
  mine,
  nameColor,
  whisper = false,
}: {
  username: string;
  message: string;
  timestamp: string;
  mine: boolean;
  nameColor?: string;
  whisper?: boolean;
}) {
  return (
    <div className={cn("flex flex-col gap-0.5", mine ? "items-end" : "items-start")}>
      {!mine && (
        <span className="text-ink-mute pl-0.5 text-[11px]">
          {whisper ? (
            <strong className="font-semibold text-(--whisper-name)">{username}</strong>
          ) : nameColor ? (
            <strong className="font-semibold" style={{ color: nameColor }}>
              {username}
            </strong>
          ) : (
            username
          )}{" "}
          · <RelativeTime iso={timestamp} variant="compact" />
        </span>
      )}
      <span
        className={cn(
          "max-w-[85%] rounded-2xl border px-2.5 py-1.5 text-xs",
          // Whisper bubbles are pink for BOTH participants so a private message
          // is unmistakable regardless of sender; alignment still marks mine vs
          // theirs. The primary channel keeps the accent/surface treatment.
          whisper
            ? "border-(--whisper-border) bg-(--whisper-fill) text-(--whisper-ink)"
            : mine
              ? "bg-accent-soft border-accent/40 text-accent"
              : "bg-surface-elevated border-border text-ink",
        )}
      >
        {message}
      </span>
    </div>
  );
}

// A single channel tab in the whisper switcher — the primary channel or a per
// friend whisper thread. Renders an unread dot when a non-active whisper thread
// has pending messages (Story 11.4 AC5).
function ChannelTab({
  label,
  active,
  onClick,
  testId,
  whisper = false,
  unread = 0,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
  testId: string;
  whisper?: boolean;
  unread?: number;
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      data-active={active}
      data-testid={testId}
      onClick={onClick}
      className={cn(
        "inline-flex max-w-32 items-center gap-1 truncate rounded-full border px-2.5 py-1 text-[11px] font-semibold transition-colors",
        active
          ? whisper
            ? "border-(--whisper-border) bg-(--whisper-fill) text-(--whisper-ink)"
            : "bg-accent-soft border-accent/40 text-accent"
          : "border-border text-ink-mute hover:text-ink",
      )}
    >
      <span className="truncate">{label}</span>
      {unread > 0 && !active && (
        <span
          data-testid={`${testId}-unread`}
          className="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-(--whisper-name) px-1 text-[9px] leading-none font-bold text-white tabular-nums"
        >
          {unread > 9 ? "9+" : unread}
        </span>
      )}
    </button>
  );
}
