import { beforeEach, describe, expect, it } from "vitest";

import type { ChatMessagePayload, WhisperPayload } from "@/shared/types/wsEvents";

import { useChatStore } from "./chatStore";

function makeMessage(overrides: Partial<ChatMessagePayload> = {}): ChatMessagePayload {
  return {
    userId: 1,
    username: "alice",
    message: "hello",
    timestamp: "2026-04-18T10:00:00Z",
    scope: "lobby",
    ...overrides,
  };
}

// alice(1) is the local user in these fixtures; bob(2)/carol(3) are friends.
function makeWhisper(overrides: Partial<WhisperPayload> = {}): WhisperPayload {
  return {
    fromUserId: 2,
    fromUsername: "bob",
    toUserId: 1,
    toUsername: "alice",
    message: "psst",
    timestamp: "2026-04-18T10:00:00Z",
    ...overrides,
  };
}

describe("chatStore", () => {
  beforeEach(() => {
    useChatStore.setState({
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
    });
  });

  it("appendLobby adds a message to the end of the list", () => {
    useChatStore.getState().appendLobby(makeMessage({ message: "first" }));
    useChatStore.getState().appendLobby(makeMessage({ message: "second" }));

    const messages = useChatStore.getState().lobbyMessages;
    expect(messages).toHaveLength(2);
    expect(messages[0]!.message).toBe("first");
    expect(messages[1]!.message).toBe("second");
  });

  it("appendLobby drops oldest when exceeding 200-message cap", () => {
    for (let i = 0; i < 210; i++) {
      useChatStore.getState().appendLobby(makeMessage({ message: `msg-${i}` }));
    }

    const messages = useChatStore.getState().lobbyMessages;
    expect(messages).toHaveLength(200);
    expect(messages[0]!.message).toBe("msg-10");
    expect(messages[199]!.message).toBe("msg-209");
  });

  it("clearLobby resets the message list", () => {
    useChatStore.getState().appendLobby(makeMessage());
    useChatStore.getState().appendLobby(makeMessage());
    expect(useChatStore.getState().lobbyMessages).toHaveLength(2);

    useChatStore.getState().clearLobby();
    expect(useChatStore.getState().lobbyMessages).toHaveLength(0);
  });

  it("appendLobby produces a new array reference (immutable updates)", () => {
    const before = useChatStore.getState().lobbyMessages;
    useChatStore.getState().appendLobby(makeMessage());
    const after = useChatStore.getState().lobbyMessages;
    expect(after).not.toBe(before);
  });

  it("appendMatch adds a message to the match partition", () => {
    useChatStore.getState().appendMatch(makeMessage({ scope: "match", message: "team1" }));
    useChatStore.getState().appendMatch(makeMessage({ scope: "match", message: "team2" }));

    const messages = useChatStore.getState().matchMessages;
    expect(messages).toHaveLength(2);
    expect(messages[1]!.message).toBe("team2");
  });

  it("appendMatch drops oldest when exceeding 200-message cap", () => {
    for (let i = 0; i < 210; i++) {
      useChatStore.getState().appendMatch(makeMessage({ scope: "match", message: `m-${i}` }));
    }

    const messages = useChatStore.getState().matchMessages;
    expect(messages).toHaveLength(200);
    expect(messages[0]!.message).toBe("m-10");
    expect(messages[199]!.message).toBe("m-209");
  });

  it("clearMatch resets only the match partition", () => {
    useChatStore.getState().appendLobby(makeMessage());
    useChatStore.getState().appendMatch(makeMessage({ scope: "match" }));
    expect(useChatStore.getState().lobbyMessages).toHaveLength(1);
    expect(useChatStore.getState().matchMessages).toHaveLength(1);

    useChatStore.getState().clearMatch();
    expect(useChatStore.getState().matchMessages).toHaveLength(0);
    expect(useChatStore.getState().lobbyMessages).toHaveLength(1);
  });

  it("partitions are independent: appendLobby does not touch matchMessages", () => {
    useChatStore.getState().appendLobby(makeMessage());
    expect(useChatStore.getState().matchMessages).toHaveLength(0);
  });

  it("partitions are independent: appendMatch does not touch lobbyMessages", () => {
    useChatStore.getState().appendMatch(makeMessage({ scope: "match" }));
    expect(useChatStore.getState().lobbyMessages).toHaveLength(0);
  });

  it("appendMatch increments matchMessagesReceivedTotal monotonically, even past the 200 cap", () => {
    for (let i = 0; i < 210; i++) {
      useChatStore.getState().appendMatch(makeMessage({ scope: "match", message: `t-${i}` }));
    }

    const state = useChatStore.getState();
    expect(state.matchMessages).toHaveLength(200);
    // Length is capped but the monotonic counter keeps growing — required for
    // the sidebar's unread badge to stay accurate after the ring buffer fills.
    expect(state.matchMessagesReceivedTotal).toBe(210);
  });

  it("clearMatch resets matchMessagesReceivedTotal to 0", () => {
    useChatStore.getState().appendMatch(makeMessage({ scope: "match" }));
    useChatStore.getState().appendMatch(makeMessage({ scope: "match" }));
    expect(useChatStore.getState().matchMessagesReceivedTotal).toBe(2);

    useChatStore.getState().clearMatch();
    expect(useChatStore.getState().matchMessagesReceivedTotal).toBe(0);
  });

  it("appendLobby does NOT affect matchMessagesReceivedTotal", () => {
    useChatStore.getState().appendLobby(makeMessage());
    expect(useChatStore.getState().matchMessagesReceivedTotal).toBe(0);
  });

  // --- Room partition ---

  it("appendRoom adds a message to the room partition", () => {
    useChatStore.getState().appendRoom(makeMessage({ scope: "room", message: "r1" }));
    useChatStore.getState().appendRoom(makeMessage({ scope: "room", message: "r2" }));

    const messages = useChatStore.getState().roomMessages;
    expect(messages).toHaveLength(2);
    expect(messages[1]!.message).toBe("r2");
  });

  it("appendRoom drops oldest when exceeding 200-message cap", () => {
    for (let i = 0; i < 210; i++) {
      useChatStore.getState().appendRoom(makeMessage({ scope: "room", message: `r-${i}` }));
    }

    const messages = useChatStore.getState().roomMessages;
    expect(messages).toHaveLength(200);
    expect(messages[0]!.message).toBe("r-10");
    expect(messages[199]!.message).toBe("r-209");
  });

  it("clearRoom resets only the room partition", () => {
    useChatStore.getState().appendLobby(makeMessage());
    useChatStore.getState().appendMatch(makeMessage({ scope: "match" }));
    useChatStore.getState().appendRoom(makeMessage({ scope: "room" }));

    useChatStore.getState().clearRoom();
    const state = useChatStore.getState();
    expect(state.roomMessages).toHaveLength(0);
    expect(state.lobbyMessages).toHaveLength(1);
    expect(state.matchMessages).toHaveLength(1);
  });

  it("partitions are independent: appendRoom does not touch global or match", () => {
    useChatStore.getState().appendRoom(makeMessage({ scope: "room" }));
    expect(useChatStore.getState().lobbyMessages).toHaveLength(0);
    expect(useChatStore.getState().matchMessages).toHaveLength(0);
  });

  it("appendRoom does NOT affect matchMessagesReceivedTotal", () => {
    useChatStore.getState().appendRoom(makeMessage({ scope: "room" }));
    expect(useChatStore.getState().matchMessagesReceivedTotal).toBe(0);
  });

  // --- hasSent* placeholder flags ---

  it("markSent flags are independent across channels", () => {
    useChatStore.getState().markSentLobby();
    let state = useChatStore.getState();
    expect(state.hasSentLobby).toBe(true);
    expect(state.hasSentMatch).toBe(false);
    expect(state.hasSentRoom).toBe(false);

    useChatStore.getState().markSentMatch();
    useChatStore.getState().markSentRoom();
    state = useChatStore.getState();
    expect(state.hasSentMatch).toBe(true);
    expect(state.hasSentRoom).toBe(true);
  });

  it("clear* resets the matching hasSent flag", () => {
    useChatStore.setState({ hasSentLobby: true, hasSentMatch: true, hasSentRoom: true });

    useChatStore.getState().clearLobby();
    expect(useChatStore.getState().hasSentLobby).toBe(false);
    expect(useChatStore.getState().hasSentMatch).toBe(true);
    expect(useChatStore.getState().hasSentRoom).toBe(true);

    useChatStore.getState().clearMatch();
    expect(useChatStore.getState().hasSentMatch).toBe(false);
    expect(useChatStore.getState().hasSentRoom).toBe(true);

    useChatStore.getState().clearRoom();
    expect(useChatStore.getState().hasSentRoom).toBe(false);
  });

  // --- Whisper threads (Story 11.4) ---

  it("appendWhisper keys an incoming thread by the sender's username", () => {
    // me = alice(1); bob(2) whispers me → thread keyed by "bob".
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    const state = useChatStore.getState();
    expect(Object.keys(state.whisperThreads)).toEqual(["bob"]);
    expect(state.whisperThreads.bob).toHaveLength(1);
    expect(state.whisperThreads.bob![0]!.message).toBe("psst");
  });

  it("appendWhisper keys an outgoing (own-echo) thread by the recipient's username", () => {
    // me = alice(1); my own-echo to bob → still keyed by "bob".
    const echo = makeWhisper({
      fromUserId: 1,
      fromUsername: "alice",
      toUserId: 2,
      toUsername: "bob",
      message: "hi bob",
    });
    useChatStore.getState().appendWhisper(echo, 1);
    const state = useChatStore.getState();
    expect(Object.keys(state.whisperThreads)).toEqual(["bob"]);
    expect(state.whisperThreads.bob![0]!.message).toBe("hi bob");
  });

  it("appendWhisper bumps unread for an incoming message on a non-active thread", () => {
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    expect(useChatStore.getState().whisperUnread.bob).toBe(1);
    useChatStore.getState().appendWhisper(makeWhisper({ message: "again" }), 1);
    expect(useChatStore.getState().whisperUnread.bob).toBe(2);
  });

  it("appendWhisper does NOT bump unread for own-echo", () => {
    const echo = makeWhisper({
      fromUserId: 1,
      fromUsername: "alice",
      toUserId: 2,
      toUsername: "bob",
    });
    useChatStore.getState().appendWhisper(echo, 1);
    expect(useChatStore.getState().whisperUnread.bob ?? 0).toBe(0);
  });

  it("appendWhisper does NOT bump unread when its thread is the active channel and the dock is open", () => {
    useChatStore.getState().setDockOpen(true);
    useChatStore.getState().setActiveChannel("whisper:bob");
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    expect(useChatStore.getState().whisperUnread.bob).toBe(0);
  });

  it("appendWhisper bumps unread on the active thread when the dock is CLOSED", () => {
    // The active channel is only "read" while it is on screen. A closed (or
    // unmounted) dock must still count the whisper, or it lands silently.
    useChatStore.getState().setDockOpen(true);
    useChatStore.getState().setActiveChannel("whisper:bob");
    useChatStore.getState().setDockOpen(false);
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    useChatStore.getState().appendWhisper(makeWhisper({ message: "again" }), 1);
    expect(useChatStore.getState().whisperUnread.bob).toBe(2);
  });

  it("setDockOpen(true) marks the active whisper thread read", () => {
    useChatStore.getState().setActiveChannel("whisper:bob");
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    expect(useChatStore.getState().whisperUnread.bob).toBe(1);
    useChatStore.getState().setDockOpen(true);
    expect(useChatStore.getState().whisperUnread.bob).toBe(0);
  });

  it("setDockOpen(true) on the primary channel leaves whisper unread intact", () => {
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    useChatStore.getState().setDockOpen(true);
    expect(useChatStore.getState().whisperUnread.bob).toBe(1);
    expect(useChatStore.getState().activeChannel).toBe("primary");
  });

  it("appendWhisper drops oldest when exceeding the 200-message cap", () => {
    for (let i = 0; i < 210; i++) {
      useChatStore.getState().appendWhisper(makeWhisper({ message: `w-${i}` }), 1);
    }
    const thread = useChatStore.getState().whisperThreads.bob!;
    expect(thread).toHaveLength(200);
    expect(thread[0]!.message).toBe("w-10");
    expect(thread[199]!.message).toBe("w-209");
  });

  it("setActiveChannel to a whisper thread resets that thread's unread", () => {
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    expect(useChatStore.getState().whisperUnread.bob).toBe(1);
    useChatStore.getState().setActiveChannel("whisper:bob");
    expect(useChatStore.getState().whisperUnread.bob).toBe(0);
    expect(useChatStore.getState().activeChannel).toBe("whisper:bob");
  });

  it("markThreadRead zeroes a thread's unread", () => {
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    useChatStore.getState().appendWhisper(makeWhisper({ message: "b" }), 1);
    expect(useChatStore.getState().whisperUnread.bob).toBe(2);
    useChatStore.getState().markThreadRead("bob");
    expect(useChatStore.getState().whisperUnread.bob).toBe(0);
  });

  it("whisper threads are independent across friends", () => {
    useChatStore.getState().appendWhisper(makeWhisper(), 1); // from bob
    useChatStore
      .getState()
      .appendWhisper(makeWhisper({ fromUserId: 3, fromUsername: "carol", message: "yo" }), 1);
    const state = useChatStore.getState();
    expect(state.whisperThreads.bob).toHaveLength(1);
    expect(state.whisperThreads.carol).toHaveLength(1);
    expect(state.whisperUnread.bob).toBe(1);
    expect(state.whisperUnread.carol).toBe(1);
  });

  it("clearWhispers resets threads, unread, and active channel", () => {
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    useChatStore.getState().setActiveChannel("whisper:bob");
    useChatStore.getState().clearWhispers();
    const state = useChatStore.getState();
    expect(state.whisperThreads).toEqual({});
    expect(state.whisperUnread).toEqual({});
    expect(state.activeChannel).toBe("primary");
  });

  it("appendWhisper leaves the primary channels untouched", () => {
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    const state = useChatStore.getState();
    expect(state.lobbyMessages).toHaveLength(0);
    expect(state.matchMessages).toHaveLength(0);
    expect(state.roomMessages).toHaveLength(0);
  });

  // openWhisper is how a surface OUTSIDE the dock (the friend list's whisper
  // button) starts a conversation.
  it("openWhisper seeds an empty thread, selects it and raises an open request", () => {
    useChatStore.getState().openWhisper("bob");
    const state = useChatStore.getState();
    expect(state.whisperThreads.bob).toEqual([]);
    expect(state.activeChannel).toBe("whisper:bob");
    expect(state.whisperOpenRequest).toBe(1);
  });

  it("openWhisper keeps an existing thread's history and clears its unread", () => {
    useChatStore.getState().appendWhisper(makeWhisper(), 1);
    expect(useChatStore.getState().whisperUnread.bob).toBe(1);

    useChatStore.getState().openWhisper("bob");

    const state = useChatStore.getState();
    expect(state.whisperThreads.bob).toHaveLength(1);
    expect(state.whisperUnread.bob).toBe(0);
  });

  it("counts every openWhisper, so a second click still asks the dock to open", () => {
    useChatStore.getState().openWhisper("bob");
    useChatStore.getState().openWhisper("bob");
    expect(useChatStore.getState().whisperOpenRequest).toBe(2);
  });
});
