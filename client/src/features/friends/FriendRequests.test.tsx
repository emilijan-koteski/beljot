import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { FriendRequests } from "@/features/friends/FriendRequests";
import { QueryWrapper } from "@/test-utils";

const mockList = vi.fn();
const mockAccept = vi.fn();
const mockDecline = vi.fn();

vi.mock("@/shared/api/friends", () => ({
  listFriendRequests: (...args: unknown[]) => mockList(...args),
  acceptFriendRequest: (...args: unknown[]) => mockAccept(...args),
  declineFriendRequest: (...args: unknown[]) => mockDecline(...args),
}));

function renderRequests() {
  render(
    <QueryWrapper>
      <FriendRequests />
    </QueryWrapper>,
  );
}

afterEach(() => vi.clearAllMocks());

describe("FriendRequests", () => {
  it("shows the localized empty state when there are no requests", async () => {
    mockList.mockResolvedValue([]);
    renderRequests();
    expect(await screen.findByTestId("friend-requests-empty")).toBeInTheDocument();
  });

  it("renders incoming requests and accepts one by its row id", async () => {
    const user = userEvent.setup();
    mockList.mockResolvedValue([
      { id: 5, fromUserId: 2, fromUsername: "bob", createdAt: "2026-01-01T00:00:00Z" },
    ]);
    mockAccept.mockResolvedValue(undefined);
    renderRequests();

    const row = await screen.findByTestId("friend-request-row");
    expect(row).toHaveTextContent("bob");

    await user.click(screen.getByTestId("friend-request-accept"));
    expect(mockAccept).toHaveBeenCalledWith(5);
  });

  it("declines a request by its row id", async () => {
    const user = userEvent.setup();
    mockList.mockResolvedValue([
      { id: 8, fromUserId: 3, fromUsername: "carol", createdAt: "2026-01-01T00:00:00Z" },
    ]);
    mockDecline.mockResolvedValue(undefined);
    renderRequests();

    await screen.findByTestId("friend-request-row");
    await user.click(screen.getByTestId("friend-request-decline"));
    expect(mockDecline).toHaveBeenCalledWith(8);
  });
});
