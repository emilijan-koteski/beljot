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
  whisperThreads: Record<string, WhisperPayload[]>;
  whisperUnread: Record<string, number>;
  activeChannel: ChatChannel;
  dockOpen: boolean;
  whisperOpenRequest: number;
  matchMessagesReceivedTotal: number;
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
  appendWhisper: (msg: WhisperPayload, myUserId: number) => void;
  setActiveChannel: (channel: ChatChannel) => void;
  setDockOpen: (open: boolean) => void;
  markThreadRead: (username: string) => void;
  clearWhispers: () => void;
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
      const isMine = msg.fromUserId === myUserId;
      const key = isMine ? msg.toUsername : msg.fromUsername;
      const thread = appendWithCap(state.whisperThreads[key] ?? [], msg);
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
      const key = channel.slice("whisper:".length);
      return {
        activeChannel: channel,
        whisperUnread: { ...state.whisperUnread, [key]: 0 },
      };
    }),
  setDockOpen: (open) =>
    set((state) => {
      if (!open || state.activeChannel === "primary") return { dockOpen: open };
      const key = state.activeChannel.slice("whisper:".length);
      return { dockOpen: true, whisperUnread: { ...state.whisperUnread, [key]: 0 } };
    }),
  markThreadRead: (username) =>
    set((state) => ({ whisperUnread: { ...state.whisperUnread, [username]: 0 } })),
  clearWhispers: () => set({ whisperThreads: {}, whisperUnread: {}, activeChannel: "primary" }),
  openWhisper: (username) =>
    set((state) => ({
      whisperThreads: state.whisperThreads[username]
        ? state.whisperThreads
        : { ...state.whisperThreads, [username]: [] },
      activeChannel: whisperChannel(username),
      whisperUnread: { ...state.whisperUnread, [username]: 0 },
      whisperOpenRequest: state.whisperOpenRequest + 1,
    })),
}));
