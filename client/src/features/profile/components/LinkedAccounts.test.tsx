import "@/shared/i18n/i18n";

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import type { LinkedIdentity } from "@/shared/api/identities";
import { QueryWrapper } from "@/test-utils";

import { LinkedAccounts } from "./LinkedAccounts";

const mockGetIdentities = vi.fn();
const mockLinkIdentity = vi.fn();
const mockUnlinkIdentity = vi.fn();
vi.mock("@/shared/api/identities", () => ({
  getIdentities: (...args: unknown[]) => mockGetIdentities(...args),
  linkIdentity: (...args: unknown[]) => mockLinkIdentity(...args),
  unlinkIdentity: (...args: unknown[]) => mockUnlinkIdentity(...args),
}));

// Stub the third-party GIS button with a plain button that emits a credential
// on click, so the link flow is testable without loading Google's script.
vi.mock("@/features/auth/components/GoogleSignInButton", () => ({
  GoogleSignInButton: ({ onCredential }: { onCredential: (c: string) => void }) => (
    <button data-testid="mock-google-button" onClick={() => onCredential("test-credential")}>
      Google
    </button>
  ),
}));

const mockToast = { success: vi.fn(), error: vi.fn() };
vi.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => mockToast.success(...a),
    error: (...a: unknown[]) => mockToast.error(...a),
  },
}));

function googleIdentity(overrides: Partial<LinkedIdentity> = {}): LinkedIdentity {
  return {
    provider: "google",
    email: "player@gmail.com",
    createdAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function renderPanel(userId: number | undefined = 1) {
  return render(
    <QueryWrapper>
      <LinkedAccounts userId={userId} />
    </QueryWrapper>,
  );
}

describe("LinkedAccounts", () => {
  beforeEach(() => {
    mockGetIdentities.mockReset();
    mockLinkIdentity.mockReset();
    mockUnlinkIdentity.mockReset();
    mockToast.success.mockReset();
    mockToast.error.mockReset();
  });

  it("renders the not-linked state with the Google sign-in button", async () => {
    mockGetIdentities.mockResolvedValue({ hasPassword: true, identities: [] });
    renderPanel();

    await waitFor(() => {
      expect(screen.getByTestId("linked-account-google-status")).toHaveTextContent("Not linked");
    });
    expect(screen.getByTestId("mock-google-button")).toBeInTheDocument();
    expect(screen.queryByTestId("unlink-google-button")).not.toBeInTheDocument();
  });

  it("renders the linked state with the email and an enabled Unlink button", async () => {
    mockGetIdentities.mockResolvedValue({ hasPassword: true, identities: [googleIdentity()] });
    renderPanel();

    await waitFor(() => {
      expect(screen.getByTestId("unlink-google-button")).toBeInTheDocument();
    });
    expect(screen.getByTestId("linked-account-google-status")).toHaveTextContent(
      "player@gmail.com",
    );
    expect(screen.getByTestId("unlink-google-button")).not.toBeDisabled();
    expect(screen.queryByTestId("unlink-google-hint")).not.toBeInTheDocument();
    expect(screen.queryByTestId("mock-google-button")).not.toBeInTheDocument();
  });

  it("links a Google account and toasts on success", async () => {
    mockGetIdentities.mockResolvedValue({ hasPassword: true, identities: [] });
    mockLinkIdentity.mockResolvedValue(googleIdentity());
    renderPanel();

    await waitFor(() => expect(screen.getByTestId("mock-google-button")).toBeInTheDocument());
    fireEvent.click(screen.getByTestId("mock-google-button"));

    await waitFor(() =>
      expect(mockLinkIdentity).toHaveBeenCalledWith(1, "google", { credential: "test-credential" }),
    );
    await waitFor(() => expect(mockToast.success).toHaveBeenCalled());
  });

  it("toasts a specific error when the Google account is already linked elsewhere", async () => {
    mockGetIdentities.mockResolvedValue({ hasPassword: true, identities: [] });
    mockLinkIdentity.mockRejectedValue(new FetchError(409, "SSO_IDENTITY_IN_USE", "in use"));
    renderPanel();

    await waitFor(() => expect(screen.getByTestId("mock-google-button")).toBeInTheDocument());
    fireEvent.click(screen.getByTestId("mock-google-button"));

    await waitFor(() => expect(mockToast.error).toHaveBeenCalled());
  });

  it("opens the confirm dialog and unlinks on confirm", async () => {
    mockGetIdentities.mockResolvedValue({ hasPassword: true, identities: [googleIdentity()] });
    mockUnlinkIdentity.mockResolvedValue(undefined);
    renderPanel();

    await waitFor(() => expect(screen.getByTestId("unlink-google-button")).toBeInTheDocument());
    fireEvent.click(screen.getByTestId("unlink-google-button"));

    await waitFor(() => expect(screen.getByTestId("unlink-account-dialog")).toBeInTheDocument());
    fireEvent.click(screen.getByTestId("unlink-account-confirm"));

    await waitFor(() => expect(mockUnlinkIdentity).toHaveBeenCalledWith(1, "google"));
    await waitFor(() => expect(mockToast.success).toHaveBeenCalled());
  });

  it("disables Unlink with a hint when it is the account's only sign-in method", async () => {
    mockGetIdentities.mockResolvedValue({ hasPassword: false, identities: [googleIdentity()] });
    renderPanel();

    await waitFor(() => expect(screen.getByTestId("unlink-google-button")).toBeInTheDocument());
    expect(screen.getByTestId("unlink-google-button")).toBeDisabled();
    expect(screen.getByTestId("unlink-google-hint")).toBeInTheDocument();
  });

  it("enables Unlink for a passwordless account that has another linked identity", async () => {
    mockGetIdentities.mockResolvedValue({
      hasPassword: false,
      identities: [
        googleIdentity(),
        googleIdentity({ provider: "facebook", email: "player@fb.com" }),
      ],
    });
    renderPanel();

    await waitFor(() => expect(screen.getByTestId("unlink-google-button")).toBeInTheDocument());
    expect(screen.getByTestId("unlink-google-button")).not.toBeDisabled();
    expect(screen.queryByTestId("unlink-google-hint")).not.toBeInTheDocument();
  });

  it("shows an error message when the identities query fails", async () => {
    mockGetIdentities.mockRejectedValue(new Error("boom"));
    renderPanel();

    await waitFor(() => expect(screen.getByTestId("linked-accounts-error")).toBeInTheDocument());
  });
});
