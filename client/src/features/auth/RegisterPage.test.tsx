import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import { QueryWrapper } from "@/test-utils";

import { AuthLayout } from "./AuthLayout";
import { RegisterPage } from "./RegisterPage";

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockRegister = vi.fn();
const mockSsoLogin = vi.fn();
const mockSsoLink = vi.fn();
vi.mock("@/shared/api/auth", () => ({
  register: (...args: unknown[]) => mockRegister(...args),
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

function renderRegisterPage() {
  return render(
    <QueryWrapper>
      <MemoryRouter initialEntries={["/register"]}>
        <Routes>
          <Route element={<AuthLayout />}>
            <Route path="/register" element={<RegisterPage />} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryWrapper>,
  );
}

let capturedOnCredential: ((credential: string) => void) | undefined;

describe("RegisterPage", () => {
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

  it("renders email, username, password fields and submit button", () => {
    renderRegisterPage();

    expect(screen.getByTestId("email-input")).toBeInTheDocument();
    expect(screen.getByTestId("username-input")).toBeInTheDocument();
    expect(screen.getByTestId("password-input")).toBeInTheDocument();
    expect(screen.getByTestId("submit-button")).toBeInTheDocument();
    expect(screen.getByTestId("login-link")).toBeInTheDocument();
    expect(screen.getByTestId("consent-checkbox")).toBeInTheDocument();
    expect(screen.getByTestId("terms-link")).toHaveAttribute("href", "/terms");
    expect(screen.getByTestId("privacy-link")).toHaveAttribute("href", "/privacy");
  });

  it("disables submit until the consent checkbox is checked", async () => {
    renderRegisterPage();
    const user = userEvent.setup();

    expect(screen.getByTestId("submit-button")).toBeDisabled();

    await user.click(screen.getByTestId("consent-checkbox"));

    expect(screen.getByTestId("submit-button")).not.toBeDisabled();
  });

  it("opens terms and privacy links in a new tab", () => {
    renderRegisterPage();

    expect(screen.getByTestId("terms-link")).toHaveAttribute("target", "_blank");
    expect(screen.getByTestId("terms-link")).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.getByTestId("privacy-link")).toHaveAttribute("target", "_blank");
    expect(screen.getByTestId("privacy-link")).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("shows validation error on blur for empty email", async () => {
    renderRegisterPage();

    const emailInput = screen.getByTestId("email-input");
    fireEvent.focus(emailInput);
    fireEvent.blur(emailInput);

    await waitFor(() => {
      expect(screen.getByTestId("email-error")).toHaveTextContent("Email is required");
    });
  });

  it("shows validation error on blur for invalid email", async () => {
    renderRegisterPage();
    const user = userEvent.setup();

    const emailInput = screen.getByTestId("email-input");
    await user.type(emailInput, "not-an-email");
    fireEvent.blur(emailInput);

    await waitFor(() => {
      expect(screen.getByTestId("email-error")).toHaveTextContent("Enter a valid email address");
    });
  });

  it("shows validation error on blur for short username", async () => {
    renderRegisterPage();
    const user = userEvent.setup();

    const usernameInput = screen.getByTestId("username-input");
    await user.type(usernameInput, "ab");
    fireEvent.blur(usernameInput);

    await waitFor(() => {
      expect(screen.getByTestId("username-error")).toHaveTextContent(
        "Username must be at least 3 characters",
      );
    });
  });

  it("shows validation error on blur for short password", async () => {
    renderRegisterPage();
    const user = userEvent.setup();

    const passwordInput = screen.getByTestId("password-input");
    await user.type(passwordInput, "short");
    fireEvent.blur(passwordInput);

    await waitFor(() => {
      expect(screen.getByTestId("password-error")).toHaveTextContent(
        "Password must be at least 8 characters",
      );
    });
  });

  it("navigates to /lobby on successful registration", async () => {
    mockRegister.mockResolvedValueOnce({
      token: "mock-token",
      id: 1,
      username: "testuser",
      email: "test@example.com",
      languagePreference: "en",
      walletBalance: 5000,
      loginStreakDays: 1,
      totalXp: 0,
      level: 0,
      createdAt: "2026-04-10T00:00:00Z",
    });

    renderRegisterPage();
    const user = userEvent.setup();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("username-input"), "testuser");
    await user.type(screen.getByTestId("password-input"), "password123");
    await user.click(screen.getByTestId("consent-checkbox"));
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
    });
  });

  it("renders the pre-auth language selector", () => {
    renderRegisterPage();
    expect(screen.getByTestId("auth-language-selector")).toBeInTheDocument();
  });

  it("includes the active i18n language in the register payload", async () => {
    await i18n.changeLanguage("mk");
    mockRegister.mockResolvedValueOnce({
      token: "mock-token",
      id: 1,
      username: "testuser",
      email: "test@example.com",
      languagePreference: "mk",
      walletBalance: 5000,
      loginStreakDays: 1,
      totalXp: 0,
      level: 0,
      createdAt: "2026-04-10T00:00:00Z",
    });

    renderRegisterPage();
    const user = userEvent.setup();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("username-input"), "testuser");
    await user.type(screen.getByTestId("password-input"), "password123");
    await user.click(screen.getByTestId("consent-checkbox"));
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(mockRegister).toHaveBeenCalledWith({
        email: "test@example.com",
        username: "testuser",
        password: "password123",
        languagePreference: "mk",
      });
    });
  });

  it("blocks submit and shows consent error when checkbox is not checked", async () => {
    renderRegisterPage();
    const user = userEvent.setup();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("username-input"), "testuser");
    await user.type(screen.getByTestId("password-input"), "password123");

    expect(screen.getByTestId("submit-button")).toBeDisabled();

    fireEvent.submit(screen.getByTestId("register-form"));

    await waitFor(() => {
      expect(screen.getByTestId("consent-error")).toHaveTextContent(
        "You must accept the Terms of Service and Privacy Policy",
      );
    });

    expect(mockRegister).not.toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("clears the consent error once the checkbox is checked", async () => {
    renderRegisterPage();
    const user = userEvent.setup();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("username-input"), "testuser");
    await user.type(screen.getByTestId("password-input"), "password123");
    fireEvent.submit(screen.getByTestId("register-form"));

    await waitFor(() => {
      expect(screen.getByTestId("consent-error")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("consent-checkbox"));

    expect(screen.queryByTestId("consent-error")).not.toBeInTheDocument();
  });

  it("displays inline error for EMAIL_TAKEN server response", async () => {
    mockRegister.mockRejectedValueOnce(
      new FetchError(409, "EMAIL_TAKEN", "email is already registered"),
    );

    renderRegisterPage();
    const user = userEvent.setup();

    await user.type(screen.getByTestId("email-input"), "taken@example.com");
    await user.type(screen.getByTestId("username-input"), "testuser");
    await user.type(screen.getByTestId("password-input"), "password123");
    await user.click(screen.getByTestId("consent-checkbox"));
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(screen.getByTestId("email-error")).toHaveTextContent(
        "This email is already registered",
      );
    });
  });

  it("displays inline error for USERNAME_TAKEN server response", async () => {
    mockRegister.mockRejectedValueOnce(
      new FetchError(409, "USERNAME_TAKEN", "username is already taken"),
    );

    renderRegisterPage();
    const user = userEvent.setup();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("username-input"), "takenuser");
    await user.type(screen.getByTestId("password-input"), "password123");
    await user.click(screen.getByTestId("consent-checkbox"));
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(screen.getByTestId("username-error")).toHaveTextContent(
        "This username is already taken",
      );
    });
  });

  it("shows validation error on blur for username with invalid characters", async () => {
    renderRegisterPage();
    const user = userEvent.setup();

    const usernameInput = screen.getByTestId("username-input");
    await user.type(usernameInput, "bad user!");
    fireEvent.blur(usernameInput);

    await waitFor(() => {
      expect(screen.getByTestId("username-error")).toHaveTextContent(
        "Letters, numbers, and underscores only",
      );
    });
  });

  it("toggles password visibility", async () => {
    renderRegisterPage();
    const user = userEvent.setup();

    const passwordInput = screen.getByTestId("password-input");
    expect(passwordInput).toHaveAttribute("type", "password");

    const toggleButton = screen.getByTestId("password-toggle");
    await user.click(toggleButton);

    expect(passwordInput).toHaveAttribute("type", "text");

    await user.click(toggleButton);

    expect(passwordInput).toHaveAttribute("type", "password");
  });

  it("disables submit button during loading", async () => {
    let resolveRegister: (value: unknown) => void;
    mockRegister.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveRegister = resolve;
      }),
    );

    renderRegisterPage();
    const user = userEvent.setup();

    await user.type(screen.getByTestId("email-input"), "test@example.com");
    await user.type(screen.getByTestId("username-input"), "testuser");
    await user.type(screen.getByTestId("password-input"), "password123");
    await user.click(screen.getByTestId("consent-checkbox"));
    await user.click(screen.getByTestId("submit-button"));

    await waitFor(() => {
      expect(screen.getByTestId("submit-button")).toBeDisabled();
    });

    resolveRegister!({
      token: "tok",
      id: 1,
      username: "testuser",
      email: "test@example.com",
      languagePreference: "en",
      walletBalance: 5000,
      loginStreakDays: 1,
      totalXp: 0,
      level: 0,
      createdAt: "2026-04-10T00:00:00Z",
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

    it("renders the Google button slot with the ToS/privacy small-print", async () => {
      renderRegisterPage();

      expect(screen.getByTestId("google-signin-button")).toBeInTheDocument();
      expect(screen.getByTestId("sso-divider")).toBeInTheDocument();
      expect(screen.getByTestId("sso-consent-note")).toBeInTheDocument();
      expect(screen.getByTestId("sso-terms-link")).toHaveAttribute("href", "/terms");
      expect(screen.getByTestId("sso-privacy-link")).toHaveAttribute("href", "/privacy");
      await waitFor(() => {
        expect(mockRenderGoogleButton).toHaveBeenCalled();
      });
    });

    it("hides the Google button when no client id is configured", () => {
      mockClientId = "";
      renderRegisterPage();

      expect(screen.queryByTestId("google-signin-button")).not.toBeInTheDocument();
      expect(mockRenderGoogleButton).not.toHaveBeenCalled();
    });

    it("stores the session and navigates to /lobby on SSO success", async () => {
      mockSsoLogin.mockResolvedValueOnce(ssoResponse());
      renderRegisterPage();

      await emitCredential("google-credential");

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
      });
      expect(mockSsoLogin).toHaveBeenCalledWith("google", { credential: "google-credential" });
      expect(useAuthStore.getState().token).toBe("sso-token");
    });

    it("reconciles the picked language after an SSO registration", async () => {
      await i18n.changeLanguage("mk");
      // SSO registration always seeds "en" server-side — the page must PATCH
      // the picked language just like the login page does.
      mockSsoLogin.mockResolvedValueOnce(ssoResponse());
      mockUpdatePreferences.mockResolvedValueOnce({ languagePreference: "mk" });
      renderRegisterPage();

      await emitCredential("google-credential");

      await waitFor(() => {
        expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { languagePreference: "mk" });
      });
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
      });
    });

    it("links via the dialog when the email matches a password account", async () => {
      const user = userEvent.setup();
      mockSsoLogin.mockRejectedValueOnce(
        new FetchError(409, "SSO_LINK_REQUIRED", "account exists"),
      );
      mockSsoLink.mockResolvedValueOnce(ssoResponse());
      renderRegisterPage();

      await emitCredential("google-credential");
      await waitFor(() => {
        expect(screen.getByTestId("link-account-dialog")).toBeInTheDocument();
      });
      expect(screen.getByTestId("link-account-email")).toHaveTextContent("match@example.com");

      await user.type(screen.getByTestId("link-account-password-input"), "password123");
      await user.click(screen.getByTestId("link-account-submit"));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
      });
      expect(mockSsoLink).toHaveBeenCalledWith("google", {
        credential: "google-credential",
        password: "password123",
      });
    });

    it("shows an inline error in the dialog on a wrong password", async () => {
      const user = userEvent.setup();
      mockSsoLogin.mockRejectedValueOnce(
        new FetchError(409, "SSO_LINK_REQUIRED", "account exists"),
      );
      mockSsoLink.mockRejectedValueOnce(
        new FetchError(401, "INVALID_CREDENTIALS", "invalid email or password"),
      );
      renderRegisterPage();

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
    });
  });
});
