import "@/shared/i18n/i18n";

import { act, render, screen, waitFor } from "@testing-library/react";
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
const mockSsoLogin = vi.fn();
const mockSsoLink = vi.fn();
vi.mock("@/shared/api/auth", () => ({
  login: (...args: unknown[]) => mockLogin(...args),
  ssoLogin: (...args: unknown[]) => mockSsoLogin(...args),
  ssoLink: (...args: unknown[]) => mockSsoLink(...args),
  logout: vi.fn(),
}));

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

// GIS never loads in tests — the mock captures the onCredential callback so
// tests can hand the page a fake Google credential.
const mockRenderGoogleButton = vi.fn();
let mockClientId = "test-client-id";
vi.mock("@/shared/lib/googleIdentity", () => ({
  getGoogleClientId: () => mockClientId,
  renderGoogleButton: (...args: unknown[]) => mockRenderGoogleButton(...args),
  decodeCredentialEmail: () => "match@example.com",
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

let capturedOnCredential: ((credential: string) => void) | undefined;

describe("LoginPage", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    await i18n.changeLanguage("en");
    mockClientId = "test-client-id";
    capturedOnCredential = undefined;
    mockRenderGoogleButton.mockImplementation((options: unknown) => {
      capturedOnCredential = (options as { onCredential: (c: string) => void }).onCredential;
      return Promise.resolve(true);
    });
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
      totalXp: 0,
      level: 0,
      createdAt: "2026-01-01T00:00:00Z",
    });

    renderLoginPage();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("password-input"), "password123");
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
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
      totalXp: 0,
      level: 0,
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
        totalXp: 0,
        level: 0,
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
      expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
    });

    it("does not fire PATCH when the picked language matches the stored one", async () => {
      await i18n.changeLanguage("en");
      mockLogin.mockResolvedValueOnce(loginResponse("en"));

      const user = userEvent.setup();
      renderLoginPage();
      await submitLogin(user);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
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
        expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
      });
      expect(useAuthStore.getState().user?.languagePreference).toBe("en");
      expect(i18n.language).toBe("mk");
      // Failure is silent — no toast.
      expect(toast.error).not.toHaveBeenCalled();
    });
  });

  describe("Google SSO", () => {
    function ssoResponse() {
      return {
        token: "sso-token",
        id: 7,
        username: "googler",
        email: "match@example.com",
        languagePreference: "en",
        walletBalance: 5000,
        loginStreakDays: 0,
        totalXp: 0,
        level: 0,
        createdAt: "2026-01-01T00:00:00Z",
      };
    }

    async function emitCredential(credential: string) {
      await waitFor(() => {
        expect(capturedOnCredential).toBeDefined();
      });
      await act(async () => {
        capturedOnCredential!(credential);
      });
    }

    it("renders the Google button slot when a client id is configured", async () => {
      renderLoginPage();

      expect(screen.getByTestId("google-signin-button")).toBeInTheDocument();
      expect(screen.getByTestId("sso-divider")).toBeInTheDocument();
      await waitFor(() => {
        expect(mockRenderGoogleButton).toHaveBeenCalled();
      });
    });

    it("hides the Google button when no client id is configured", () => {
      mockClientId = "";
      renderLoginPage();

      expect(screen.queryByTestId("google-signin-button")).not.toBeInTheDocument();
      expect(mockRenderGoogleButton).not.toHaveBeenCalled();
    });

    it("stores the session and navigates to /lobby on SSO success", async () => {
      mockSsoLogin.mockResolvedValueOnce(ssoResponse());
      renderLoginPage();

      await emitCredential("google-credential");

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
      });
      expect(mockSsoLogin).toHaveBeenCalledWith("google", { credential: "google-credential" });
      expect(useAuthStore.getState().token).toBe("sso-token");
      expect(useAuthStore.getState().user?.username).toBe("googler");
    });

    it("opens the link dialog with the matched email when linking is required", async () => {
      mockSsoLogin.mockRejectedValueOnce(
        new FetchError(409, "SSO_LINK_REQUIRED", "account exists"),
      );
      renderLoginPage();

      await emitCredential("google-credential");

      await waitFor(() => {
        expect(screen.getByTestId("link-account-dialog")).toBeInTheDocument();
      });
      expect(screen.getByTestId("link-account-email")).toHaveTextContent("match@example.com");
    });

    it("links the account and logs in when the password is correct", async () => {
      const user = userEvent.setup();
      mockSsoLogin.mockRejectedValueOnce(
        new FetchError(409, "SSO_LINK_REQUIRED", "account exists"),
      );
      mockSsoLink.mockResolvedValueOnce(ssoResponse());
      renderLoginPage();

      await emitCredential("google-credential");
      await waitFor(() => {
        expect(screen.getByTestId("link-account-dialog")).toBeInTheDocument();
      });

      await user.type(screen.getByTestId("link-account-password-input"), "password123");
      await user.click(screen.getByTestId("link-account-submit"));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
      });
      expect(mockSsoLink).toHaveBeenCalledWith("google", {
        credential: "google-credential",
        password: "password123",
      });
      expect(useAuthStore.getState().token).toBe("sso-token");
    });

    it("shows an inline error and keeps the dialog open on a wrong password", async () => {
      const user = userEvent.setup();
      mockSsoLogin.mockRejectedValueOnce(
        new FetchError(409, "SSO_LINK_REQUIRED", "account exists"),
      );
      mockSsoLink.mockRejectedValueOnce(
        new FetchError(401, "INVALID_CREDENTIALS", "invalid email or password"),
      );
      renderLoginPage();

      await emitCredential("google-credential");
      await waitFor(() => {
        expect(screen.getByTestId("link-account-dialog")).toBeInTheDocument();
      });

      await user.type(screen.getByTestId("link-account-password-input"), "wrongpassword");
      await user.click(screen.getByTestId("link-account-submit"));

      await waitFor(() => {
        expect(screen.getByTestId("link-account-error")).toHaveTextContent(
          "Incorrect password. Try again.",
        );
      });
      expect(screen.getByTestId("link-account-dialog")).toBeInTheDocument();
      expect(mockNavigate).not.toHaveBeenCalled();
      expect(useAuthStore.getState().token).toBeNull();
    });

    it("shows a toast when SSO fails with a non-link error", async () => {
      mockSsoLogin.mockRejectedValueOnce(
        new FetchError(401, "SSO_INVALID_CREDENTIAL", "sign-in credential is invalid or expired"),
      );
      renderLoginPage();

      await emitCredential("google-credential");

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalled();
      });
      expect(screen.queryByTestId("link-account-dialog")).not.toBeInTheDocument();
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it("renders the ToS/privacy small-print under the Google button", () => {
      // The /login Google button can register a brand-new account, so the
      // consent small-print must appear here exactly like on /register.
      renderLoginPage();

      expect(screen.getByTestId("sso-consent-note")).toBeInTheDocument();
      expect(screen.getByTestId("sso-terms-link")).toHaveAttribute("href", "/terms");
      expect(screen.getByTestId("sso-privacy-link")).toHaveAttribute("href", "/privacy");
    });

    it("ignores a second GIS callback while the first sign-in is pending", async () => {
      let resolveSso: (value: unknown) => void;
      mockSsoLogin.mockReturnValueOnce(
        new Promise((resolve) => {
          resolveSso = resolve;
        }),
      );
      renderLoginPage();

      await emitCredential("google-credential");
      await emitCredential("google-credential-again");

      expect(mockSsoLogin).toHaveBeenCalledTimes(1);

      await act(async () => {
        resolveSso!(ssoResponse());
      });
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
      });
      expect(mockSsoLogin).toHaveBeenCalledTimes(1);
    });

    it("closes the dialog and toasts when the held Google credential expires during linking", async () => {
      const user = userEvent.setup();
      mockSsoLogin.mockRejectedValueOnce(
        new FetchError(409, "SSO_LINK_REQUIRED", "account exists"),
      );
      // Both are 401s — the dialog must discriminate by code, or an expired
      // credential loops "Incorrect password" forever on the right password.
      mockSsoLink.mockRejectedValueOnce(
        new FetchError(401, "SSO_INVALID_CREDENTIAL", "sign-in credential is invalid or expired"),
      );
      renderLoginPage();

      await emitCredential("google-credential");
      await waitFor(() => {
        expect(screen.getByTestId("link-account-dialog")).toBeInTheDocument();
      });

      await user.type(screen.getByTestId("link-account-password-input"), "correctpassword");
      await user.click(screen.getByTestId("link-account-submit"));

      await waitFor(() => {
        expect(screen.queryByTestId("link-account-dialog")).not.toBeInTheDocument();
      });
      expect(toast.error).toHaveBeenCalledWith(
        "Your Google sign-in expired. Tap the Google button and try again.",
      );
      expect(screen.queryByTestId("link-account-error")).not.toBeInTheDocument();
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it("closes the dialog and toasts the generic failure on a non-auth link error", async () => {
      const user = userEvent.setup();
      mockSsoLogin.mockRejectedValueOnce(
        new FetchError(409, "SSO_LINK_REQUIRED", "account exists"),
      );
      mockSsoLink.mockRejectedValueOnce(new FetchError(500, "INTERNAL_ERROR", "boom"));
      renderLoginPage();

      await emitCredential("google-credential");
      await waitFor(() => {
        expect(screen.getByTestId("link-account-dialog")).toBeInTheDocument();
      });

      await user.type(screen.getByTestId("link-account-password-input"), "password123");
      await user.click(screen.getByTestId("link-account-submit"));

      await waitFor(() => {
        expect(screen.queryByTestId("link-account-dialog")).not.toBeInTheDocument();
      });
      expect(toast.error).toHaveBeenCalledWith("Couldn't link your account. Please try again.");
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it("blocks cancel and dismissal while the link request is pending", async () => {
      const user = userEvent.setup();
      mockSsoLogin.mockRejectedValueOnce(
        new FetchError(409, "SSO_LINK_REQUIRED", "account exists"),
      );
      let resolveLink: (value: unknown) => void;
      mockSsoLink.mockReturnValueOnce(
        new Promise((resolve) => {
          resolveLink = resolve;
        }),
      );
      renderLoginPage();

      await emitCredential("google-credential");
      await waitFor(() => {
        expect(screen.getByTestId("link-account-dialog")).toBeInTheDocument();
      });

      await user.type(screen.getByTestId("link-account-password-input"), "password123");
      await user.click(screen.getByTestId("link-account-submit"));

      await waitFor(() => {
        expect(screen.getByTestId("link-account-cancel")).toBeDisabled();
      });
      // Neither Escape nor the (disabled) cancel button may dismiss it.
      await user.keyboard("{Escape}");
      expect(screen.getByTestId("link-account-dialog")).toBeInTheDocument();

      await act(async () => {
        resolveLink!(ssoResponse());
      });
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
      });
    });
  });
});
