import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { FriendRequests } from "@/features/friends/FriendRequests";
import { TestProviders } from "@/test-utils";

const mockList = vi.fn();
const mockAccept = vi.fn();
const mockDecline = vi.fn();
const mockNavigate = vi.fn();

vi.mock("@/shared/api/friends", () => ({
  listFriendRequests: (...args: unknown[]) => mockList(...args),
  acceptFriendRequest: (...args: unknown[]) => mockAccept(...args),
  declineFriendRequest: (...args: unknown[]) => mockDecline(...args),
}));

vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return { ...actual, useNavigate: () => mockNavigate };
});

// TestProviders (not QueryWrapper): rows link to the sender's public profile, so
// the section needs a router around it.
function renderRequests() {
  render(
    <TestProviders>
      <FriendRequests />
    </TestProviders>,
  );
}

afterEach(() => vi.clearAllMocks());

describe("FriendRequests", () => {
  // An empty inbox is the normal state, and it is not worth a card that says so:
  // the card renders only when it has requests, and the friend list beside it
  // takes the freed width.
  it("renders nothing at all when there are no requests", async () => {
    mockList.mockResolvedValue([]);
    renderRequests();

    await waitFor(() => expect(mockList).toHaveBeenCalled());
    expect(screen.queryByTestId("friend-requests")).toBeNull();
  });

  it("renders nothing while the inbox is still loading", () => {
    mockList.mockReturnValue(new Promise(() => {}));
    renderRequests();

    expect(screen.queryByTestId("friend-requests")).toBeNull();
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

  // Same affordance as a friend row: you look the sender up before deciding.
  it("opens the sender's public profile from their name", async () => {
    const user = userEvent.setup();
    mockList.mockResolvedValue([
      { id: 5, fromUserId: 42, fromUsername: "bob", createdAt: "2026-01-01T00:00:00Z" },
    ]);
    renderRequests();

    await screen.findByTestId("friend-request-row");
    await user.click(screen.getByTestId("friend-request-profile"));

    expect(mockNavigate).toHaveBeenCalledWith("/players/42");
    // Opening the profile must not answer the request.
    expect(mockAccept).not.toHaveBeenCalled();
    expect(mockDecline).not.toHaveBeenCalled();
  });
});
