import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import { QueryWrapper } from "@/test-utils";

import { AuthLayout } from "./AuthLayout";
import { ResetPasswordPage } from "./ResetPasswordPage";

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockResetPassword = vi.fn();
vi.mock("@/shared/api/auth", () => ({
  resetPassword: (...args: unknown[]) => mockResetPassword(...args),
  logout: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

import { toast } from "sonner";

function renderResetPage(entry = "/reset-password?token=tok123") {
  return render(
    <QueryWrapper>
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route element={<AuthLayout />}>
            <Route path="/reset-password" element={<ResetPasswordPage />} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryWrapper>,
  );
}

describe("ResetPasswordPage", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    await i18n.changeLanguage("en");
  });

  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("renders the password fields when a token is present", () => {
    renderResetPage();
    expect(screen.getByTestId("password-input")).toBeInTheDocument();
    expect(screen.getByTestId("confirm-input")).toBeInTheDocument();
    expect(screen.getByTestId("submit-button")).toBeInTheDocument();
  });

  it("shows the invalid-link state when the token is missing", () => {
    renderResetPage("/reset-password");
    expect(screen.getByTestId("request-new-link")).toHaveAttribute("href", "/forgot-password");
    expect(screen.queryByTestId("reset-form")).not.toBeInTheDocument();
  });

  it("shows a mismatch error when passwords differ", async () => {
    const user = userEvent.setup();
    renderResetPage();

    await user.type(screen.getByTestId("password-input"), "brandnewpass1");
    await user.type(screen.getByTestId("confirm-input"), "different99");
    await user.click(screen.getByTestId("submit-button"));

    expect(screen.getByTestId("confirm-error")).toHaveTextContent("Passwords don't match");
    expect(mockResetPassword).not.toHaveBeenCalled();
  });

  it("submits and redirects to /login on success", async () => {
    const user = userEvent.setup();
    mockResetPassword.mockResolvedValueOnce(undefined);

    renderResetPage();
    await user.type(screen.getByTestId("password-input"), "brandnewpass1");
    await user.type(screen.getByTestId("confirm-input"), "brandnewpass1");
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(mockResetPassword).toHaveBeenCalledWith({
        token: "tok123",
        password: "brandnewpass1",
      });
    });
    expect(toast.success).toHaveBeenCalled();
    expect(mockNavigate).toHaveBeenCalledWith("/login");
  });

  it("shows the invalid-link state when the server rejects the token", async () => {
    const user = userEvent.setup();
    mockResetPassword.mockRejectedValueOnce(
      new FetchError(400, "INVALID_RESET_TOKEN", "reset link is invalid or has expired"),
    );

    renderResetPage();
    await user.type(screen.getByTestId("password-input"), "brandnewpass1");
    await user.type(screen.getByTestId("confirm-input"), "brandnewpass1");
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(screen.getByTestId("request-new-link")).toBeInTheDocument();
    });
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
