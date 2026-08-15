import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { FriendButton } from "@/features/profile/components/FriendButton";
import { useAuthStore } from "@/shared/stores/authStore";
import type { FriendshipState } from "@/shared/types/apiTypes";
import { makeUser, QueryWrapper } from "@/test-utils";

const mockGetStatus = vi.fn();
const mockSend = vi.fn();
const mockAccept = vi.fn();
const mockDecline = vi.fn();

vi.mock("@/shared/api/friends", () => ({
  getFriendshipStatus: (...args: unknown[]) => mockGetStatus(...args),
  sendFriendRequest: (...args: unknown[]) => mockSend(...args),
  acceptFriendRequest: (...args: unknown[]) => mockAccept(...args),
  declineFriendRequest: (...args: unknown[]) => mockDecline(...args),
}));

function renderButton(userId = 7) {
  render(
    <QueryWrapper>
      <FriendButton userId={userId} />
    </QueryWrapper>,
  );
}

function statusOf(state: FriendshipState, requestId: number | null = null) {
  mockGetStatus.mockResolvedValue({ status: state, requestId });
}

afterEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ user: null });
});

describe("FriendButton", () => {
  it("shows Add Friend for 'none' and sends a request on click", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    statusOf("none");
    mockSend.mockResolvedValue(undefined);
    renderButton(7);

    const btn = await screen.findByTestId("friend-button-add");
    await user.click(btn);
    expect(mockSend).toHaveBeenCalledWith(7);
  });

  it("shows a disabled 'Request sent' for pending_outgoing", async () => {
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    statusOf("pending_outgoing", 42);
    renderButton(7);

    const btn = await screen.findByTestId("friend-button-pending");
    expect(btn).toBeDisabled();
  });

  it("shows Accept for pending_incoming and accepts the request row on click", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    statusOf("pending_incoming", 42);
    mockAccept.mockResolvedValue(undefined);
    renderButton(7);

    const btn = await screen.findByTestId("friend-button-accept");
    await user.click(btn);
    expect(mockAccept).toHaveBeenCalledWith(42);
  });

  it("shows a disabled 'Friends' affordance when already friends", async () => {
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    statusOf("friends", 42);
    renderButton(7);

    const btn = await screen.findByTestId("friend-button-friends");
    expect(btn).toBeDisabled();
  });

  it("renders nothing on the viewer's own profile", () => {
    useAuthStore.setState({ user: makeUser({ id: 7 }) });
    // A never-resolving status keeps the query pending so the self-guard's null
    // render is the only output (no post-render state update to wrap in act()).
    mockGetStatus.mockReturnValue(new Promise<never>(() => {}));
    renderButton(7);

    // The self-guard returns null; no button of any state is rendered.
    expect(screen.queryByTestId("friend-button-add")).toBeNull();
    expect(screen.queryByTestId("friend-button")).toBeNull();
  });
});
