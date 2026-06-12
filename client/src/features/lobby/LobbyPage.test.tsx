import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { QueryWrapper } from "@/test-utils";

import { LobbyPage } from "./LobbyPage";

const mockGetRooms = vi.fn();
const mockQuickPlay = vi.fn();
const mockQuickJoin = vi.fn();
const mockJoinRoom = vi.fn();
const mockGetLobbyStats = vi.fn();

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock("@/shared/api/rooms", () => ({
  addBot: vi.fn(),
  removeBot: vi.fn(),
  createRoom: vi.fn(),
  getRooms: (...args: unknown[]) => mockGetRooms(...args),
  joinRoom: (...args: unknown[]) => mockJoinRoom(...args),
  quickJoin: (...args: unknown[]) => mockQuickJoin(...args),
  quickPlay: (...args: unknown[]) => mockQuickPlay(...args),
  getRoomByCode: vi.fn(),
}));

vi.mock("@/shared/api/lobby", () => ({
  getLobbyStats: (...args: unknown[]) => mockGetLobbyStats(...args),
}));

vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsSendMessage: () => vi.fn(),
  useWsConnectionState: () => "connected" as const,
}));

afterEach(() => {
  vi.clearAllMocks();
});

function renderLobbyPage() {
  // Default mocks so the page can render without unhandled query rejections.
  mockGetRooms.mockResolvedValue([]);
  mockGetLobbyStats.mockResolvedValue({
    inLobby: 0,
    inRoom: 0,
    inMatch: 0,
    online: 0,
    registered: 0,
  });
  render(
    <QueryWrapper>
      <BrowserRouter>
        <LobbyPage />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

describe("LobbyPage", () => {
  it("renders hero action tiles", () => {
    renderLobbyPage();

    expect(screen.getByTestId("quick-play-card")).toBeInTheDocument();
    expect(screen.getByTestId("create-room-card")).toBeInTheDocument();
    expect(screen.getByTestId("join-by-code")).toBeInTheDocument();
  });

  it("renders the chat dock FAB", () => {
    renderLobbyPage();
    expect(screen.getByTestId("lobby-chat-fab")).toBeInTheDocument();
  });

  it("renders the filter rail with search + chips + sort", () => {
    renderLobbyPage();
    expect(screen.getByTestId("room-list-search")).toBeInTheDocument();
    expect(screen.getByTestId("filter-chip-all")).toBeInTheDocument();
    expect(screen.getByTestId("filter-chip-open")).toBeInTheDocument();
    expect(screen.getByTestId("filter-chip-relaxed")).toBeInTheDocument();
    expect(screen.getByTestId("filter-chip-timed")).toBeInTheDocument();
    expect(screen.getByTestId("sort-toggle")).toBeInTheDocument();
  });

  it("renders the lobby stats panel with all four pills", () => {
    renderLobbyPage();
    expect(screen.getByTestId("lobby-stats-panel")).toBeInTheDocument();
    expect(screen.getByTestId("stats-in-lobby")).toBeInTheDocument();
    expect(screen.getByTestId("stats-in-room")).toBeInTheDocument();
    expect(screen.getByTestId("stats-in-game")).toBeInTheDocument();
    expect(screen.getByTestId("stats-active-ratio")).toBeInTheDocument();
  });

  it("opens CreateRoomModal when Create Room card is clicked", async () => {
    const user = userEvent.setup();
    renderLobbyPage();

    const createRoomCard = screen.getByTestId("create-room-card");
    await user.click(createRoomCard);

    expect(screen.getByTestId("room-name-input")).toBeInTheDocument();
  });

  it("fetches rooms on mount (always-on lobby grid)", async () => {
    renderLobbyPage();
    await waitFor(() => {
      expect(mockGetRooms).toHaveBeenCalledWith("waiting");
    });
  });

  it("calls quickPlay API and navigates to the matchmaking screen on success", async () => {
    const user = userEvent.setup();
    mockQuickPlay.mockResolvedValueOnce({
      room: { id: 42, isQuickPlay: true },
      seat: 0,
      matchStarted: false,
    });
    renderLobbyPage();

    const quickPlayCard = screen.getByTestId("quick-play-card");
    await user.click(quickPlayCard);

    await waitFor(() => {
      expect(mockQuickPlay).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/matchmaking/42");
    });
  });

  it("navigates straight to the game when quickPlay reports matchStarted", async () => {
    const user = userEvent.setup();
    mockQuickPlay.mockResolvedValueOnce({
      room: { id: 77, isQuickPlay: true },
      seat: 3,
      matchStarted: true,
    });
    renderLobbyPage();

    const quickPlayCard = screen.getByTestId("quick-play-card");
    await user.click(quickPlayCard);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/match/77", { state: { fromRoom: true } });
    });
  });

  it("ports a quick-play room join to the matchmaking screen (quick-join)", async () => {
    const user = userEvent.setup();
    mockGetRooms.mockResolvedValueOnce([
      {
        id: 5,
        name: "Quick Play QPX",
        code: "QPX123",
        ownerId: 1,
        ownerUsername: "host",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        status: "waiting",
        playerCount: 1,
        isQuickPlay: true,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
        players: [
          { id: 1, roomId: 5, userId: 1, username: "host", seat: 0, team: "teamA", createdAt: "" },
        ],
      },
    ]);
    mockQuickJoin.mockResolvedValueOnce({
      room: { id: 5, isQuickPlay: true },
      seat: 1,
      matchStarted: false,
    });
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card-join")).toBeInTheDocument());
    await user.click(screen.getByTestId("room-card-join"));

    await waitFor(() => expect(mockQuickJoin).toHaveBeenCalledWith(5));
    expect(mockNavigate).toHaveBeenCalledWith("/matchmaking/5");
    expect(mockJoinRoom).not.toHaveBeenCalled();
  });

  it("sends a custom room join to the in-room lobby", async () => {
    const user = userEvent.setup();
    mockGetRooms.mockResolvedValueOnce([
      {
        id: 9,
        name: "Custom Table",
        code: "CUS123",
        ownerId: 1,
        ownerUsername: "host",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        status: "waiting",
        playerCount: 1,
        isQuickPlay: false,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
        players: [
          { id: 1, roomId: 9, userId: 1, username: "host", seat: 0, team: "teamA", createdAt: "" },
        ],
      },
    ]);
    mockJoinRoom.mockResolvedValueOnce({ id: 9 });
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card-join")).toBeInTheDocument());
    await user.click(screen.getByTestId("room-card-join"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(9));
    expect(mockNavigate).toHaveBeenCalledWith("/rooms/9");
    expect(mockQuickJoin).not.toHaveBeenCalled();
  });
});
