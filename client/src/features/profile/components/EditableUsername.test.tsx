import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { useAuthStore } from "@/shared/stores/authStore";
import type { User } from "@/shared/types/apiTypes";
import { QueryWrapper } from "@/test-utils";

import { EditableUsername } from "./EditableUsername";

const mockUpdateUsername = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: vi.fn(),
  updateUsername: (...args: unknown[]) => mockUpdateUsername(...args),
}));

function userFixture(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    username: "oldname",
    email: "u@example.com",
    languagePreference: "en",
    walletBalance: 5000,
    loginStreakDays: 0,
    totalXp: 0,
    level: 0,
    createdAt: "2026-01-15T10:00:00Z",
    ...overrides,
  };
}

type Props = Parameters<typeof EditableUsername>[0];

function renderEditable(props: Partial<Props> = {}) {
  return render(
    <QueryWrapper>
      <EditableUsername username="oldname" userId={1} usernameChangedAt={null} {...props} />
    </QueryWrapper>,
  );
}

beforeEach(() => {
  mockUpdateUsername.mockReset();
  useAuthStore.setState({ token: "t", user: userFixture(), isLoading: false });
});

describe("EditableUsername", () => {
  it("shows the username with an edit button and opens the editor on click", async () => {
    const user = userEvent.setup();
    renderEditable();

    expect(screen.getByTestId("profile-username")).toHaveTextContent("oldname");

    await user.click(screen.getByTestId("profile-edit-username-button"));
    expect(screen.getByTestId("profile-username-input")).toBeInTheDocument();
    // Save is disabled while the draft still equals the current name.
    expect(screen.getByTestId("profile-username-save")).toBeDisabled();
  });

  it("saves a valid new username and updates the auth store", async () => {
    mockUpdateUsername.mockResolvedValueOnce({
      username: "newname",
      usernameChangedAt: "2026-07-10T00:00:00Z",
    });
    const user = userEvent.setup();
    renderEditable();

    await user.click(screen.getByTestId("profile-edit-username-button"));
    const input = screen.getByTestId("profile-username-input");
    await user.clear(input);
    await user.type(input, "newname");
    await user.click(screen.getByTestId("profile-username-save"));

    await waitFor(() =>
      expect(mockUpdateUsername).toHaveBeenCalledWith(1, { username: "newname" }),
    );
    await waitFor(() => expect(useAuthStore.getState().user?.username).toBe("newname"));
    expect(screen.queryByTestId("profile-username-input")).not.toBeInTheDocument();
  });

  it("saves on Enter", async () => {
    mockUpdateUsername.mockResolvedValueOnce({
      username: "entername",
      usernameChangedAt: "2026-07-10T00:00:00Z",
    });
    const user = userEvent.setup();
    renderEditable();

    await user.click(screen.getByTestId("profile-edit-username-button"));
    const input = screen.getByTestId("profile-username-input");
    await user.clear(input);
    await user.type(input, "entername{Enter}");

    await waitFor(() =>
      expect(mockUpdateUsername).toHaveBeenCalledWith(1, { username: "entername" }),
    );
  });

  it("cancels without calling the API", async () => {
    const user = userEvent.setup();
    renderEditable();

    await user.click(screen.getByTestId("profile-edit-username-button"));
    await user.clear(screen.getByTestId("profile-username-input"));
    await user.type(screen.getByTestId("profile-username-input"), "discarded");
    await user.click(screen.getByTestId("profile-username-cancel"));

    expect(screen.queryByTestId("profile-username-input")).not.toBeInTheDocument();
    expect(mockUpdateUsername).not.toHaveBeenCalled();
    expect(screen.getByTestId("profile-username")).toHaveTextContent("oldname");
  });

  it("shows an inline validation error and does not call the API for a too-short name", async () => {
    const user = userEvent.setup();
    renderEditable();

    await user.click(screen.getByTestId("profile-edit-username-button"));
    await user.clear(screen.getByTestId("profile-username-input"));
    await user.type(screen.getByTestId("profile-username-input"), "ab");
    await user.click(screen.getByTestId("profile-username-save"));

    expect(screen.getByTestId("profile-username-error")).toBeInTheDocument();
    expect(mockUpdateUsername).not.toHaveBeenCalled();
  });

  it("surfaces a server 'taken' error inline and stays in edit mode", async () => {
    mockUpdateUsername.mockRejectedValueOnce(
      new FetchError(409, "USERNAME_TAKEN", "username is already taken"),
    );
    const user = userEvent.setup();
    renderEditable();

    await user.click(screen.getByTestId("profile-edit-username-button"));
    await user.clear(screen.getByTestId("profile-username-input"));
    await user.type(screen.getByTestId("profile-username-input"), "takenname");
    await user.click(screen.getByTestId("profile-username-save"));

    await waitFor(() => expect(screen.getByTestId("profile-username-error")).toBeInTheDocument());
    // Still editing so the user can correct it.
    expect(screen.getByTestId("profile-username-input")).toBeInTheDocument();
  });

  it("disables editing and shows a hint while on cooldown", () => {
    const oneHourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString();
    renderEditable({ usernameChangedAt: oneHourAgo });

    expect(screen.getByTestId("profile-edit-username-button")).toBeDisabled();
    expect(screen.getByTestId("profile-username-cooldown")).toBeInTheDocument();
  });

  it("hides the edit affordance when there is no userId", () => {
    renderEditable({ userId: undefined });
    expect(screen.queryByTestId("profile-edit-username-button")).not.toBeInTheDocument();
  });
});
