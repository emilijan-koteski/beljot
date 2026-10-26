import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({
  toast: { info: vi.fn(), success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

const listInvitableFriendsSpy = vi.fn();
const listInvitablePlayersSpy = vi.fn();
const inviteToRoomSpy = vi.fn();
vi.mock("@/shared/api/rooms", () => ({
  listInvitableFriends: (roomId: number) => listInvitableFriendsSpy(roomId),
  listInvitablePlayers: (roomId: number, search: string) => listInvitablePlayersSpy(roomId, search),
  inviteToRoom: (roomId: number, friendUserId: number) => inviteToRoomSpy(roomId, friendUserId),
}));

import { FetchError } from "@/shared/api/axiosClient";
import type { InvitableFriend } from "@/shared/types/apiTypes";

import { InviteFriendsDialog } from "./InviteFriendsDialog";

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <InviteFriendsDialog open roomId={7} onOpenChange={() => {}} />
    </QueryClientProvider>,
  );
}

const roster: InvitableFriend[] = [
  { userId: 200, username: "ana", avatarUrl: null, available: true, reason: "" },
  { userId: 201, username: "bob", avatarUrl: null, available: false, reason: "offline" },
  { userId: 202, username: "cvete", avatarUrl: null, available: false, reason: "in_match" },
  { userId: 203, username: "dime", avatarUrl: null, available: false, reason: "in_room" },
];

function row(userId: number) {
  return screen
    .getAllByTestId("invite-friend-row")
    .find((r) => r.getAttribute("data-user-id") === String(userId))!;
}

describe("InviteFriendsDialog", () => {
  beforeEach(() => {
    listInvitableFriendsSpy.mockReset();
    listInvitablePlayersSpy.mockReset();
    inviteToRoomSpy.mockReset();
  });

  it("opens on the Friends tab and does not load the lobby", async () => {
    listInvitableFriendsSpy.mockResolvedValue(roster);
    renderDialog();

    await waitFor(() => expect(screen.getAllByTestId("invite-friend-row")).toHaveLength(4));
    expect(screen.getByTestId("invite-tab-friends")).toHaveAttribute("aria-selected", "true");
    expect(screen.getByTestId("invite-tab-lobby")).toHaveAttribute("aria-selected", "false");
    expect(listInvitablePlayersSpy).not.toHaveBeenCalled();
  });

  it("lists every friend and disables the unavailable ones with a reason", async () => {
    listInvitableFriendsSpy.mockResolvedValue(roster);
    renderDialog();

    await waitFor(() => expect(screen.getAllByTestId("invite-friend-row")).toHaveLength(4));

    expect(within(row(200)).getByTestId("invite-friend-200")).toBeEnabled();
    expect(within(row(201)).getByTestId("invite-friend-201")).toBeDisabled();
    expect(within(row(202)).getByTestId("invite-friend-202")).toBeDisabled();
    expect(within(row(203)).getByTestId("invite-friend-203")).toBeDisabled();

    expect(screen.getByTestId("invite-friend-reason-201")).toHaveTextContent(/offline/i);
    expect(screen.getByTestId("invite-friend-reason-202")).toBeInTheDocument();
    expect(screen.getByTestId("invite-friend-reason-203")).toBeInTheDocument();
    expect(screen.queryByTestId("invite-friend-reason-200")).not.toBeInTheDocument();
  });

  it("sends the invite and marks the row invited", async () => {
    const user = userEvent.setup();
    listInvitableFriendsSpy.mockResolvedValue(roster);
    inviteToRoomSpy.mockResolvedValue(undefined);
    renderDialog();

    await waitFor(() => expect(screen.getByTestId("invite-friend-200")).toBeEnabled());
    await user.click(screen.getByTestId("invite-friend-200"));

    await waitFor(() => expect(inviteToRoomSpy).toHaveBeenCalledWith(7, 200));
    await waitFor(() => expect(screen.getByTestId("invite-friend-200")).toBeDisabled());
  });

  it("reports a stale FRIEND_NOT_AVAILABLE inline on that row", async () => {
    const user = userEvent.setup();
    listInvitableFriendsSpy.mockResolvedValue(roster);
    inviteToRoomSpy.mockRejectedValue(
      new FetchError(409, "FRIEND_NOT_AVAILABLE", "friend is not available"),
    );
    renderDialog();

    await waitFor(() => expect(screen.getByTestId("invite-friend-200")).toBeEnabled());
    await user.click(screen.getByTestId("invite-friend-200"));

    expect(await screen.findByTestId("invite-friend-reason-200")).toBeInTheDocument();
    expect(screen.getByTestId("invite-friend-200")).toBeEnabled();
  });

  it("disables every row when the room itself is full", async () => {
    listInvitableFriendsSpy.mockResolvedValue([
      { userId: 200, username: "ana", avatarUrl: null, available: false, reason: "room_full" },
    ] satisfies InvitableFriend[]);
    renderDialog();

    await waitFor(() => expect(screen.getByTestId("invite-friend-200")).toBeDisabled());
    expect(screen.getByTestId("invite-friend-reason-200")).toBeInTheDocument();
  });

  it("shows the empty state when the viewer has no friends", async () => {
    listInvitableFriendsSpy.mockResolvedValue([]);
    renderDialog();

    expect(await screen.findByTestId("invite-friends-empty")).toBeInTheDocument();
  });

  it("shows a load failure rather than an empty list", async () => {
    listInvitableFriendsSpy.mockRejectedValue(new FetchError(500, "INTERNAL_ERROR", "boom"));
    renderDialog();

    expect(await screen.findByTestId("invite-friends-error")).toBeInTheDocument();
    expect(screen.queryByTestId("invite-friends-empty")).not.toBeInTheDocument();
  });
});

const lobby: InvitableFriend[] = [
  { userId: 300, username: "marija", avatarUrl: null, available: true, reason: "" },
  { userId: 301, username: "petar", avatarUrl: null, available: true, reason: "" },
];

function lobbyRow(userId: number) {
  return screen
    .getAllByTestId("invite-player-row")
    .find((r) => r.getAttribute("data-user-id") === String(userId))!;
}

async function openLobbyTab(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByTestId("invite-tab-lobby"));
}

describe("InviteFriendsDialog Lobby tab", () => {
  beforeEach(() => {
    listInvitableFriendsSpy.mockReset();
    listInvitablePlayersSpy.mockReset();
    inviteToRoomSpy.mockReset();
    listInvitableFriendsSpy.mockResolvedValue([]);
  });

  it("lists idle lobby players when the Lobby tab is selected", async () => {
    const user = userEvent.setup();
    listInvitablePlayersSpy.mockResolvedValue(lobby);
    renderDialog();

    await openLobbyTab(user);

    await waitFor(() => expect(screen.getAllByTestId("invite-player-row")).toHaveLength(2));
    expect(screen.getByTestId("invite-tab-lobby")).toHaveAttribute("aria-selected", "true");
    expect(listInvitablePlayersSpy).toHaveBeenCalledWith(7, "");
    expect(within(lobbyRow(300)).getByTestId("invite-player-300")).toBeEnabled();
    expect(screen.queryByTestId("invite-friend-row")).not.toBeInTheDocument();
  });

  it("invites a lobby player exactly like a friend", async () => {
    const user = userEvent.setup();
    listInvitablePlayersSpy.mockResolvedValue(lobby);
    inviteToRoomSpy.mockResolvedValue(undefined);
    renderDialog();

    await openLobbyTab(user);
    await waitFor(() => expect(screen.getByTestId("invite-player-301")).toBeEnabled());
    await user.click(screen.getByTestId("invite-player-301"));

    await waitFor(() => expect(inviteToRoomSpy).toHaveBeenCalledWith(7, 301));
    await waitFor(() =>
      expect(screen.getByTestId("invite-player-301")).toHaveTextContent(/invited/i),
    );
  });

  it("shows an opt-out refusal inline on that row", async () => {
    const user = userEvent.setup();
    listInvitablePlayersSpy.mockResolvedValue(lobby);
    inviteToRoomSpy.mockRejectedValue(new FetchError(409, "NOT_FRIENDS", "not friends"));
    renderDialog();

    await openLobbyTab(user);
    await waitFor(() => expect(screen.getByTestId("invite-player-300")).toBeEnabled());
    await user.click(screen.getByTestId("invite-player-300"));

    expect(await screen.findByTestId("invite-player-reason-300")).toHaveTextContent(
      "This player only accepts invites from friends",
    );
  });

  it("debounces the search and sends the trimmed term", async () => {
    const user = userEvent.setup();
    listInvitablePlayersSpy.mockResolvedValue(lobby);
    renderDialog();

    await openLobbyTab(user);
    await waitFor(() => expect(listInvitablePlayersSpy).toHaveBeenCalledWith(7, ""));
    listInvitablePlayersSpy.mockResolvedValue([lobby[0]]);

    await user.type(screen.getByTestId("invite-lobby-search"), " mar ");

    await waitFor(() => expect(listInvitablePlayersSpy).toHaveBeenCalledWith(7, "mar"));
    expect(listInvitablePlayersSpy).not.toHaveBeenCalledWith(7, "m");
    await waitFor(() => expect(screen.getAllByTestId("invite-player-row")).toHaveLength(1));
  });

  it("shows the empty lobby state without a search", async () => {
    const user = userEvent.setup();
    listInvitablePlayersSpy.mockResolvedValue([]);
    renderDialog();

    await openLobbyTab(user);

    expect(await screen.findByTestId("invite-lobby-empty")).toHaveTextContent(
      "Nobody else is free in the lobby right now.",
    );
  });

  it("names the search term when nobody matches it", async () => {
    const user = userEvent.setup();
    listInvitablePlayersSpy.mockResolvedValue([]);
    renderDialog();

    await openLobbyTab(user);
    await user.type(screen.getByTestId("invite-lobby-search"), "zzz");

    await waitFor(() =>
      expect(screen.getByTestId("invite-lobby-empty")).toHaveTextContent(
        'No lobby players match "zzz"',
      ),
    );
  });

  it("shows a loading state while the lobby loads", async () => {
    const user = userEvent.setup();
    listInvitablePlayersSpy.mockReturnValue(new Promise(() => {}));
    renderDialog();

    await openLobbyTab(user);

    expect(await screen.findByTestId("invite-lobby-loading")).toBeInTheDocument();
  });

  it("shows a load failure rather than an empty lobby", async () => {
    const user = userEvent.setup();
    listInvitablePlayersSpy.mockRejectedValue(new FetchError(500, "INTERNAL_ERROR", "boom"));
    renderDialog();

    await openLobbyTab(user);

    expect(await screen.findByTestId("invite-lobby-error")).toBeInTheDocument();
    expect(screen.queryByTestId("invite-lobby-empty")).not.toBeInTheDocument();
  });

  it("reopens with a blank search and never queries the stale term", async () => {
    const user = userEvent.setup();
    listInvitablePlayersSpy.mockResolvedValue([]);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const dialog = (open: boolean) => (
      <QueryClientProvider client={queryClient}>
        <InviteFriendsDialog open={open} roomId={7} onOpenChange={() => {}} />
      </QueryClientProvider>
    );
    const { rerender } = render(dialog(true));

    await openLobbyTab(user);
    await user.type(screen.getByTestId("invite-lobby-search"), "mar");
    await waitFor(() =>
      expect(screen.getByTestId("invite-lobby-empty")).toHaveTextContent(
        'No lobby players match "mar"',
      ),
    );
    const callsBeforeReopen = listInvitablePlayersSpy.mock.calls.length;

    rerender(dialog(false));
    rerender(dialog(true));

    expect(screen.getByTestId("invite-lobby-search")).toHaveValue("");
    expect(screen.queryByText('No lobby players match "mar"')).not.toBeInTheDocument();
    await waitFor(() => expect(listInvitablePlayersSpy).toHaveBeenLastCalledWith(7, ""));
    const reopenCalls = listInvitablePlayersSpy.mock.calls.slice(callsBeforeReopen);
    expect(reopenCalls.length).toBeGreaterThan(0);
    expect(reopenCalls.every(([, term]) => term === "")).toBe(true);
    expect(screen.getByTestId("invite-lobby-empty")).toHaveTextContent(
      "Nobody else is free in the lobby right now.",
    );
  });

  it("disables every lobby row when the room is full", async () => {
    const user = userEvent.setup();
    listInvitablePlayersSpy.mockResolvedValue([
      { userId: 300, username: "marija", avatarUrl: null, available: false, reason: "room_full" },
    ] satisfies InvitableFriend[]);
    renderDialog();

    await openLobbyTab(user);

    await waitFor(() => expect(screen.getByTestId("invite-player-300")).toBeDisabled());
    expect(screen.getByTestId("invite-player-reason-300")).toHaveTextContent("This room is full");
  });
});
