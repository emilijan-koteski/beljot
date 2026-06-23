import "@/shared/i18n/i18n";

import { QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { queryKeys } from "@/shared/api/queryKeys";
import { useAuthStore } from "@/shared/stores/authStore";
import { useRoomStore } from "@/shared/stores/roomStore";
import { createTestQueryClient, QueryWrapper } from "@/test-utils";

import { MatchmakingPage } from "./MatchmakingPage";

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useParams: () => ({ id: "1" }),
    useNavigate: () => mockNavigate,
  };
});

vi.mock("@/shared/api/rooms", () => ({
  addBot: vi.fn(),
  removeBot: vi.fn(),
  getRoom: vi.fn(),
  leaveRoom: vi.fn(),
}));

vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsConnectionState: () => "connected",
  useWsSendMessage: () => vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

import { toast } from "sonner";

import { getRoom, leaveRoom } from "@/shared/api/rooms";

const mockGetRoom = vi.mocked(getRoom);
const mockLeaveRoom = vi.mocked(leaveRoom);

const qpRoom = {
  id: 1,
  name: "Quick Play ABC123",
  code: "ABC123",
  ownerId: 10,
  ownerUsername: "alice",
  variant: "bitola",
  matchMode: "1001",
  timerStyle: "relaxed",
  timerDurationSeconds: null,
  status: "waiting",
  playerCount: 1,
  isQuickPlay: true,
  coinBuyIn: 0,
  createdAt: "",
  updatedAt: "",
};

const viewer = {
  id: 10,
  username: "alice",
  email: "a@b.com",
  languagePreference: "en",
  walletBalance: 5000,
  loginStreakDays: 1,
  totalXp: 0,
  level: 0,
  createdAt: "",
};

function renderPage() {
  render(
    <QueryWrapper>
      <BrowserRouter>
        <MatchmakingPage />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

beforeEach(() => {
  mockLeaveRoom.mockResolvedValue(undefined);
  useAuthStore.setState({ user: viewer, token: "tok" });
});

afterEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ user: null, token: null });
  useRoomStore.getState().reset();
});

describe("MatchmakingPage", () => {
  it("renders the matchmaking diagram for a quick-play room the viewer is in", async () => {
    mockGetRoom.mockResolvedValue({
      room: qpRoom,
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByTestId("matchmaking-diagram")).toBeInTheDocument();
    });
    expect(screen.getByText("1 / 4 seated")).toBeInTheDocument();
    expect(screen.getByText("You")).toBeInTheDocument();
  });

  it("navigates to the game when the match starts", async () => {
    mockGetRoom.mockResolvedValue({
      room: qpRoom,
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderPage();
    await waitFor(() => expect(screen.getByTestId("matchmaking-diagram")).toBeInTheDocument());

    act(() => {
      useRoomStore.getState().setMatchStarted(true);
    });

    expect(mockNavigate).toHaveBeenCalledWith("/match/1", { state: { fromRoom: true } });
  });

  it("redirects a non-quick-play room to the in-room lobby", async () => {
    mockGetRoom.mockResolvedValue({
      room: { ...qpRoom, isQuickPlay: false },
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderPage();

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/rooms/1", { replace: true });
    });
  });

  it("cancels the queue: leaves the room and returns to the lobby", async () => {
    const user = userEvent.setup();
    mockGetRoom.mockResolvedValue({
      room: qpRoom,
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderPage();
    await waitFor(() => expect(screen.getByTestId("matchmaking-cancel")).toBeInTheDocument());

    await user.click(screen.getByTestId("matchmaking-cancel"));

    await waitFor(() => expect(mockLeaveRoom).toHaveBeenCalledWith(1));
    expect(mockNavigate).toHaveBeenCalledWith("/lobby");
  });

  it("keeps the player on cancel when the game already started", async () => {
    const user = userEvent.setup();
    mockLeaveRoom.mockRejectedValue(
      new FetchError(409, "MATCH_ALREADY_STARTED", "the match has already started"),
    );
    mockGetRoom.mockResolvedValue({
      room: qpRoom,
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderPage();
    await waitFor(() => expect(screen.getByTestId("matchmaking-cancel")).toBeInTheDocument());

    await user.click(screen.getByTestId("matchmaking-cancel"));

    await waitFor(() => expect(mockLeaveRoom).toHaveBeenCalledWith(1));
    expect(toast.error).toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalledWith("/lobby");
  });

  // Regression: re-entering a recently-visited room serves a STALE cached
  // detail snapshot that omits the just-joined viewer. The page must judge
  // membership on the fresh post-mount refetch — not the stale cache — or it
  // bounces the player to /lobby while the server keeps them in the room.
  it("does not bounce to lobby when the cached snapshot is stale (member added by refetch)", async () => {
    const client = createTestQueryClient();
    // Seed the cache as if the viewer left earlier: only the owner is present.
    client.setQueryData(queryKeys.rooms.detail(1), {
      room: qpRoom,
      players: [
        {
          id: 9,
          roomId: 1,
          userId: 9,
          username: "cece",
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
      ],
    });
    // The fresh fetch (after the just-completed quick-join) includes the viewer.
    mockGetRoom.mockResolvedValue({
      room: qpRoom,
      players: [
        {
          id: 9,
          roomId: 1,
          userId: 9,
          username: "cece",
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          seat: 1,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    render(
      <QueryClientProvider client={client}>
        <BrowserRouter>
          <MatchmakingPage />
        </BrowserRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => expect(screen.getByTestId("matchmaking-diagram")).toBeInTheDocument());
    expect(screen.getByText("2 / 4 seated")).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalledWith("/lobby", { replace: true });
  });
});
