import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TopBar } from "@/shared/components/TopBar";
import { useAuthStore } from "@/shared/stores/authStore";

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

function renderWithRouter() {
  return render(
    <MemoryRouter initialEntries={["/lobby"]}>
      <Routes>
        <Route path="/lobby" element={<TopBar showNav showUserMenu />} />
        <Route path="/" element={<div data-testid="landing-page">Landing</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

function setAuthUser(overrides: Partial<import("@/shared/types/apiTypes").User> = {}) {
  useAuthStore.setState({
    token: "test-token",
    user: {
      id: 1,
      username: "kiro",
      email: "kiro@example.com",
      languagePreference: "en",
      walletBalance: 5000,
      loginStreakDays: 1,
      createdAt: "2026-01-01T00:00:00Z",
      ...overrides,
    },
    isLoading: false,
  });
}

describe("TopBar logout", () => {
  beforeEach(() => {
    setAuthUser();
  });

  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("clears auth state and navigates to the landing page (/) on logout", async () => {
    const user = userEvent.setup();
    renderWithRouter();

    await user.click(screen.getByTestId("nav-user"));
    await waitFor(() => {
      expect(screen.getByTestId("nav-logout")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("nav-logout"));

    await waitFor(() => {
      expect(useAuthStore.getState().token).toBeNull();
      expect(screen.getByTestId("landing-page")).toBeInTheDocument();
    });
  });
});

describe("TopBar coin balance", () => {
  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders the wallet balance from the store, locale-formatted", () => {
    setAuthUser({ walletBalance: 12345 });
    renderWithRouter();

    const pill = screen.getByTestId("coin-balance");
    expect(pill).toHaveTextContent((12345).toLocaleString());
  });

  it("renders correctly at a zero balance", () => {
    setAuthUser({ walletBalance: 0 });
    renderWithRouter();

    expect(screen.getByTestId("coin-balance")).toHaveTextContent("0");
  });

  it("does not render the login streak in the header, even at a high streak", () => {
    // The streak is surfaced in the daily-reward dialog and the profile only —
    // never alongside the header coin balance.
    setAuthUser({ loginStreakDays: 7 });
    renderWithRouter();

    expect(screen.getByTestId("coin-balance")).toBeInTheDocument();
    expect(screen.queryByTestId("login-streak")).not.toBeInTheDocument();
  });
});
