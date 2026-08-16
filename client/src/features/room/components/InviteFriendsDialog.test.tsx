import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({
  toast: { info: vi.fn(), success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

const listInvitableFriendsSpy = vi.fn();
const inviteToRoomSpy = vi.fn();
vi.mock("@/shared/api/rooms", () => ({
  listInvitableFriends: (roomId: number) => listInvitableFriendsSpy(roomId),
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
  { userId: 200, username: "ana", available: true, reason: "" },
  { userId: 201, username: "bob", available: false, reason: "offline" },
  { userId: 202, username: "cvete", available: false, reason: "in_match" },
  { userId: 203, username: "dime", available: false, reason: "in_room" },
];

function row(userId: number) {
  return screen
    .getAllByTestId("invite-friend-row")
    .find((r) => r.getAttribute("data-user-id") === String(userId))!;
}

describe("InviteFriendsDialog", () => {
  beforeEach(() => {
    listInvitableFriendsSpy.mockReset();
    inviteToRoomSpy.mockReset();
  });

  // AC1: unavailable friends are LISTED, disabled, with a reason — never hidden.
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
    // The available friend needs no explanation.
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

  // Availability is recomputed server-side, so a row can go stale between render
  // and click. The failure belongs on the row that failed, not in a toast that
  // loses track of which friend it was about.
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
    // Still invitable — the player can retry once their friend frees up.
    expect(screen.getByTestId("invite-friend-200")).toBeEnabled();
  });

  it("disables every row when the room itself is full", async () => {
    listInvitableFriendsSpy.mockResolvedValue([
      { userId: 200, username: "ana", available: false, reason: "room_full" },
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
