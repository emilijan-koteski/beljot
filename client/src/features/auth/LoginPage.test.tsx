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
import { LoginPage } from "./LoginPage";

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockLogin = vi.fn();
vi.mock("@/shared/api/auth", () => ({
  login: (...args: unknown[]) => mockLogin(...args),
  logout: vi.fn(),
}));

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn() },
}));

import { toast } from "sonner";

function renderLoginPage() {
  return render(
    <QueryWrapper>
      <MemoryRouter initialEntries={["/login"]}>
        <Routes>
          <Route element={<AuthLayout />}>
            <Route path="/login" element={<LoginPage />} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryWrapper>,
  );
}

describe("LoginPage", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    await i18n.changeLanguage("en");
  });

  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("renders email and password fields with submit button", () => {
    renderLoginPage();

    expect(screen.getByTestId("email-input")).toBeInTheDocument();
    expect(screen.getByTestId("password-input")).toBeInTheDocument();
    expect(screen.getByTestId("submit-button")).toBeInTheDocument();
    expect(screen.getByTestId("login-title")).toHaveTextContent("Log in");
  });

  it("shows validation errors on blur for empty fields", async () => {
    const user = userEvent.setup();
    renderLoginPage();

    const emailInput = screen.getByTestId("email-input");
    const passwordInput = screen.getByTestId("password-input");

    await user.click(emailInput);
    await user.tab();

    expect(screen.getByTestId("email-error")).toHaveTextContent("Email is required");

    await user.click(passwordInput);
    await user.tab();

    expect(screen.getByTestId("password-error")).toHaveTextContent("Password is required");
  });

  it("navigates to /lobby on successful login", async () => {
    const user = userEvent.setup();
    mockLogin.mockResolvedValueOnce({
      token: "access-token",
      id: 1,
      username: "testuser",
      email: "test@example.com",
      languagePreference: "en",
      walletBalance: 5000,
      loginStreakDays: 1,
      createdAt: "2026-01-01T00:00:00Z",
    });

    renderLoginPage();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("password-input"), "password123");
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/lobby");
    });

    expect(useAuthStore.getState().token).toBe("access-token");
    expect(useAuthStore.getState().user?.username).toBe("testuser");
  });

  it("displays generic error for 401 response", async () => {
    const user = userEvent.setup();
    mockLogin.mockRejectedValueOnce(
      new FetchError(401, "INVALID_CREDENTIALS", "invalid email or password"),
    );

    renderLoginPage();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("password-input"), "wrongpassword");
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(screen.getByTestId("form-error")).toBeInTheDocument();
    });

    expect(screen.getByTestId("form-error")).toHaveTextContent("Invalid email or password");
  });

  it("shows toast for non-401 errors", async () => {
    const user = userEvent.setup();
    mockLogin.mockRejectedValueOnce(new FetchError(500, "INTERNAL_ERROR", "Something went wrong"));

    renderLoginPage();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("password-input"), "password123");
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled();
    });
  });

  it("disables submit button during loading", async () => {
    const user = userEvent.setup();
    let resolveLogin: (value: unknown) => void;
    mockLogin.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveLogin = resolve;
      }),
    );

    renderLoginPage();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("password-input"), "password123");
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(screen.getByTestId("submit-button")).toBeDisabled();
    });

    resolveLogin!({
      token: "t",
      id: 1,
      username: "u",
      email: "e@e.com",
      languagePreference: "en",
      walletBalance: 5000,
      loginStreakDays: 1,
      createdAt: "2026-01-01",
    });

    await waitFor(() => {
      expect(screen.getByTestId("submit-button")).not.toBeDisabled();
    });
  });

  it("toggles password visibility", async () => {
    const user = userEvent.setup();
    renderLoginPage();

    const passwordInput = screen.getByTestId("password-input");
    expect(passwordInput).toHaveAttribute("type", "password");

    await user.click(screen.getByTestId("password-toggle"));
    expect(passwordInput).toHaveAttribute("type", "text");

    await user.click(screen.getByTestId("password-toggle"));
    expect(passwordInput).toHaveAttribute("type", "password");
  });

  it("has link to register page", () => {
    renderLoginPage();

    const registerLink = screen.getByTestId("register-link");
    expect(registerLink).toBeInTheDocument();
    expect(registerLink).toHaveAttribute("href", "/register");
  });

  it("renders the pre-auth language selector", () => {
    renderLoginPage();
    expect(screen.getByTestId("auth-language-selector")).toBeInTheDocument();
  });

  describe("post-login language reconciliation", () => {
    function loginResponse(languagePreference: string) {
      return {
        token: "access-token",
        id: 42,
        username: "testuser",
        email: "test@example.com",
        languagePreference,
        walletBalance: 5000,
        loginStreakDays: 1,
        createdAt: "2026-01-01T00:00:00Z",
      };
    }

    async function submitLogin(user: ReturnType<typeof userEvent.setup>) {
      await user.type(screen.getByTestId("email-input"), "test@example.com");
      await user.type(screen.getByTestId("password-input"), "password123");
      await user.click(screen.getByTestId("submit-button"));
    }

    it("fires PATCH /preferences when the picked language differs from the stored one", async () => {
      await i18n.changeLanguage("sr");
      mockLogin.mockResolvedValueOnce(loginResponse("en"));
      mockUpdatePreferences.mockResolvedValueOnce({ languagePreference: "sr" });

      const user = userEvent.setup();
      renderLoginPage();
      await submitLogin(user);

      await waitFor(() => {
        expect(mockUpdatePreferences).toHaveBeenCalledWith(42, { languagePreference: "sr" });
      });
      expect(useAuthStore.getState().user?.languagePreference).toBe("sr");
      expect(mockNavigate).toHaveBeenCalledWith("/lobby");
    });

    it("does not fire PATCH when the picked language matches the stored one", async () => {
      await i18n.changeLanguage("en");
      mockLogin.mockResolvedValueOnce(loginResponse("en"));

      const user = userEvent.setup();
      renderLoginPage();
      await submitLogin(user);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith("/lobby");
      });
      expect(mockUpdatePreferences).not.toHaveBeenCalled();
      expect(useAuthStore.getState().user?.languagePreference).toBe("en");
    });

    it("reverts the auth-store preference but keeps the UI language on PATCH failure", async () => {
      await i18n.changeLanguage("mk");
      mockLogin.mockResolvedValueOnce(loginResponse("en"));
      mockUpdatePreferences.mockRejectedValueOnce(new Error("boom"));

      const user = userEvent.setup();
      renderLoginPage();
      await submitLogin(user);

      await waitFor(() => {
        expect(mockUpdatePreferences).toHaveBeenCalled();
      });
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith("/lobby");
      });
      expect(useAuthStore.getState().user?.languagePreference).toBe("en");
      expect(i18n.language).toBe("mk");
      // Failure is silent — no toast.
      expect(toast.error).not.toHaveBeenCalled();
    });
  });
});
