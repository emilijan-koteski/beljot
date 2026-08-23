import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import type { ChatMessagePayload, WhisperPayload } from "@/shared/types/wsEvents";
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

// A public lobby-channel message from someone else (bob), so it counts toward
// the brass unread chip. Own messages (userId 1) never do.
function lobbyMessage(overrides: Partial<ChatMessagePayload> = {}): ChatMessagePayload {
  return {
    userId: 2,
    username: "bob",
    message: "hello lobby",
    timestamp: "2026-04-18T12:00:00Z",
    scope: "lobby",
    ...overrides,
  };
}

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

  it("surfaces an incoming whisper on its own pink FAB chip", () => {
    // Dock stays CLOSED (no fab click). Without this, an incoming whisper is
    // invisible until the dock is opened — the chip must reflect it.
    render(<ChatDock variant="lobby" />);
    expect(screen.queryByTestId("lobby-chat-whisper-unread")).not.toBeInTheDocument();

    act(() => {
      useChatStore.getState().appendWhisper(whisper({ message: "psst" }), 1);
    });

    // Numeric count only. `textContent` is asserted whole, not with a substring
    // matcher: that pins the chip to EXACTLY the number, so any extra node
    // smuggled in beside it (a preview, a label) fails here.
    const badge = screen.getByTestId("lobby-chat-whisper-unread");
    expect(badge.textContent).toBe("1");
    expect(screen.queryByText("psst")).not.toBeInTheDocument();
  });

  it("renders only the brass chip when the unread is public-channel only", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      useChatStore.getState().appendLobby(lobbyMessage({ message: "m1" }));
      useChatStore.getState().appendLobby(lobbyMessage({ message: "m2" }));
    });

    expect(screen.getByTestId("lobby-chat-unread")).toHaveTextContent("2");
    expect(screen.queryByTestId("lobby-chat-whisper-unread")).not.toBeInTheDocument();
  });

  it("renders only the pink chip when the unread is whispers only", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      useChatStore.getState().appendWhisper(whisper(), 1);
    });

    expect(screen.getByTestId("lobby-chat-whisper-unread")).toHaveTextContent("1");
    // The brass chip counts the PUBLIC channel only — a whisper must not inflate
    // it (the pre-split badge summed both, hiding the whisper in public noise).
    expect(screen.queryByTestId("lobby-chat-unread")).not.toBeInTheDocument();
  });

  it("raises no pink chip for a whisper I sent myself", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      // Own-echo: the server delivers my own whisper back to me, keyed to the
      // recipient's thread. It must never read as something waiting for me.
      useChatStore
        .getState()
        .appendWhisper(
          whisper({ fromUserId: 1, fromUsername: "alice", toUserId: 2, toUsername: "bob" }),
          1,
        );
    });

    expect(screen.queryByTestId("lobby-chat-whisper-unread")).not.toBeInTheDocument();
    expect(screen.queryByTestId("lobby-chat-unread")).not.toBeInTheDocument();
  });

  it("announces BOTH counts on the FAB's accessible name, public first", () => {
    // An sr-only span nested in the button would be swallowed by the button's
    // own aria-label, so the counts have to be part of that label. Naming only
    // the whisper count would leave a screen-reader user hearing the same name
    // whether 0 or 12 public messages were waiting.
    render(<ChatDock variant="lobby" />);
    expect(screen.getByTestId("lobby-chat-fab")).toHaveAccessibleName("Open lobby chat");

    act(() => {
      useChatStore.getState().appendWhisper(whisper(), 1);
      useChatStore.getState().appendWhisper(whisper({ message: "again" }), 1);
    });
    expect(screen.getByTestId("lobby-chat-fab")).toHaveAccessibleName(
      "Open lobby chat, Unread whispers: 2",
    );

    act(() => {
      useChatStore.getState().appendLobby(lobbyMessage({ message: "m1" }));
    });
    expect(screen.getByTestId("lobby-chat-fab")).toHaveAccessibleName(
      "Open lobby chat, Unread messages: 1, Unread whispers: 2",
    );
  });

  it("announces the public count with no whisper pending", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      useChatStore.getState().appendLobby(lobbyMessage({ message: "m1" }));
      useChatStore.getState().appendLobby(lobbyMessage({ message: "m2" }));
    });

    expect(screen.getByTestId("lobby-chat-fab")).toHaveAccessibleName(
      "Open lobby chat, Unread messages: 2",
    );
  });

  it("keeps the two chips independent: brass = public, pink = whispers across threads", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      useChatStore.getState().appendLobby(lobbyMessage({ message: "m1" }));
      useChatStore.getState().appendLobby(lobbyMessage({ message: "m2" }));
      // bob(1) + carol(2) → pink sums to 3 across both threads.
      useChatStore.getState().appendWhisper(whisper({ message: "b1" }), 1);
      useChatStore
        .getState()
        .appendWhisper(whisper({ fromUserId: 3, fromUsername: "carol", message: "c1" }), 1);
      useChatStore
        .getState()
        .appendWhisper(whisper({ fromUserId: 3, fromUsername: "carol", message: "c2" }), 1);
    });

    expect(screen.getByTestId("lobby-chat-unread")).toHaveTextContent("2");
    expect(screen.getByTestId("lobby-chat-whisper-unread")).toHaveTextContent("3");
  });

  it("caps the pink whisper chip at 99+", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      for (let i = 0; i < 120; i++) {
        useChatStore.getState().appendWhisper(whisper({ message: `w-${i}` }), 1);
      }
    });

    expect(screen.getByTestId("lobby-chat-whisper-unread")).toHaveTextContent("99+");
  });

  it("narrows both overflow chips to two characters when they are paired", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      for (let i = 0; i < 120; i++) {
        useChatStore.getState().appendLobby(lobbyMessage({ message: `m-${i}` }));
        useChatStore.getState().appendWhisper(whisper({ message: `w-${i}` }), 1);
      }
    });

    // Two "99+" chips overlapped by 6px overhang the 56px FAB, so the paired
    // overflow form is two characters wide. The exact totals still reach a
    // screen reader through the button's name.
    expect(screen.getByTestId("lobby-chat-unread").textContent).toBe("9+");
    expect(screen.getByTestId("lobby-chat-whisper-unread").textContent).toBe("9+");
    expect(screen.getByTestId("lobby-chat-fab")).toHaveAccessibleName(
      "Open lobby chat, Unread messages: 120, Unread whispers: 120",
    );
  });

  it("keeps sub-100 counts exact when the chips are paired", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      for (let i = 0; i < 12; i++) {
        useChatStore.getState().appendLobby(lobbyMessage({ message: `m-${i}` }));
      }
      useChatStore.getState().appendWhisper(whisper(), 1);
    });

    // The paired narrowing is an OVERFLOW form only — it must not clamp a
    // two-digit count that fits.
    expect(screen.getByTestId("lobby-chat-unread").textContent).toBe("12");
    expect(screen.getByTestId("lobby-chat-whisper-unread").textContent).toBe("1");
  });

  it("stacks the two chips in one anchored container, pink in front of brass", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      useChatStore.getState().appendLobby(lobbyMessage());
      useChatStore.getState().appendWhisper(whisper(), 1);
    });

    const brass = screen.getByTestId("lobby-chat-unread");
    const pink = screen.getByTestId("lobby-chat-whisper-unread");
    // ONE shrink-wrapped anchor holds both, so the pair's right edge stays put
    // however wide the numbers get — the whole point of dropping the two fixed
    // absolute offsets that buried the brass digit past one character.
    const anchor = brass.parentElement;
    expect(anchor).not.toBeNull();
    expect(pink.parentElement).toBe(anchor);
    expect(anchor?.className).toContain("absolute");
    expect(anchor?.className).toContain("-top-0.5");
    expect(anchor?.className).toContain("-right-0.5");
    // Brass first in DOM, pink second — pulled 6px over it and winning z-order,
    // because the whisper is the rarer, higher-signal event.
    expect(Array.from(anchor?.children ?? [])).toEqual([brass, pink]);
    expect(pink.className).toContain("-ml-1.5");
    expect(pink.className).toContain("z-10");
  });

  it("drops the overlap and z-order from a pink chip that stands alone", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      useChatStore.getState().appendWhisper(whisper(), 1);
    });

    // Lone chip sits on the container's own anchor, so a whisper-only FAB looks
    // exactly like the single-badge FAB always did.
    const pink = screen.getByTestId("lobby-chat-whisper-unread");
    expect(pink.className).not.toContain("-ml-1.5");
    expect(pink.className).not.toContain("z-10");
  });

  it("paints the FAB chip and the whisper tab badge from the same badge tokens", () => {
    render(<ChatDock variant="lobby" />);
    act(() => {
      useChatStore.getState().appendWhisper(whisper(), 1);
    });

    const chip = screen.getByTestId("lobby-chat-whisper-unread");
    expect(chip.className).toContain("bg-(--whisper-badge)");
    expect(chip.className).toContain("text-(--whisper-badge-ink)");

    fireEvent.click(screen.getByTestId("lobby-chat-fab"));

    // The open dock's per-friend badge draws from the SAME pair, so closed and
    // open states read as one system — and the felt skin gets dark ink instead
    // of the near-invisible white it carried on its bright pink fill.
    const badge = screen.getByTestId("lobby-chat-whisper-tab-bob-unread");
    expect(badge.className).toContain("bg-(--whisper-badge)");
    expect(badge.className).toContain("text-(--whisper-badge-ink)");
  });

  it("raises no peek bubble for an incoming whisper on the closed dock", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      useChatStore.getState().appendWhisper(whisper({ message: "meet me at the bar" }), 1);
    });

    // The peek is primary-channel ONLY. A whisper reaches the closed FAB as a
    // bare number and nothing else — no bubble, and above all no private text
    // on screen for whoever is looking over your shoulder.
    expect(screen.queryByTestId("lobby-chat-peek")).not.toBeInTheDocument();
    expect(screen.queryByText("meet me at the bar")).not.toBeInTheDocument();
    expect(screen.getByTestId("lobby-chat-whisper-unread").textContent).toBe("1");
  });

  it("does raise a peek bubble for an incoming public message", () => {
    render(<ChatDock variant="lobby" />);

    act(() => {
      useChatStore.getState().appendLobby(lobbyMessage({ message: "gg everyone" }));
    });

    // The contrast that makes the whisper assertion above meaningful: the public
    // channel DOES preview its text, so "no peek for a whisper" is pinning a
    // real distinction rather than a peek that never fires at all.
    const peek = screen.getByTestId("lobby-chat-peek");
    expect(peek).toHaveTextContent("gg everyone");
    expect(peek).toHaveTextContent("bob");
  });

  it("shows no FAB chip while the dock is open, only per-thread tab badges", () => {
    render(<ChatDock variant="lobby" />);
    act(() => {
      useChatStore.getState().appendWhisper(whisper(), 1);
      useChatStore.getState().appendWhisper(whisper({ fromUserId: 3, fromUsername: "carol" }), 1);
    });
    fireEvent.click(screen.getByTestId("lobby-chat-fab"));

    expect(screen.queryByTestId("lobby-chat-whisper-unread")).not.toBeInTheDocument();
    expect(screen.queryByTestId("lobby-chat-unread")).not.toBeInTheDocument();
    // Per-friend counts stay on the whisper tab strip. The active channel is
    // still `primary`, so neither tab badge is suppressed as "active".
    expect(screen.getByTestId("lobby-chat-whisper-tab-bob-unread")).toHaveTextContent("1");
    expect(screen.getByTestId("lobby-chat-whisper-tab-carol-unread")).toHaveTextContent("1");
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

    expect(screen.getByTestId("lobby-chat-whisper-unread")).toHaveTextContent("1");
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
