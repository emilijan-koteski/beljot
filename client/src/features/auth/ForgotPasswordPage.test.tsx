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
import { ForgotPasswordPage } from "./ForgotPasswordPage";

const mockForgotPassword = vi.fn();
vi.mock("@/shared/api/auth", () => ({
  forgotPassword: (...args: unknown[]) => mockForgotPassword(...args),
  logout: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

import { toast } from "sonner";

function renderForgotPage() {
  return render(
    <QueryWrapper>
      <MemoryRouter initialEntries={["/forgot-password"]}>
        <Routes>
          <Route element={<AuthLayout />}>
            <Route path="/forgot-password" element={<ForgotPasswordPage />} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryWrapper>,
  );
}

describe("ForgotPasswordPage", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    await i18n.changeLanguage("en");
  });

  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("renders the email field and submit button", () => {
    renderForgotPage();
    expect(screen.getByTestId("email-input")).toBeInTheDocument();
    expect(screen.getByTestId("submit-button")).toBeInTheDocument();
    expect(screen.getByTestId("login-link")).toHaveAttribute("href", "/login");
  });

  it("shows a validation error for an invalid email", async () => {
    const user = userEvent.setup();
    renderForgotPage();

    await user.type(screen.getByTestId("email-input"), "not-an-email");
    await user.tab();

    expect(screen.getByTestId("email-error")).toHaveTextContent("Enter a valid email address");
    expect(mockForgotPassword).not.toHaveBeenCalled();
  });

  it("shows the generic success state after submitting a valid email", async () => {
    const user = userEvent.setup();
    mockForgotPassword.mockResolvedValueOnce(undefined);

    renderForgotPage();
    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(screen.getByTestId("forgot-success")).toBeInTheDocument();
    });
    expect(mockForgotPassword).toHaveBeenCalledWith({ email: "test@example.com" });
    // The form is gone — replaced by the confirmation state.
    expect(screen.queryByTestId("forgot-form")).not.toBeInTheDocument();
  });

  it("shows a toast and stays on the form when the request fails", async () => {
    const user = userEvent.setup();
    mockForgotPassword.mockRejectedValueOnce(new FetchError(0, "NETWORK_ERROR", "Network error"));

    renderForgotPage();
    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled();
    });
    expect(screen.queryByTestId("forgot-success")).not.toBeInTheDocument();
    expect(screen.getByTestId("forgot-form")).toBeInTheDocument();
  });
});
