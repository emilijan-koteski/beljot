import "@/shared/i18n/i18n";

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useChatStore } from "@/shared/stores/chatStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import type { ChatMessagePayload, WhisperPayload } from "@/shared/types/wsEvents";
import { ACTION_CHAT_MESSAGE } from "@/shared/types/wsEvents";

import { MatchChatDock } from "./MatchChatDock";

// Re-creates the production controlled-component contract: MatchPage owns the
// open/closed flag and passes it down. Tests target behavior, not the wiring.
function ControlledDock({ initialOpen = false }: { initialOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(initialOpen);
  return <MatchChatDock isOpen={isOpen} onOpenChange={setIsOpen} />;
}

const mockSendMessage = vi.fn();
let mockConnectionState: "connected" | "connecting" | "authenticating" | "disconnected" =
  "connected";

vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsSendMessage: () => mockSendMessage,
  useWsConnectionState: () => mockConnectionState,
}));

function makeMatchMessage(overrides: Partial<ChatMessagePayload> = {}): ChatMessagePayload {
  return {
    userId: 1,
    username: "alice",
    message: "hi team",
    timestamp: "2026-04-18T12:00:00Z",
    scope: "match",
    ...overrides,
  };
}

// An incoming whisper from bob(2); `me` is 1 in every call below.
function makeWhisper(overrides: Partial<WhisperPayload> = {}): WhisperPayload {
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

beforeEach(() => {
  mockSendMessage.mockReset();
  mockConnectionState = "connected";
  useChatStore.setState({
    lobbyMessages: [],
    matchMessages: [],
    matchMessagesReceivedTotal: 0,
    hasSentMatch: false,
    whisperThreads: {},
    whisperUnread: {},
    activeChannel: "primary",
    dockOpen: false,
    whisperOpenRequest: 0,
  });
  useMatchStore.setState({ roomId: 42 });
  Element.prototype.scrollIntoView = vi.fn();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  useMatchStore.setState({ roomId: null });
});

describe("MatchChatDock", () => {
  it("renders the FAB (closed) when roomId is set", () => {
    render(<ControlledDock />);
    expect(screen.getByTestId("match-chat-fab")).toBeInTheDocument();
    expect(screen.queryByTestId("match-chat-input")).not.toBeInTheDocument();
  });

  it("does NOT render anything when roomId is null", () => {
    act(() => {
      useMatchStore.setState({ roomId: null });
    });
    const { container } = render(<ControlledDock />);
    expect(container).toBeEmptyDOMElement();
  });

  it("toggles the dock open/closed", () => {
    render(<ControlledDock />);

    fireEvent.click(screen.getByTestId("match-chat-fab"));
    expect(screen.getByTestId("match-chat-input")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("match-chat-close"));
    expect(screen.queryByTestId("match-chat-input")).not.toBeInTheDocument();
    expect(screen.getByTestId("match-chat-fab")).toBeInTheDocument();
  });

  it("opens directly when the controlled flag starts open", () => {
    render(<ControlledDock initialOpen />);
    expect(screen.getByTestId("match-chat-input")).toBeInTheDocument();
    expect(screen.queryByTestId("match-chat-fab")).not.toBeInTheDocument();
  });

  it("increments the unread badge when new messages arrive while closed", () => {
    render(<ControlledDock />);

    act(() => {
      useChatStore.getState().appendMatch(makeMatchMessage({ message: "m1" }));
      useChatStore.getState().appendMatch(makeMatchMessage({ message: "m2" }));
      useChatStore.getState().appendMatch(makeMatchMessage({ message: "m3" }));
    });

    expect(screen.getByTestId("match-chat-unread")).toHaveTextContent("3");
  });

  it("resets the unread count to 0 when the dock is opened", () => {
    render(<ControlledDock />);

    act(() => {
      useChatStore.getState().appendMatch(makeMatchMessage({ message: "m1" }));
      useChatStore.getState().appendMatch(makeMatchMessage({ message: "m2" }));
    });
    expect(screen.getByTestId("match-chat-unread")).toHaveTextContent("2");

    fireEvent.click(screen.getByTestId("match-chat-fab"));
    expect(screen.queryByTestId("match-chat-unread")).not.toBeInTheDocument();
  });

  it("does NOT increment the unread badge while the dock is open", () => {
    render(<ControlledDock initialOpen />);

    act(() => {
      useChatStore.getState().appendMatch(makeMatchMessage({ message: "while-open" }));
    });

    expect(screen.queryByTestId("match-chat-unread")).not.toBeInTheDocument();
  });

  it("caps the unread badge at 99+", () => {
    render(<ControlledDock />);

    act(() => {
      for (let i = 0; i < 120; i++) {
        useChatStore.getState().appendMatch(makeMatchMessage({ message: `m-${i}` }));
      }
    });

    expect(screen.getByTestId("match-chat-unread")).toHaveTextContent("99+");
  });

  it("renders the whisper chip beside the brass badge on the match FAB", () => {
    render(<ControlledDock />);

    act(() => {
      useChatStore
        .getState()
        .appendMatch(makeMatchMessage({ userId: 2, username: "bob", message: "public" }));
      useChatStore.getState().appendWhisper(makeWhisper(), 1);
    });

    expect(screen.getByTestId("match-chat-unread")).toHaveTextContent("1");
    expect(screen.getByTestId("match-chat-whisper-unread")).toHaveTextContent("1");
  });

  it("clears the unread badge when match history is cleared while closed", () => {
    render(<ControlledDock />);

    act(() => {
      useChatStore.getState().appendMatch(makeMatchMessage({ message: "pre-clear" }));
      useChatStore.getState().appendMatch(makeMatchMessage({ message: "pre-clear-2" }));
    });
    expect(screen.getByTestId("match-chat-unread")).toHaveTextContent("2");

    act(() => {
      useChatStore.getState().clearMatch();
    });

    expect(screen.queryByTestId("match-chat-unread")).not.toBeInTheDocument();
  });

  it("renders match messages inside the opened dock", () => {
    render(<ControlledDock initialOpen />);
    act(() => {
      useChatStore.setState({
        matchMessages: [makeMatchMessage({ userId: 2, username: "bob", message: "nice play" })],
      });
    });

    const list = screen.getByTestId("match-chat-list");
    expect(list).toHaveTextContent("bob");
    expect(list).toHaveTextContent("nice play");
  });

  it("sends a match-scoped message when composing", () => {
    render(<ControlledDock initialOpen />);

    const input = screen.getByTestId("match-chat-input") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "gg wp" } });
    fireEvent.click(screen.getByTestId("match-chat-send"));

    expect(mockSendMessage).toHaveBeenCalledWith(ACTION_CHAT_MESSAGE, {
      channel: "match",
      roomId: 42,
      text: "gg wp",
    });
    expect(input.value).toBe("");
  });

  it("disables the composer while disconnected", () => {
    mockConnectionState = "disconnected";
    render(<ControlledDock initialOpen />);

    const input = screen.getByTestId("match-chat-input") as HTMLInputElement;
    expect(input).toBeDisabled();
    expect(screen.getByTestId("match-chat-send")).toBeDisabled();
  });
});
