import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import type { WhisperPayload } from "@/shared/types/wsEvents";
import { ACTION_CHAT_MESSAGE, ACTION_WHISPER } from "@/shared/types/wsEvents";
import { makeUser } from "@/test-utils";

import { ChatDock } from "./ChatDock";

const mockSendMessage = vi.fn();
let mockConnectionState: "connected" | "connecting" | "authenticating" | "disconnected" =
  "connected";

vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsSendMessage: () => mockSendMessage,
  useWsConnectionState: () => mockConnectionState,
}));

// me = alice(1). bob(2)/carol(3) are the whisper counterparts.
function whisper(overrides: Partial<WhisperPayload> = {}): WhisperPayload {
  return {
    fromUserId: 2,
    fromUsername: "bob",
    toUserId: 1,
    toUsername: "alice",
    message: "psst",
    timestamp: "2026-04-18T12:00:00Z",
    ...overrides,
  };
}

function resetChat() {
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
}

beforeEach(() => {
  mockSendMessage.mockReset();
  mockConnectionState = "connected";
  resetChat();
  useAuthStore.setState({ user: makeUser({ id: 1, username: "alice" }) });
  Element.prototype.scrollIntoView = vi.fn();
});

afterEach(() => {
  vi.restoreAllMocks();
});

// Opens the (self-managed) lobby dock.
function renderOpenLobbyDock() {
  render(<ChatDock variant="lobby" />);
  fireEvent.click(screen.getByTestId("lobby-chat-fab"));
}

describe("ChatDock — whisper", () => {
  it("sends action:whisper for a /w command instead of a chat message", () => {
    renderOpenLobbyDock();
    const input = screen.getByTestId("lobby-chat-input") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "/w bob hi there" } });
    fireEvent.click(screen.getByTestId("lobby-chat-send"));

    expect(mockSendMessage).toHaveBeenCalledWith(ACTION_WHISPER, {
      toUsername: "bob",
      text: "hi there",
    });
    expect(mockSendMessage).not.toHaveBeenCalledWith(ACTION_CHAT_MESSAGE, expect.anything());
    expect(input.value).toBe("");
  });

  it("switches to the thread once a /w send comes back from the server", () => {
    renderOpenLobbyDock();
    const input = screen.getByTestId("lobby-chat-input") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "/w bruno_qa ej" } });
    fireEvent.click(screen.getByTestId("lobby-chat-send"));

    // Still on the primary channel: nothing has come back yet, and with no thread
    // to show the dock has no tab strip at all.
    expect(useChatStore.getState().activeChannel).toBe("primary");
    expect(screen.queryByTestId("lobby-chat-tab-primary")).not.toBeInTheDocument();

    // The server echoes the whisper — that echo IS the success signal.
    act(() => {
      useChatStore.getState().appendWhisper(
        whisper({
          fromUserId: 1,
          fromUsername: "alice",
          toUserId: 4,
          toUsername: "bruno_qa",
          message: "ej",
        }),
        1,
      );
    });

    expect(screen.getByTestId("lobby-chat-whisper-tab-bruno_qa")).toHaveAttribute(
      "data-active",
      "true",
    );
    expect(screen.getByTestId("lobby-chat-list")).toHaveTextContent("ej");
  });

  it("matches the server's spelling of the target, not the typed case", () => {
    renderOpenLobbyDock();
    const input = screen.getByTestId("lobby-chat-input") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "/w BRUNO_QA ej" } });
    fireEvent.click(screen.getByTestId("lobby-chat-send"));

    act(() => {
      useChatStore.getState().appendWhisper(
        whisper({
          fromUserId: 1,
          fromUsername: "alice",
          toUserId: 4,
          toUsername: "bruno_qa",
          message: "ej",
        }),
        1,
      );
    });

    expect(screen.getByTestId("lobby-chat-whisper-tab-bruno_qa")).toHaveAttribute(
      "data-active",
      "true",
    );
  });

  it("stays on the primary channel when a /w is rejected", () => {
    renderOpenLobbyDock();
    const input = screen.getByTestId("lobby-chat-input") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "/w stranger hi" } });
    fireEvent.click(screen.getByTestId("lobby-chat-send"));

    // A rejected whisper creates no thread (the error surfaces as a toast), so
    // there is nothing to switch to and the view must not move.
    expect(useChatStore.getState().whisperThreads).toEqual({});
    expect(useChatStore.getState().activeChannel).toBe("primary");
    expect(screen.queryByTestId("lobby-chat-whisper-tab-stranger")).not.toBeInTheDocument();
  });

  it("does NOT leak a partial /w command to the public channel", () => {
    renderOpenLobbyDock();
    const input = screen.getByTestId("lobby-chat-input") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "/w bob" } });
    fireEvent.click(screen.getByTestId("lobby-chat-send"));

    // Neither a whisper (no message yet) nor a public chat message is sent.
    expect(mockSendMessage).not.toHaveBeenCalled();
  });

  // The friend list's whisper button (Story 11.2 card) reaches the dock through
  // the store: it selects the thread and raises whisperOpenRequest, and a CLOSED
  // dock has to answer by opening on that thread.
  it("opens itself on the thread when openWhisper is called from outside", () => {
    render(<ChatDock variant="lobby" />);
    // Closed: only the FAB is on screen.
    expect(screen.getByTestId("lobby-chat-fab")).toBeInTheDocument();
    expect(screen.queryByTestId("lobby-chat-input")).not.toBeInTheDocument();

    act(() => {
      useChatStore.getState().openWhisper("bob");
    });

    expect(screen.getByTestId("lobby-chat-input")).toBeInTheDocument();
    expect(screen.getByTestId("lobby-chat-whisper-tab-bob")).toHaveAttribute("data-active", "true");
    // Composing now goes to bob without a /w prefix.
    const input = screen.getByTestId("lobby-chat-input") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "hey" } });
    fireEvent.click(screen.getByTestId("lobby-chat-send"));
    expect(mockSendMessage).toHaveBeenCalledWith(ACTION_WHISPER, {
      toUsername: "bob",
      text: "hey",
    });
  });

  it("does not spring open on a request raised before it mounted", () => {
    act(() => {
      useChatStore.getState().openWhisper("bob");
    });

    render(<ChatDock variant="lobby" />);

    // A stale counter from an earlier page must not pop this dock open.
    expect(screen.queryByTestId("lobby-chat-input")).not.toBeInTheDocument();
    expect(screen.getByTestId("lobby-chat-fab")).toBeInTheDocument();
  });

  it("renders a whisper tab + pink bubble once a thread exists", () => {
    renderOpenLobbyDock();
    act(() => {
      useChatStore.getState().appendWhisper(whisper({ message: "hey alice" }), 1);
    });

    // The friend's tab appears and the primary tab is present.
    expect(screen.getByTestId("lobby-chat-tab-primary")).toBeInTheDocument();
    const bobTab = screen.getByTestId("lobby-chat-whisper-tab-bob");
    expect(bobTab).toBeInTheDocument();

    // An incoming whisper on a non-active thread shows an unread badge.
    expect(screen.getByTestId("lobby-chat-whisper-tab-bob-unread")).toHaveTextContent("1");

    // Activating the thread renders its message in the list.
    fireEvent.click(bobTab);
    const list = screen.getByTestId("lobby-chat-list");
    expect(list).toHaveTextContent("hey alice");
    // Unread badge clears once the thread is active.
    expect(screen.queryByTestId("lobby-chat-whisper-tab-bob-unread")).not.toBeInTheDocument();
    expect(bobTab).toHaveAttribute("data-active", "true");
  });

  // Border marks the channel, fill marks the sender: an incoming whisper is
  // neutral parchment behind a pink hairline, mine is filled pink.
  it("gives an incoming whisper a neutral fill and a pink border", () => {
    renderOpenLobbyDock();
    act(() => {
      // Theirs (bob → me), then mine (me → bob) as the server echoes it back.
      useChatStore.getState().appendWhisper(whisper({ message: "theirs" }), 1);
      useChatStore.getState().appendWhisper(
        whisper({
          fromUserId: 1,
          fromUsername: "alice",
          toUserId: 2,
          toUsername: "bob",
          message: "mine",
        }),
        1,
      );
    });
    fireEvent.click(screen.getByTestId("lobby-chat-whisper-tab-bob"));

    const theirs = screen.getByText("theirs");
    expect(theirs).toHaveClass("bg-surface-elevated", "text-ink");
    expect(theirs).toHaveClass("border-(--whisper-border)");
    expect(theirs).not.toHaveClass("bg-(--whisper-fill)");

    const ours = screen.getByText("mine");
    expect(ours).toHaveClass("bg-(--whisper-fill)", "border-(--whisper-border)");
  });

  it("composing on an active whisper thread sends to that friend without /w", () => {
    renderOpenLobbyDock();
    act(() => {
      useChatStore.getState().appendWhisper(whisper(), 1);
    });
    fireEvent.click(screen.getByTestId("lobby-chat-whisper-tab-bob"));

    const input = screen.getByTestId("lobby-chat-input") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "reply to bob" } });
    fireEvent.click(screen.getByTestId("lobby-chat-send"));

    expect(mockSendMessage).toHaveBeenCalledWith(ACTION_WHISPER, {
      toUsername: "bob",
      text: "reply to bob",
    });
  });

  it("cycles channels with the Tab key", () => {
    renderOpenLobbyDock();
    act(() => {
      useChatStore.getState().appendWhisper(whisper(), 1); // bob
      useChatStore
        .getState()
        .appendWhisper(whisper({ fromUserId: 3, fromUsername: "carol", message: "yo" }), 1); // carol
    });

    const input = screen.getByTestId("lobby-chat-input");
    // primary → bob (threads are sorted: bob, carol)
    fireEvent.keyDown(input, { key: "Tab" });
    expect(screen.getByTestId("lobby-chat-whisper-tab-bob")).toHaveAttribute("data-active", "true");
    // bob → carol
    fireEvent.keyDown(input, { key: "Tab" });
    expect(screen.getByTestId("lobby-chat-whisper-tab-carol")).toHaveAttribute(
      "data-active",
      "true",
    );
    // carol → wraps back to primary
    fireEvent.keyDown(input, { key: "Tab" });
    expect(screen.getByTestId("lobby-chat-tab-primary")).toHaveAttribute("data-active", "true");
    // Shift+Tab reverses primary → carol
    fireEvent.keyDown(input, { key: "Tab", shiftKey: true });
    expect(screen.getByTestId("lobby-chat-whisper-tab-carol")).toHaveAttribute(
      "data-active",
      "true",
    );
  });

  it("shows no tabs when there are no whisper threads", () => {
    renderOpenLobbyDock();
    expect(screen.queryByTestId("lobby-chat-tabs")).not.toBeInTheDocument();
  });

  it("surfaces an incoming whisper on the closed FAB unread badge", () => {
    // Dock stays CLOSED (no fab click). Without this, an incoming whisper is
    // invisible until the dock is opened — the badge must reflect it.
    render(<ChatDock variant="lobby" />);
    expect(screen.queryByTestId("lobby-chat-unread")).not.toBeInTheDocument();

    act(() => {
      useChatStore.getState().appendWhisper(whisper({ message: "psst" }), 1);
    });

    // Numeric count only — the private message text is never previewed on the FAB.
    const badge = screen.getByTestId("lobby-chat-unread");
    expect(badge).toHaveTextContent("1");
    expect(badge).not.toHaveTextContent("psst");
  });

  it("badges a whisper that arrives on the last-selected thread after the dock is closed", () => {
    // The dock closing does not deselect the thread, so the store must stop
    // treating that thread as "on screen" — otherwise the whisper is marked
    // read on arrival and raises no badge at all.
    render(<ChatDock variant="lobby" />);
    fireEvent.click(screen.getByTestId("lobby-chat-fab"));
    act(() => {
      useChatStore.getState().appendWhisper(whisper(), 1);
    });
    fireEvent.click(screen.getByTestId("lobby-chat-whisper-tab-bob"));
    fireEvent.click(screen.getByTestId("lobby-chat-close"));

    act(() => {
      useChatStore.getState().appendWhisper(whisper({ message: "while you were away" }), 1);
    });

    expect(screen.getByTestId("lobby-chat-unread")).toHaveTextContent("1");
    expect(useChatStore.getState().whisperUnread.bob).toBe(1);
  });

  it("clears the active thread's unread when the dock is reopened", () => {
    render(<ChatDock variant="lobby" />);
    fireEvent.click(screen.getByTestId("lobby-chat-fab"));
    act(() => {
      useChatStore.getState().appendWhisper(whisper(), 1);
    });
    fireEvent.click(screen.getByTestId("lobby-chat-whisper-tab-bob"));
    fireEvent.click(screen.getByTestId("lobby-chat-close"));
    act(() => {
      useChatStore.getState().appendWhisper(whisper({ message: "while you were away" }), 1);
    });

    fireEvent.click(screen.getByTestId("lobby-chat-fab"));

    expect(useChatStore.getState().whisperUnread.bob).toBe(0);
    expect(screen.queryByTestId("lobby-chat-whisper-tab-bob-unread")).not.toBeInTheDocument();
  });

  it("counts a whisper as unread while no dock is mounted", () => {
    // Navigating to a page without a chat dock unmounts it; the thread is then
    // off screen regardless of which channel was last selected.
    const view = render(<ChatDock variant="lobby" />);
    fireEvent.click(screen.getByTestId("lobby-chat-fab"));
    act(() => {
      useChatStore.getState().appendWhisper(whisper(), 1);
    });
    fireEvent.click(screen.getByTestId("lobby-chat-whisper-tab-bob"));
    view.unmount();

    act(() => {
      useChatStore.getState().appendWhisper(whisper({ message: "offscreen" }), 1);
    });

    expect(useChatStore.getState().whisperUnread.bob).toBe(1);
  });
});
