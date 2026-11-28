import "@/shared/i18n/i18n";

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser, QueryWrapper } from "@/test-utils";

import { DeleteAccountSection } from "./DeleteAccountSection";

const mockLogoutApi = vi.fn();
vi.mock("@/shared/api/auth", () => ({
  logout: (...args: unknown[]) => mockLogoutApi(...args),
}));

const mockGetIdentities = vi.fn();
vi.mock("@/shared/api/identities", () => ({
  getIdentities: (...args: unknown[]) => mockGetIdentities(...args),
}));

const mockDeleteAccount = vi.fn();
vi.mock("@/shared/api/users", () => ({
  deleteAccount: (...args: unknown[]) => mockDeleteAccount(...args),
}));

const toastSuccess = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: vi.fn(),
  },
}));

function renderSection() {
  return render(
    <QueryWrapper>
      <MemoryRouter initialEntries={["/settings"]}>
        <Routes>
          <Route path="/settings" element={<DeleteAccountSection />} />
          <Route path="/" element={<p data-testid="landing">landing</p>} />
        </Routes>
      </MemoryRouter>
    </QueryWrapper>,
  );
}

async function openDialog(user: ReturnType<typeof userEvent.setup>) {
  const button = await screen.findByTestId("settings-delete-account");
  await waitFor(() => expect(button).toBeEnabled());
  await user.click(button);
  return screen.findByTestId("delete-account-dialog");
}

beforeEach(() => {
  mockLogoutApi.mockReset();
  mockDeleteAccount.mockReset().mockResolvedValue(undefined);
  mockGetIdentities.mockReset().mockResolvedValue({ hasPassword: true, identities: [] });
  toastSuccess.mockReset();
  useAuthStore.setState({
    token: "t",
    user: makeUser({ id: 7, username: "kiro", email: "kiro@example.com" }),
    isLoading: false,
  });
});

describe("DeleteAccountSection", () => {
  it("explains what is deleted and what stays before anything happens", async () => {
    const user = userEvent.setup();
    renderSection();

    const dialog = await openDialog(user);

    expect(within(dialog).getAllByRole("listitem")).toHaveLength(5);
    expect(dialog).toHaveTextContent(
      "Match results stay for the other players, without your name.",
    );
    expect(dialog).toHaveTextContent("cannot be undone");
    expect(within(dialog).getByTestId("delete-account-confirm")).toBeDisabled();
    expect(mockDeleteAccount).not.toHaveBeenCalled();
  });

  it("deletes a password account, ends the session and lands on the landing page", async () => {
    const user = userEvent.setup();
    renderSection();
    const dialog = await openDialog(user);

    await user.type(within(dialog).getByTestId("delete-account-input"), "secret-pw");
    await user.click(within(dialog).getByTestId("delete-account-confirm"));

    expect(await screen.findByTestId("landing")).toBeInTheDocument();
    expect(mockDeleteAccount).toHaveBeenCalledWith(7, { password: "secret-pw" });
    expect(useAuthStore.getState().user).toBeNull();
    expect(useAuthStore.getState().token).toBeNull();
    expect(mockLogoutApi).not.toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalledWith("Your account has been deleted.");
  });

  it("asks a Google-only account to type its username", async () => {
    mockGetIdentities.mockResolvedValue({
      hasPassword: false,
      identities: [
        { provider: "google", email: "kiro@gmail.com", createdAt: "2026-01-01T00:00:00Z" },
      ],
    });
    const user = userEvent.setup();
    renderSection();
    const dialog = await openDialog(user);

    expect(dialog).toHaveTextContent('Type your username "kiro" to confirm');
    const input = within(dialog).getByTestId("delete-account-input");
    expect(input).toHaveAttribute("type", "text");
    await user.type(input, "kir");
    expect(within(dialog).getByTestId("delete-account-confirm")).toBeDisabled();
    await user.type(input, "o");
    await user.click(within(dialog).getByTestId("delete-account-confirm"));

    await waitFor(() => expect(mockDeleteAccount).toHaveBeenCalledWith(7, { username: "kiro" }));
  });

  it("tells the player to leave the room first and keeps the session", async () => {
    mockDeleteAccount.mockRejectedValue(
      new FetchError(409, "ACCOUNT_DELETE_IN_ROOM", "leave your room"),
    );
    const user = userEvent.setup();
    renderSection();
    const dialog = await openDialog(user);

    await user.type(within(dialog).getByTestId("delete-account-input"), "secret-pw");
    await user.click(within(dialog).getByTestId("delete-account-confirm"));

    expect(await within(dialog).findByTestId("delete-account-error")).toHaveTextContent(
      "Leave your room or match first, then try again.",
    );
    expect(useAuthStore.getState().user).not.toBeNull();
  });

  it("shows a wrong password under the field", async () => {
    mockDeleteAccount.mockRejectedValue(
      new FetchError(400, "ACCOUNT_DELETE_CONFIRMATION_MISMATCH", "mismatch"),
    );
    const user = userEvent.setup();
    renderSection();
    const dialog = await openDialog(user);

    await user.type(within(dialog).getByTestId("delete-account-input"), "wrong-pw");
    await user.click(within(dialog).getByTestId("delete-account-confirm"));

    expect(await within(dialog).findByTestId("delete-account-error")).toHaveTextContent(
      "That password is not correct.",
    );
  });
});
