import "@/shared/i18n/i18n";

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FriendList } from "@/features/friends/FriendList";
import { useChatStore } from "@/shared/stores/chatStore";
import { TestProviders } from "@/test-utils";

const mockListFriends = vi.fn();
const mockListRequests = vi.fn();

// The card now contains the requests section too, so the friends API stub has to
// cover both queries. Requests default to empty — that section then draws
// nothing, which is what every test below expects unless it says otherwise.
vi.mock("@/shared/api/friends", () => ({
  listFriends: (...args: unknown[]) => mockListFriends(...args),
  listFriendRequests: (...args: unknown[]) => mockListRequests(...args),
  acceptFriendRequest: vi.fn(),
  declineFriendRequest: vi.fn(),
}));

// The card hosts player search behind its header icon; no test here types a
// query, so the stub only has to exist.
vi.mock("@/shared/api/users", () => ({
  searchUsers: vi.fn().mockResolvedValue([]),
}));

function renderList() {
  render(
    <TestProviders>
      <FriendList />
    </TestProviders>,
  );
}

beforeEach(() => {
  mockListRequests.mockResolvedValue([]);
});

afterEach(() => {
  vi.clearAllMocks();
  useChatStore.getState().clearWhispers();
});

describe("FriendList", () => {
  it("shows the localized empty state when there are no friends", async () => {
    mockListFriends.mockResolvedValue([]);
    renderList();
    expect(await screen.findByTestId("friend-list-empty")).toBeInTheDocument();
  });

  it("offers whisper only for ONLINE friends", async () => {
    mockListFriends.mockResolvedValue([
      { id: 2, username: "bob", online: true },
      { id: 3, username: "carol", online: false },
    ]);
    renderList();

    const rows = await screen.findAllByTestId("friend-row");
    expect(rows).toHaveLength(2);

    const bobRow = rows.find((r) => r.getAttribute("data-user-id") === "2")!;
    const carolRow = rows.find((r) => r.getAttribute("data-user-id") === "3")!;

    // Whispers are real-time: the server rejects one to an offline friend, so the
    // button is not offered there.
    expect(within(bobRow).getByTestId("friend-whisper")).toBeInTheDocument();
    expect(within(carolRow).queryByTestId("friend-whisper")).toBeNull();
  });

  it("shows the friend's initial rather than a generic person glyph", async () => {
    mockListFriends.mockResolvedValue([{ id: 2, username: "bob", online: false }]);
    renderList();

    const row = await screen.findByTestId("friend-row");
    expect(row).toHaveTextContent("B");
  });

  it("opens a whisper thread with the friend and asks the dock to open", async () => {
    const user = userEvent.setup();
    mockListFriends.mockResolvedValue([{ id: 2, username: "bob", online: true }]);
    renderList();

    await screen.findByTestId("friend-row");
    const before = useChatStore.getState().whisperOpenRequest;

    await user.click(screen.getByTestId("friend-whisper"));

    const state = useChatStore.getState();
    // A thread the pair has never used is seeded empty, so the dock's
    // activeThread guard resolves it instead of snapping back to primary.
    expect(state.whisperThreads.bob).toEqual([]);
    expect(state.activeChannel).toBe("whisper:bob");
    expect(state.whisperOpenRequest).toBe(before + 1);
  });

  // Player search lives in this card's header now — one icon, expanded on demand,
  // instead of a permanent full-width card holding an almost-always-empty field.
  it("expands player search from the header icon and collapses it again", async () => {
    const user = userEvent.setup();
    mockListFriends.mockResolvedValue([]);
    renderList();

    await screen.findByTestId("friend-list-empty");
    expect(screen.queryByTestId("player-search")).toBeNull();

    const toggle = screen.getByTestId("player-search-toggle");
    expect(toggle).toHaveAttribute("aria-expanded", "false");

    await user.click(toggle);

    expect(screen.getByTestId("player-search")).toHaveFocus();
    expect(screen.getByTestId("player-search-toggle")).toHaveAttribute("aria-expanded", "true");
    // The friend list is not replaced by the search — both stay readable.
    expect(screen.getByTestId("friend-list-empty")).toBeInTheDocument();

    await user.click(screen.getByTestId("player-search-toggle"));

    expect(screen.queryByTestId("player-search")).toBeNull();
  });

  it("collapses the search when Escape is pressed in the field", async () => {
    const user = userEvent.setup();
    mockListFriends.mockResolvedValue([]);
    renderList();

    await screen.findByTestId("friend-list-empty");
    await user.click(screen.getByTestId("player-search-toggle"));
    await user.type(screen.getByTestId("player-search"), "al{Escape}");

    expect(screen.queryByTestId("player-search")).toBeNull();
  });
});
