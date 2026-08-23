import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { FriendButton } from "@/features/profile/components/FriendButton";
import { useAuthStore } from "@/shared/stores/authStore";
import type { FriendshipState } from "@/shared/types/apiTypes";
import { expectActionBeforeStatusQuo, makeUser, QueryWrapper } from "@/test-utils";

const mockGetStatus = vi.fn();
const mockSend = vi.fn();
const mockAccept = vi.fn();
const mockDecline = vi.fn();
const mockRemove = vi.fn();

vi.mock("@/shared/api/friends", () => ({
  getFriendshipStatus: (...args: unknown[]) => mockGetStatus(...args),
  sendFriendRequest: (...args: unknown[]) => mockSend(...args),
  acceptFriendRequest: (...args: unknown[]) => mockAccept(...args),
  declineFriendRequest: (...args: unknown[]) => mockDecline(...args),
  removeFriend: (...args: unknown[]) => mockRemove(...args),
}));

function renderButton(userId = 7) {
  render(
    <QueryWrapper>
      <FriendButton userId={userId} username="bojan" />
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

  it("shows Decline for pending_incoming and declines the request row on click", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    statusOf("pending_incoming", 42);
    mockDecline.mockResolvedValue(undefined);
    renderButton(7);

    const btn = await screen.findByTestId("friend-button-decline");
    await user.click(btn);
    expect(mockDecline).toHaveBeenCalledWith(42);
  });

  it("removes the friend row after confirming the remove dialog", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    statusOf("friends", 42);
    mockRemove.mockResolvedValue(undefined);
    renderButton(7);

    // The remove affordance opens a confirm dialog — nothing fires yet.
    await user.click(await screen.findByTestId("friend-button-remove"));
    expect(await screen.findByTestId("remove-friend-dialog")).toBeInTheDocument();
    expect(mockRemove).not.toHaveBeenCalled();

    await user.click(screen.getByTestId("remove-friend-confirm"));
    expect(mockRemove).toHaveBeenCalledWith(42);
  });

  it("cancelling the remove dialog fires no request", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    statusOf("friends", 42);
    renderButton(7);

    await user.click(await screen.findByTestId("friend-button-remove"));
    // Footer convention: the destructive action leads, cancel trails.
    expectActionBeforeStatusQuo("remove-friend-confirm", "remove-friend-cancel");

    await user.click(screen.getByTestId("remove-friend-cancel"));

    expect(screen.queryByTestId("remove-friend-dialog")).toBeNull();
    expect(mockRemove).not.toHaveBeenCalled();
  });

  it("flips to Add friend after a successful remove", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    // "friends" on the first load, then "none" once the remove's onSuccess
    // invalidation refetches the status — proving the invalidation is live.
    mockGetStatus
      .mockResolvedValueOnce({ status: "friends", requestId: 42 })
      .mockResolvedValue({ status: "none", requestId: null });
    mockRemove.mockResolvedValue(undefined);
    renderButton(7);

    await user.click(await screen.findByTestId("friend-button-remove"));
    await user.click(screen.getByTestId("remove-friend-confirm"));

    expect(await screen.findByTestId("friend-button-add")).toBeInTheDocument();
  });

  it("locks the remove dialog while the request is in flight", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    statusOf("friends", 42);
    // A hanging remove keeps the mutation pending for the whole assertion.
    mockRemove.mockReturnValue(new Promise(() => {}));
    renderButton(7);

    await user.click(await screen.findByTestId("friend-button-remove"));
    await user.click(screen.getByTestId("remove-friend-confirm"));

    expect(await screen.findByText("Removing…")).toBeInTheDocument();
    expect(screen.getByTestId("remove-friend-confirm")).toBeDisabled();
    expect(screen.getByTestId("remove-friend-cancel")).toBeDisabled();
  });

  it("disables remove when the status carries no row id", async () => {
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    // A defensive impossibility ("friends" without a row id) must never offer a
    // confirm that would fire without an id.
    statusOf("friends", null);
    renderButton(7);

    const btn = await screen.findByTestId("friend-button-remove");
    expect(btn).toBeDisabled();
  });

  it("disables accept and decline when the status carries no row id", async () => {
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    statusOf("pending_incoming", null);
    renderButton(7);

    expect(await screen.findByTestId("friend-button-accept")).toBeDisabled();
    expect(screen.getByTestId("friend-button-decline")).toBeDisabled();
  });

  it("disables decline while accept is in flight", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: makeUser({ id: 99 }) });
    statusOf("pending_incoming", 42);
    // A hanging accept keeps the mutation pending — both actions target the
    // same request row, so decline must lock alongside it.
    mockAccept.mockReturnValue(new Promise(() => {}));
    renderButton(7);

    const accept = await screen.findByTestId("friend-button-accept");
    await user.click(accept);

    expect(accept).toBeDisabled();
    expect(screen.getByTestId("friend-button-decline")).toBeDisabled();
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
