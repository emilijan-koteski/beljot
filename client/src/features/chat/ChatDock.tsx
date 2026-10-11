import { MessageSquare, Send, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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

// A chat line normalized across the primary channel and whisper threads.
interface DisplayLine {
  key: string;
  userId: number;
  username: string;
  message: string;
  timestamp: string;
}

// Parses a `/w <username> <message>` command, returning null when the text is not one.
// An incomplete command returns { complete: false } so it is swallowed instead of sent publicly.
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
const FAB_CHIP_CLASS =
  "border-surface relative inline-flex h-5 min-w-5 items-center justify-center rounded-full border-2 px-1.5 text-[11px] font-bold leading-none tabular-nums";
const PENDING_WHISPER_MS = 5000;

type Variant = "lobby" | "room" | "match";

interface ChatDockBaseProps {
  className?: string;
  resolveNameColor?: (userId: number) => string | undefined;
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
 * Floating bottom-right chat dock shared by the lobby, room, and match channels.
 * Closed it is a FAB with unread chips and a peek bubble; open it is a panel with whisper tabs.
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

  const whisperThreads = useChatStore((s) => s.whisperThreads);
  const whisperUnread = useChatStore((s) => s.whisperUnread);
  const activeChannel = useChatStore((s) => s.activeChannel);
  const setActiveChannel = useChatStore((s) => s.setActiveChannel);
  const setDockOpen = useChatStore((s) => s.setDockOpen);
  const whisperOpenRequest = useChatStore((s) => s.whisperOpenRequest);

  const threadKeys = useMemo(() => Object.keys(whisperThreads).sort(), [whisperThreads]);
  const whisperUnreadTotal = useMemo(
    () => Object.values(whisperUnread).reduce((sum, n) => sum + n, 0),
    [whisperUnread],
  );
  const activeThread =
    activeChannel !== "primary" && whisperThreads[activeChannel.slice("whisper:".length)]
      ? activeChannel.slice("whisper:".length)
      : null;

  const [internalOpen, setInternalOpen] = useState(false);
  const onOpenChange = props.onOpenChange;
  const isControlled = onOpenChange != null;
  const open = isControlled ? props.open === true : internalOpen;
  const setOpen = useCallback(
    (next: boolean) => {
      if (onOpenChange) onOpenChange(next);
      else setInternalOpen(next);
    },
    [onOpenChange],
  );

  const [draft, setDraft] = useState("");
  const [unread, setUnread] = useState(0);
  const [peekVisible, setPeekVisible] = useState(false);
  const seenCountRef = useRef(messages.length);

  useEffect(() => {
    if (messages.length < seenCountRef.current) {
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

  useEffect(() => {
    if (!peekVisible) return;
    const handle = window.setTimeout(() => setPeekVisible(false), PEEK_MS);
    return () => window.clearTimeout(handle);
  }, [peekVisible, messages.length]);

  useEffect(() => {
    if (open) {
      setUnread(0);
      setPeekVisible(false);
    }
  }, [open]);

  useEffect(() => {
    setDockOpen(open);
    return () => setDockOpen(false);
  }, [open, setDockOpen]);

  const handledWhisperOpen = useRef(whisperOpenRequest);
  useEffect(() => {
    if (whisperOpenRequest === handledWhisperOpen.current) return;
    handledWhisperOpen.current = whisperOpenRequest;
    setOpen(true);
  }, [whisperOpenRequest, setOpen]);

  const isPhone = useMediaQuery("(max-width: 639px)");
  const viewportBox = useVisualViewport(open && isPhone);

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

  const pendingWhisper = useRef<{ username: string; at: number } | null>(null);
  useEffect(() => {
    const pending = pendingWhisper.current;
    if (!pending) return;
    const key = Object.keys(whisperThreads).find(
      (k) => k.toLowerCase() === pending.username.toLowerCase(),
    );
    if (!key) {
      if (Date.now() - pending.at > PENDING_WHISPER_MS) pendingWhisper.current = null;
      return;
    }
    pendingWhisper.current = null;
    setActiveChannel(whisperChannel(key));
  }, [whisperThreads, setActiveChannel]);

  function send() {
    if (!isConnected) return;

    const whisper = parseWhisperCommand(draft);
    if (whisper) {
      if (whisper.complete) {
        pendingWhisper.current = { username: whisper.toUsername, at: Date.now() };
        sendWhisper(whisper.toUsername, whisper.text);
      }
      return;
    }

    const text = draft.trim();
    if (!text) return;

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

  // Cycles through the primary channel and each whisper thread; Shift+Tab reverses.
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

  const skin = variant === "match" ? "chat-dock-match" : "";
  const frosted = variant === "match" ? "backdrop-blur-md" : "";

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

  if (!open) {
    const hasPublicUnread = unread > 0;
    const hasWhisperUnread = whisperUnreadTotal > 0;
    const bothChips = hasPublicUnread && hasWhisperUnread;
    const overflowLabel = bothChips ? "9+" : "99+";
    const chipLabel = (n: number) => (n > 99 ? overflowLabel : String(n));
    const fabLabel = [
      t(keys.openLabel),
      hasPublicUnread ? t("chat.unreadCount", { count: unread }) : null,
      hasWhisperUnread ? t("whisper.unreadCount", { count: whisperUnreadTotal }) : null,
    ]
      .filter(Boolean)
      .join(", ");
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
          aria-label={fabLabel}
          data-testid={`${testIdRoot}-fab`}
          className={cn(
            "bg-surface text-ink relative inline-flex size-14 items-center justify-center rounded-full transition-transform hover:-translate-y-0.5",
            frosted,
            hasPublicUnread || hasWhisperUnread
              ? "border border-brass shadow-[0_0_0_3px_var(--brass-soft),0_10px_28px_-10px_rgba(14,58,36,0.35)]"
              : "border-border-2 border shadow-(--chat-shadow-fab)",
          )}
        >
          <MessageSquare className="size-5.5" strokeWidth={1.8} />
          {(hasPublicUnread || hasWhisperUnread) && (
            <span className="absolute -top-0.5 -right-0.5 flex items-center">
              {hasPublicUnread && (
                <span
                  data-testid={`${testIdRoot}-unread`}
                  className={cn(
                    FAB_CHIP_CLASS,
                    "bg-brass text-brass-ink shadow-[0_0_10px_rgba(201,168,118,0.55)]",
                  )}
                >
                  {chipLabel(unread)}
                </span>
              )}
              {hasWhisperUnread && (
                <span
                  data-testid={`${testIdRoot}-whisper-unread`}
                  className={cn(
                    FAB_CHIP_CLASS,
                    "bg-(--whisper-badge) text-(--whisper-badge-ink) shadow-[0_0_10px_var(--whisper-badge-glow)]",
                    bothChips && "z-10 -ml-1.5",
                  )}
                >
                  {chipLabel(whisperUnreadTotal)}
                </span>
              )}
            </span>
          )}
        </button>
      </div>
    );
  }

  return (
    <aside
      data-testid={`${testIdRoot}-dock`}
      className={cn(
        "bg-surface fixed inset-0 z-50 flex flex-col overflow-hidden sm:inset-auto sm:right-4.5 sm:bottom-4.5 sm:h-120 sm:w-85 sm:animate-[card-in_.18s_ease_both] sm:rounded-lg sm:border sm:border-border sm:shadow-(--chat-shadow-panel)",
        frosted,
        skin,
        className,
      )}
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
          whisper ? "border-(--whisper-border)" : mine ? "border-accent/40" : "border-border",
          mine
            ? whisper
              ? "bg-(--whisper-fill) text-(--whisper-ink)"
              : "bg-accent-soft text-accent"
            : "bg-surface-elevated text-ink",
        )}
      >
        {message}
      </span>
    </div>
  );
}

// A channel switcher tab for the primary channel or a whisper thread, with an unread badge.
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
          className="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-(--whisper-badge) px-1 text-[9px] leading-none font-bold text-(--whisper-badge-ink) tabular-nums"
        >
          {unread > 9 ? "9+" : unread}
        </span>
      )}
    </button>
  );
}
