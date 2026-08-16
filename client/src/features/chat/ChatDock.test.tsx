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

  it("does NOT leak a partial /w command to the public channel", () => {
    renderOpenLobbyDock();
    const input = screen.getByTestId("lobby-chat-input") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "/w bob" } });
    fireEvent.click(screen.getByTestId("lobby-chat-send"));

    // Neither a whisper (no message yet) nor a public chat message is sent.
    expect(mockSendMessage).not.toHaveBeenCalled();
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
