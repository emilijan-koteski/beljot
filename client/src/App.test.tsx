import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { LoginPage } from "@/features/auth/LoginPage";
import { LobbyPage } from "@/features/lobby/LobbyPage";
import { useAuthStore } from "@/shared/stores/authStore";

vi.mock("@/shared/api/auth", () => ({
  login: vi.fn(),
  refresh: vi.fn(),
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
});
