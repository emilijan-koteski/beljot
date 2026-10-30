import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { resetInvitePreferenceRequestSequence } from "@/shared/lib/invitePreference";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import { InvitePrivacyPanel } from "./InvitePrivacyPanel";

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

describe("InvitePrivacyPanel", () => {
  beforeEach(() => {
    mockUpdatePreferences.mockReset();
    mockUpdatePreferences.mockResolvedValue({});
    resetInvitePreferenceRequestSequence();
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ id: 7, allowNonFriendInvites: true }),
      isLoading: false,
    });
  });

  it("shows the switch in the stored state, labelled by its row", () => {
    useAuthStore.setState({ user: makeUser({ id: 7, allowNonFriendInvites: false }) });
    render(<InvitePrivacyPanel />);

    expect(screen.getByTestId("profile-invites")).toHaveTextContent("Room invites");
    const toggle = screen.getByRole("switch", { name: "Invites from non-friends" });
    expect(toggle).toBe(screen.getByTestId("profile-allow-non-friend-invites"));
    expect(toggle).toHaveAttribute("aria-checked", "false");
  });

  it("reads a missing preference as ON", () => {
    useAuthStore.setState({
      user: { ...makeUser({ id: 7 }), allowNonFriendInvites: undefined as never },
    });
    render(<InvitePrivacyPanel />);

    expect(screen.getByTestId("profile-allow-non-friend-invites")).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("turns the switch off optimistically and PATCHes only its own field", async () => {
    const user = userEvent.setup();
    render(<InvitePrivacyPanel />);

    await user.click(screen.getByTestId("profile-allow-non-friend-invites"));

    expect(useAuthStore.getState().user?.allowNonFriendInvites).toBe(false);
    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { allowNonFriendInvites: false });
    expect(screen.getByTestId("profile-allow-non-friend-invites")).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  it("toggles from its row label too", async () => {
    const user = userEvent.setup();
    render(<InvitePrivacyPanel />);

    await user.click(screen.getByText("Invites from non-friends"));

    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { allowNonFriendInvites: false });
  });

  it("reverts the switch when the PATCH fails", async () => {
    const user = userEvent.setup();
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
    render(<InvitePrivacyPanel />);

    await user.click(screen.getByTestId("profile-allow-non-friend-invites"));

    await waitFor(() => {
      expect(useAuthStore.getState().user?.allowNonFriendInvites).toBe(true);
    });
    expect(screen.getByTestId("profile-allow-non-friend-invites")).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("keeps a later toggle when an earlier PATCH fails", async () => {
    const user = userEvent.setup();
    let rejectFirst: (err: Error) => void = () => {};
    mockUpdatePreferences.mockImplementationOnce(
      () =>
        new Promise((_, reject) => {
          rejectFirst = reject;
        }),
    );
    render(<InvitePrivacyPanel />);

    await user.click(screen.getByTestId("profile-allow-non-friend-invites"));
    await user.click(screen.getByTestId("profile-allow-non-friend-invites"));
    rejectFirst(new Error("offline"));

    await waitFor(() => expect(mockUpdatePreferences).toHaveBeenCalledTimes(2));
    expect(useAuthStore.getState().user?.allowNonFriendInvites).toBe(true);
    expect(mockUpdatePreferences).toHaveBeenLastCalledWith(7, { allowNonFriendInvites: true });
  });

  it("ignores a stale failure after two later toggles", async () => {
    const user = userEvent.setup();
    let rejectFirst: (err: Error) => void = () => {};
    mockUpdatePreferences.mockImplementationOnce(
      () =>
        new Promise((_, reject) => {
          rejectFirst = reject;
        }),
    );
    render(<InvitePrivacyPanel />);
    const toggle = screen.getByTestId("profile-allow-non-friend-invites");

    await user.click(toggle);
    await user.click(toggle);
    await user.click(toggle);
    await waitFor(() => expect(mockUpdatePreferences).toHaveBeenCalledTimes(3));
    rejectFirst(new Error("offline"));
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(useAuthStore.getState().user?.allowNonFriendInvites).toBe(false);
    expect(toggle).toHaveAttribute("aria-checked", "false");
  });
});
