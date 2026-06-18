import { beforeEach, describe, expect, it, vi } from "vitest";

const mockLogoutApi = vi.fn();
vi.mock("@/shared/api/auth", () => ({
  logout: (...args: unknown[]) => mockLogoutApi(...args),
}));

import { useAuthStore } from "./authStore";
import { useChatStore } from "./chatStore";
import { useMatchStore } from "./matchStore";
import { useRoomStore } from "./roomStore";

describe("authStore", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    useMatchStore.getState().clearGame();
    useRoomStore.getState().reset();
    useChatStore.getState().clearLobby();
    useChatStore.getState().clearMatch();
    useChatStore.getState().clearRoom();
  });

  it("logout clears token and user from store", () => {
    useAuthStore.setState({
      token: "test-token",
      user: {
        id: 1,
        username: "test",
        email: "test@test.com",
        languagePreference: "en",
        walletBalance: 5000,
        loginStreakDays: 1,
        createdAt: "2026-01-01",
      },
    });

    useAuthStore.getState().logout();

    expect(useAuthStore.getState().token).toBeNull();
    expect(useAuthStore.getState().user).toBeNull();
  });

  it("logout calls backend /auth/logout endpoint", () => {
    useAuthStore.setState({ token: "test-token" });

    useAuthStore.getState().logout();

    expect(mockLogoutApi).toHaveBeenCalled();
  });

  // Story 8.5-1 AC6 (D66): logout must wipe session-scoped stores so a
  // re-login does not inherit stale state. Without this, the 401 →
  // refresh-fail → logout path leaves matchStore.matchState populated and
  // useReconnectionRedirect navigates the freshly re-logged-in user to a
  // finished game's /game/{id} page.
  it("clears matchStore on logout", () => {
    // Bypass setMatchState's normalization (which expects a full MatchState
    // shape) — the test only cares that matchState is non-null before logout
    // and null after. Other tests cover the MatchState shape.
    useMatchStore.setState({
      matchState: {
        phase: "match_end",
        players: [],
      } as unknown as Parameters<typeof useMatchStore.setState>[0] extends infer S
        ? S extends { matchState?: infer G }
          ? G
          : never
        : never,
    });
    expect(useMatchStore.getState().matchState).not.toBeNull();

    useAuthStore.getState().logout();

    expect(useMatchStore.getState().matchState).toBeNull();
  });

  it("resets roomStore on logout", () => {
    useRoomStore
      .getState()
      .setPlayers([{ id: 1, roomId: 1, userId: 10, username: "P1" }] as unknown as Parameters<
        ReturnType<typeof useRoomStore.getState>["setPlayers"]
      >[0]);
    expect(useRoomStore.getState().players.length).toBeGreaterThan(0);

    useAuthStore.getState().logout();

    expect(useRoomStore.getState().players).toEqual([]);
  });

  it("clears chatStore buffers on logout", () => {
    const msg = {
      userId: 10,
      username: "P1",
      message: "hello",
      timestamp: new Date().toISOString(),
      scope: "lobby" as const,
    };
    useChatStore.getState().appendLobby(msg);
    useChatStore.getState().appendMatch({ ...msg, scope: "match" as const });
    useChatStore.getState().appendRoom({ ...msg, scope: "room" as const });
    expect(useChatStore.getState().lobbyMessages.length).toBe(1);
    expect(useChatStore.getState().matchMessages.length).toBe(1);
    expect(useChatStore.getState().roomMessages.length).toBe(1);

    useAuthStore.getState().logout();

    expect(useChatStore.getState().lobbyMessages).toEqual([]);
    expect(useChatStore.getState().matchMessages).toEqual([]);
    expect(useChatStore.getState().roomMessages).toEqual([]);
  });
});
