import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { App } from "@/App";
import { LoginPage } from "@/features/auth/LoginPage";
import { LobbyPage } from "@/features/lobby/LobbyPage";
import { useAuthStore } from "@/shared/stores/authStore";

vi.mock("@/shared/api/auth", () => ({
  login: vi.fn(),
  // Reject so useAuthInit settles to the logged-out landing page when <App />
  // is rendered (no real session in jsdom).
  refresh: vi.fn(() => Promise.reject(new Error("no session"))),
  logout: vi.fn(),
}));

vi.mock("@/shared/api/rooms", () => ({
  addBot: vi.fn(),
  removeBot: vi.fn(),
  createRoom: vi.fn(),
}));

vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsSendMessage: () => vi.fn(),
  useWsConnectionState: () => "connected" as const,
}));

describe("App routing", () => {
  beforeEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders login page at /login", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={["/login"]}>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByTestId("login-title")).toHaveTextContent("Log in");
  });

  it("renders lobby page at /lobby", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={["/lobby"]}>
          <Routes>
            <Route path="/lobby" element={<LobbyPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByTestId("quick-play-card")).toBeInTheDocument();
    expect(screen.getByTestId("create-room-card")).toBeInTheDocument();
  });

  // Regression guard: the sonner <Toaster> host must be mounted at the app
  // root. Without it, every toast.*() call (join/settlement feedback, auth
  // errors) is silently invisible even though the calls succeed — a bug that
  // unit tests miss because they mock `toast`. The Toaster renders an
  // aria-label="Notifications …" region regardless of route or auth state.
  it("mounts the global sonner Toaster so toasts are visible app-wide", async () => {
    render(<App />);

    expect(await screen.findByRole("region", { name: /notifications/i })).toBeInTheDocument();
  });
});
