import "@/shared/i18n/i18n";

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FriendsDialog } from "@/features/friends/FriendsDialog";
import { FriendsPanel } from "@/features/friends/FriendsPanel";
import { FriendsSidebar } from "@/features/friends/FriendsSidebar";
import { useChatStore } from "@/shared/stores/chatStore";
import { TestProviders } from "@/test-utils";

const mockListFriends = vi.fn();
const mockListRequests = vi.fn();
const mockSearchUsers = vi.fn();
const mockNavigate = vi.fn();

vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return { ...actual, useNavigate: () => mockNavigate };
});

vi.mock("@/shared/api/friends", () => ({
  listFriends: (...args: unknown[]) => mockListFriends(...args),
  listFriendRequests: (...args: unknown[]) => mockListRequests(...args),
  acceptFriendRequest: vi.fn(),
  declineFriendRequest: vi.fn(),
}));

vi.mock("@/shared/api/users", () => ({
  searchUsers: (...args: unknown[]) => mockSearchUsers(...args),
}));

function renderPanel(onWhisper?: (username: string) => void) {
  render(
    <TestProviders>
      <FriendsPanel onWhisper={onWhisper} />
    </TestProviders>,
  );
}

beforeEach(() => {
  mockListRequests.mockResolvedValue([]);
  mockSearchUsers.mockResolvedValue([]);
});

afterEach(() => {
  vi.clearAllMocks();
  useChatStore.getState().clearWhispers();
});

describe("FriendsPanel", () => {
  it("shows the localized empty state when there are no friends", async () => {
    mockListFriends.mockResolvedValue([]);
    renderPanel();
    expect(await screen.findByTestId("friend-list-empty")).toBeInTheDocument();
  });

  it("lists online friends first and counts them", async () => {
    mockListFriends.mockResolvedValue([
      { id: 3, username: "carol", online: false, avatarUrl: null },
      { id: 2, username: "bob", online: true, avatarUrl: null },
    ]);
    renderPanel();

    const rows = await screen.findAllByTestId("friend-row");
    expect(rows.map((r) => r.getAttribute("data-user-id"))).toEqual(["2", "3"]);
    expect(screen.getByText("Friends · 1 online")).toBeInTheDocument();
    expect(within(rows[0]!).getByText("Online")).toBeInTheDocument();
    expect(within(rows[1]!).getByText("Offline")).toBeInTheDocument();
  });

  it("offers whisper only for online friends", async () => {
    mockListFriends.mockResolvedValue([
      { id: 2, username: "bob", online: true, avatarUrl: null },
      { id: 3, username: "carol", online: false, avatarUrl: null },
    ]);
    renderPanel();

    const rows = await screen.findAllByTestId("friend-row");
    expect(within(rows[0]!).getByTestId("friend-whisper")).toBeInTheDocument();
    expect(within(rows[1]!).queryByTestId("friend-whisper")).toBeNull();
  });

  it("opens a whisper thread with the friend, asks the dock to open and tells its host", async () => {
    const user = userEvent.setup();
    const onWhisper = vi.fn();
    mockListFriends.mockResolvedValue([{ id: 2, username: "bob", online: true, avatarUrl: null }]);
    renderPanel(onWhisper);

    await screen.findByTestId("friend-row");
    const before = useChatStore.getState().whisperOpenRequest;
    await user.click(screen.getByTestId("friend-whisper"));

    const state = useChatStore.getState();
    expect(state.whisperThreads.bob).toEqual([]);
    expect(state.activeChannel).toBe("whisper:bob");
    expect(state.whisperOpenRequest).toBe(before + 1);
    expect(onWhisper).toHaveBeenCalledWith("bob");
  });

  it("opens a friend's profile from their row", async () => {
    const user = userEvent.setup();
    mockListFriends.mockResolvedValue([{ id: 2, username: "bob", online: false, avatarUrl: null }]);
    renderPanel();

    await user.click(await screen.findByRole("button", { name: "View bob's profile" }));

    expect(mockNavigate).toHaveBeenCalledWith("/players/2");
  });

  it("shows pending requests above the friends", async () => {
    mockListFriends.mockResolvedValue([]);
    mockListRequests.mockResolvedValue([
      { id: 11, fromUserId: 8, fromUsername: "marko", avatarUrl: null, createdAt: "" },
    ]);
    renderPanel();

    const requests = await screen.findByTestId("friend-requests");
    expect(requests).toHaveTextContent("Requests · 1");
    expect(requests).toHaveTextContent("Wants to be friends");
  });

  it("swaps the friends for player results after two characters", async () => {
    const user = userEvent.setup();
    mockListFriends.mockResolvedValue([{ id: 2, username: "bob", online: true, avatarUrl: null }]);
    mockSearchUsers.mockResolvedValue([
      { id: 7, username: "alice", avatarUrl: null },
      { id: 9, username: "alina", avatarUrl: null },
    ]);
    renderPanel();
    await screen.findByTestId("friend-row");

    await user.type(screen.getByTestId("player-search"), "al");

    await waitFor(() => expect(mockSearchUsers).toHaveBeenCalledWith("al"));
    const results = await screen.findAllByTestId("player-search-result");
    expect(results).toHaveLength(2);
    expect(results[0]).toHaveTextContent("alice");
    expect(results[1]).toHaveTextContent("alina");
    expect(screen.queryByTestId("friend-row")).toBeNull();

    await user.click(results[0]!);
    expect(mockNavigate).toHaveBeenCalledWith("/players/7");
  });

  it("does not search for a single character", async () => {
    const user = userEvent.setup();
    mockListFriends.mockResolvedValue([]);
    renderPanel();

    await user.type(screen.getByTestId("player-search"), "a");
    await act(async () => {
      await new Promise((r) => setTimeout(r, 300));
    });

    expect(mockSearchUsers).not.toHaveBeenCalled();
    expect(screen.getByTestId("friend-list-empty")).toBeInTheDocument();
  });

  it("says when no player matches", async () => {
    const user = userEvent.setup();
    mockListFriends.mockResolvedValue([]);
    renderPanel();

    await user.type(screen.getByTestId("player-search"), "zz");

    expect(await screen.findByTestId("player-search-empty")).toHaveTextContent("zz");
  });
});

describe("FriendsDialog and FriendsSidebar", () => {
  it("titles the sheet with how many friends are online and closes it on whisper", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    mockListFriends.mockResolvedValue([
      { id: 2, username: "bob", online: true, avatarUrl: null },
      { id: 3, username: "carol", online: false, avatarUrl: null },
    ]);
    render(
      <TestProviders>
        <FriendsDialog open onClose={onClose} />
      </TestProviders>,
    );

    expect(await screen.findByText("1 of 2 online")).toBeInTheDocument();
    await user.click(await screen.findByTestId("friend-whisper"));
    expect(onClose).toHaveBeenCalled();
  });

  it("heads the sidebar with the online count", async () => {
    mockListFriends.mockResolvedValue([{ id: 2, username: "bob", online: true, avatarUrl: null }]);
    render(
      <TestProviders>
        <FriendsSidebar />
      </TestProviders>,
    );

    const sidebar = screen.getByTestId("friends-sidebar");
    await waitFor(() => expect(sidebar).toHaveTextContent("1 online"));
    expect(within(sidebar).getByTestId("friends-panel")).toBeInTheDocument();
  });
});
