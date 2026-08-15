import "@/shared/i18n/i18n";

import { render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { FriendList } from "@/features/friends/FriendList";
import { TestProviders } from "@/test-utils";

const mockListFriends = vi.fn();

vi.mock("@/shared/api/friends", () => ({
  listFriends: (...args: unknown[]) => mockListFriends(...args),
}));

function renderList() {
  render(
    <TestProviders>
      <FriendList />
    </TestProviders>,
  );
}

afterEach(() => vi.clearAllMocks());

describe("FriendList", () => {
  it("shows the localized empty state when there are no friends", async () => {
    mockListFriends.mockResolvedValue([]);
    renderList();
    expect(await screen.findByTestId("friend-list-empty")).toBeInTheDocument();
  });

  it("shows an Invite-to-Room affordance only for ONLINE friends", async () => {
    mockListFriends.mockResolvedValue([
      { id: 2, username: "bob", online: true },
      { id: 3, username: "carol", online: false },
    ]);
    renderList();

    const rows = await screen.findAllByTestId("friend-row");
    expect(rows).toHaveLength(2);

    const bobRow = rows.find((r) => r.getAttribute("data-user-id") === "2")!;
    const carolRow = rows.find((r) => r.getAttribute("data-user-id") === "3")!;

    // The online friend exposes the (Story 11.5-owned) invite hook; the offline
    // friend does not.
    expect(within(bobRow).getByTestId("friend-invite-room")).toBeInTheDocument();
    expect(within(carolRow).queryByTestId("friend-invite-room")).toBeNull();
  });
});
