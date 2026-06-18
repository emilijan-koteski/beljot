import "@/shared/i18n/i18n";

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { i18n } from "@/shared/i18n/i18n";
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
vi.mock("@/shared/api/auth", () => ({
  register: (...args: unknown[]) => mockRegister(...args),
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

describe("RegisterPage", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    await i18n.changeLanguage("en");
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
      expect(mockNavigate).toHaveBeenCalledWith("/lobby");
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
      createdAt: "2026-04-10T00:00:00Z",
    });
  });
});
