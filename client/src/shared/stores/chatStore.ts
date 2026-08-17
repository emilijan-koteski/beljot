import { create } from "zustand";

import type { ChatMessagePayload, WhisperPayload } from "@/shared/types/wsEvents";

const MAX_MESSAGES = 200;

// A chat channel is either the dock's own primary channel (lobby/room/match) or
// an open whisper thread keyed by the OTHER participant's username. Threads are
// created lazily when the first whisper (sent or received) for that friend lands.
export type ChatChannel = "primary" | `whisper:${string}`;

export function whisperChannel(username: string): ChatChannel {
  return `whisper:${username}`;
}

interface ChatState {
  lobbyMessages: ChatMessagePayload[];
  matchMessages: ChatMessagePayload[];
  roomMessages: ChatMessagePayload[];
  // Whisper threads (Story 11.4) — ephemeral, keyed by the OTHER participant's
  // username. Global (not per-variant): a whisper is between two friends
  // regardless of which dock is open, and only one dock mounts at a time.
  whisperThreads: Record<string, WhisperPayload[]>;
  // Per-thread unread counts, keyed the same way. Incremented for an INCOMING
  // whisper whose thread is not the active channel; reset when that thread
  // becomes active (setActiveChannel / markThreadRead).
  whisperUnread: Record<string, number>;
  // The channel the mounted dock is currently showing. Global so it survives a
  // dock swap on navigation; the dock falls back to "primary" if it points at a
  // thread that no longer exists.
  activeChannel: ChatChannel;
  // Whether a dock is currently mounted AND open — i.e. whether activeChannel is
  // actually on screen. Without this, an incoming whisper on the last-selected
  // thread counts as "read" even though the dock is shut (or unmounted on a
  // page that has no dock), so it raises no tab badge and no FAB badge and the
  // user never learns it arrived. The dock keeps this in sync.
  dockOpen: boolean;
  // Monotonic counter of "open a whisper with X" requests raised outside the dock
  // (the friend list's whisper button). A COUNTER, not a boolean or a channel:
  // two clicks in a row must both land, and a dock that mounts later — a page
  // navigation — must not spring open on a stale value it never saw raised. The
  // dock records the value at mount and only reacts to increments past it.
  whisperOpenRequest: number;
  // Monotonic count of match messages received since the last clear.
  // Unlike matchMessages.length (which plateaus at MAX_MESSAGES once the ring
  // buffer is full), this counter keeps incrementing so unread-badge tracking
  // still sees every arrival. Reset to 0 by clearMatch.
  matchMessagesReceivedTotal: number;
  // Whether the local user has sent at least one message in each channel
  // since the channel was last cleared. Tracked explicitly (rather than
  // derived from messages) so it latches through ring-buffer eviction — a
  // busy global lobby can push the user's first message out of the
  // MAX_MESSAGES window, but the placeholder should still reflect "already
  // chatted". Reset by the corresponding clear* action so a new match or
  // room gets a fresh invitation placeholder.
  hasSentLobby: boolean;
  hasSentMatch: boolean;
  hasSentRoom: boolean;
  appendLobby: (msg: ChatMessagePayload) => void;
  appendMatch: (msg: ChatMessagePayload) => void;
  appendRoom: (msg: ChatMessagePayload) => void;
  markSentLobby: () => void;
  markSentMatch: () => void;
  markSentRoom: () => void;
  clearLobby: () => void;
  clearMatch: () => void;
  clearRoom: () => void;
  // Whisper actions (Story 11.4).
  appendWhisper: (msg: WhisperPayload, myUserId: number) => void;
  setActiveChannel: (channel: ChatChannel) => void;
  setDockOpen: (open: boolean) => void;
  markThreadRead: (username: string) => void;
  clearWhispers: () => void;
  /**
   * Start (or resume) a whisper conversation from OUTSIDE the dock — the friend
   * list's whisper button. Seeds an empty thread if there is none, selects it,
   * and asks the mounted dock to open via `whisperOpenRequest`.
   */
  openWhisper: (username: string) => void;
}

function appendWithCap<T>(buffer: T[], msg: T): T[] {
  const next = [...buffer, msg];
  if (next.length > MAX_MESSAGES) {
    next.splice(0, next.length - MAX_MESSAGES);
  }
  return next;
}

export const useChatStore = create<ChatState>((set) => ({
  lobbyMessages: [],
  matchMessages: [],
  roomMessages: [],
  whisperThreads: {},
  whisperUnread: {},
  activeChannel: "primary",
  dockOpen: false,
  whisperOpenRequest: 0,
  matchMessagesReceivedTotal: 0,
  hasSentLobby: false,
  hasSentMatch: false,
  hasSentRoom: false,
  appendLobby: (msg) =>
    set((state) => ({ lobbyMessages: appendWithCap(state.lobbyMessages, msg) })),
  appendMatch: (msg) =>
    set((state) => ({
      matchMessages: appendWithCap(state.matchMessages, msg),
      matchMessagesReceivedTotal: state.matchMessagesReceivedTotal + 1,
    })),
  appendRoom: (msg) => set((state) => ({ roomMessages: appendWithCap(state.roomMessages, msg) })),
  markSentLobby: () => set({ hasSentLobby: true }),
  markSentMatch: () => set({ hasSentMatch: true }),
  markSentRoom: () => set({ hasSentRoom: true }),
  clearLobby: () => set({ lobbyMessages: [], hasSentLobby: false }),
  clearMatch: () => set({ matchMessages: [], matchMessagesReceivedTotal: 0, hasSentMatch: false }),
  clearRoom: () => set({ roomMessages: [], hasSentRoom: false }),
  appendWhisper: (msg, myUserId) =>
    set((state) => {
      // Key by whichever participant is NOT me. The server delivers the SAME
      // payload to both, so this yields the same thread key on both ends.
      const isMine = msg.fromUserId === myUserId;
      const key = isMine ? msg.toUsername : msg.fromUsername;
      const thread = appendWithCap(state.whisperThreads[key] ?? [], msg);
      // Bump unread only for an incoming message the user cannot currently see:
      // the thread isn't the active channel, OR the dock is closed/unmounted so
      // even the active channel is off screen. Own-echo never counts as unread.
      const bump = !isMine && (!state.dockOpen || state.activeChannel !== whisperChannel(key));
      return {
        whisperThreads: { ...state.whisperThreads, [key]: thread },
        whisperUnread: bump
          ? { ...state.whisperUnread, [key]: (state.whisperUnread[key] ?? 0) + 1 }
          : state.whisperUnread,
      };
    }),
  setActiveChannel: (channel) =>
    set((state) => {
      if (channel === "primary") return { activeChannel: channel };
      // Switching to a whisper thread marks it read.
      const key = channel.slice("whisper:".length);
      return {
        activeChannel: channel,
        whisperUnread: { ...state.whisperUnread, [key]: 0 },
      };
    }),
  setDockOpen: (open) =>
    set((state) => {
      // Opening the dock reveals the active channel, so an already-counted
      // thread is now read. Without this the badge would linger in the store and
      // resurface on the FAB the next time the dock closes.
      if (!open || state.activeChannel === "primary") return { dockOpen: open };
      const key = state.activeChannel.slice("whisper:".length);
      return { dockOpen: true, whisperUnread: { ...state.whisperUnread, [key]: 0 } };
    }),
  markThreadRead: (username) =>
    set((state) => ({ whisperUnread: { ...state.whisperUnread, [username]: 0 } })),
  clearWhispers: () => set({ whisperThreads: {}, whisperUnread: {}, activeChannel: "primary" }),
  openWhisper: (username) =>
    set((state) => ({
      // Threads are born from the first message, so a friend you have never
      // whispered has none — seed an empty one, or the dock's activeThread guard
      // sees a channel pointing at nothing and falls back to the primary channel.
      whisperThreads: state.whisperThreads[username]
        ? state.whisperThreads
        : { ...state.whisperThreads, [username]: [] },
      activeChannel: whisperChannel(username),
      // Deliberately opening the thread reads whatever was waiting in it.
      whisperUnread: { ...state.whisperUnread, [username]: 0 },
      whisperOpenRequest: state.whisperOpenRequest + 1,
    })),
}));
