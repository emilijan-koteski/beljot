import "@/shared/i18n/i18n";

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { i18n } from "@/shared/i18n/i18n";
import { makeRoom, makeRoomPlayer, makeUser, mockViewportWidth, QueryWrapper } from "@/test-utils";

import { LobbyPage } from "./LobbyPage";

const mockGetRooms = vi.fn();
const mockQuickPlay = vi.fn();
const mockQuickJoin = vi.fn();
const mockJoinRoom = vi.fn();
const mockGetMyRoom = vi.fn();
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
  getMyRoom: (...args: unknown[]) => mockGetMyRoom(...args),
  getRoomByCode: vi.fn(),
}));

vi.mock("@/shared/api/lobby", () => ({
  getLobbyStats: (...args: unknown[]) => mockGetLobbyStats(...args),
}));

vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsSendMessage: () => vi.fn(),
  useWsConnectionState: () => "connected" as const,
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
    loading: vi.fn(),
    dismiss: vi.fn(),
  },
}));

let restoreViewport = () => {};

afterEach(() => {
  vi.clearAllMocks();
  restoreViewport();
  restoreViewport = () => {};
});

function renderLobbyPage() {
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
  it("shows two action tiles on a phone and pins New room to the bottom bar", () => {
    renderLobbyPage();

    const tiles = screen.getByTestId("lobby-actions");
    expect(within(tiles).getByTestId("quick-play-card")).toBeInTheDocument();
    expect(within(tiles).getByTestId("join-by-code-card")).toBeInTheDocument();
    expect(within(tiles).queryByTestId("create-room-card")).not.toBeInTheDocument();
    expect(
      within(screen.getByTestId("page-bottom-bar")).getByTestId("create-room-card"),
    ).toBeInTheDocument();
    expect(screen.getByTestId("open-friends")).toBeInTheDocument();
    expect(screen.queryByTestId("friends-sidebar")).not.toBeInTheDocument();
  });

  it("moves New room into a third tile on a tablet and keeps Friends in a sheet", () => {
    restoreViewport = mockViewportWidth(820);
    renderLobbyPage();

    const tiles = screen.getByTestId("lobby-actions");
    expect(within(tiles).getByTestId("create-room-card")).toBeInTheDocument();
    expect(within(tiles).getByTestId("join-by-code-card")).toBeInTheDocument();
    expect(screen.queryByTestId("page-bottom-bar")).not.toBeInTheDocument();
    expect(screen.getByTestId("open-friends")).toBeInTheDocument();
    expect(screen.queryByTestId("friends-sidebar")).not.toBeInTheDocument();
  });

  it("types the room code in place and shows the Friends sidebar on a desktop", () => {
    restoreViewport = mockViewportWidth(1280);
    renderLobbyPage();

    const tiles = screen.getByTestId("lobby-actions");
    expect(within(tiles).getByTestId("join-by-code-input")).toBeInTheDocument();
    expect(within(tiles).getByTestId("create-room-card")).toBeInTheDocument();
    expect(screen.getByTestId("friends-sidebar")).toBeInTheDocument();
    expect(screen.getByTestId("lobby-counts")).toBeInTheDocument();
    expect(screen.queryByTestId("open-friends")).not.toBeInTheDocument();
    expect(screen.queryByTestId("page-bottom-bar")).not.toBeInTheDocument();
  });

  it("opens the Friends sheet from the presence line", async () => {
    const user = userEvent.setup();
    renderLobbyPage();

    await user.click(screen.getByTestId("open-friends"));

    expect(await screen.findByTestId("friends-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("friends-panel")).toBeInTheDocument();
  });

  it("opens the Join by code sheet from its tile on a phone", async () => {
    const user = userEvent.setup();
    renderLobbyPage();

    await user.click(screen.getByTestId("join-by-code-card"));

    expect(await screen.findByTestId("join-by-code-dialog")).toBeInTheDocument();
  });

  it("renders the chat dock FAB", () => {
    renderLobbyPage();
    expect(screen.getByTestId("lobby-chat-fab")).toBeInTheDocument();
  });

  it("folds room search behind a button on a phone", async () => {
    const user = userEvent.setup();
    renderLobbyPage();

    expect(screen.queryByTestId("room-list-search")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("room-list-search-open"));
    expect(screen.getByTestId("room-list-search")).toBeInTheDocument();
  });

  it("renders the filter row with search, chips and sort on a desktop", () => {
    restoreViewport = mockViewportWidth(1280);
    renderLobbyPage();
    expect(screen.getByTestId("room-list-search")).toBeInTheDocument();
    expect(screen.getByTestId("filter-chip-all")).toBeInTheDocument();
    expect(screen.getByTestId("filter-chip-open")).toBeInTheDocument();
    expect(screen.getByTestId("filter-chip-relaxed")).toBeInTheDocument();
    expect(screen.getByTestId("filter-chip-timed")).toBeInTheDocument();
    expect(screen.getByTestId("sort-toggle")).toHaveTextContent("Filling up first");
  });

  it("shows the live head-counts beside the presence line", async () => {
    mockGetLobbyStats.mockResolvedValueOnce({
      inLobby: 3,
      inRoom: 4,
      inMatch: 46,
      online: 128,
      registered: 900,
    });
    renderLobbyPage();

    await waitFor(() =>
      expect(screen.getByTestId("lobby-counts")).toHaveTextContent("128 online · 46 playing"),
    );
  });

  it("mounts no seasonal surface — no rank banner, no leaderboard aside", () => {
    renderLobbyPage();

    expect(screen.queryByTestId("rank-banner")).not.toBeInTheDocument();
    expect(screen.queryByTestId("leaderboard-panel")).not.toBeInTheDocument();
    expect(screen.queryByTestId("lobby-sidebar")).not.toBeInTheDocument();
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

  it("offers a way back to the match when Quick Play says the player is already in one", async () => {
    const user = userEvent.setup();
    const { toast } = await import("sonner");
    mockQuickPlay.mockRejectedValueOnce(
      new FetchError(409, "ALREADY_IN_ROOM", "already in a room"),
    );
    mockGetMyRoom.mockResolvedValueOnce({ roomId: 77, status: "playing", isQuickPlay: true });
    renderLobbyPage();

    await user.click(screen.getByTestId("quick-play-card"));

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    const [message, options] = vi.mocked(toast.error).mock.calls[0] as [
      string,
      { action: { label: string; onClick: () => void } },
    ];
    expect(message).toBe(i18n.t("lobby.matchmaking.errors.alreadyInRoom"));
    expect(options.action.label).toBe("Back to your match");
    options.action.onClick();
    expect(mockNavigate).toHaveBeenCalledWith("/match/77", { state: { fromRoom: true } });
  });

  it("asks to try again when the room that blocked Quick Play is gone", async () => {
    const user = userEvent.setup();
    const { toast } = await import("sonner");
    mockQuickPlay.mockRejectedValueOnce(
      new FetchError(409, "ALREADY_IN_ROOM", "already in a room"),
    );
    mockGetMyRoom.mockResolvedValueOnce(null);
    renderLobbyPage();

    await user.click(screen.getByTestId("quick-play-card"));

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith("Your last room has closed. Try again."),
    );
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
        coinBuyIn: 0,
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

  it("shows the bracket-mismatch toast when a cross-bracket quick-play room is tapped (Story 9.4)", async () => {
    const user = userEvent.setup();
    const { toast } = await import("sonner");
    const { FetchError } = await import("@/shared/api/axiosClient");
    mockGetRooms.mockResolvedValueOnce([
      {
        id: 5,
        name: "Quick Play QPX",
        code: "QPX123",
        ownerId: 1,
        ownerUsername: "host",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "per-move",
        timerDurationSeconds: 30,
        status: "waiting",
        playerCount: 1,
        isQuickPlay: true,
        coinBuyIn: 500,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
        players: [
          { id: 1, roomId: 5, userId: 1, username: "host", seat: 0, team: "teamA", createdAt: "" },
        ],
      },
    ]);
    mockQuickJoin.mockRejectedValueOnce(
      new FetchError(409, "QUICK_PLAY_BRACKET_MISMATCH", "different bracket"),
    );
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card-join")).toBeInTheDocument());
    await user.click(screen.getByTestId("room-card-join"));

    await waitFor(() => expect(mockQuickJoin).toHaveBeenCalledWith(5));
    const msg = (toast.error as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[0] as string;
    expect(msg).toContain("coin bracket");
    expect(mockNavigate).not.toHaveBeenCalledWith("/matchmaking/5");
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
        coinBuyIn: 0,
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

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(9, undefined));
    expect(mockNavigate).toHaveBeenCalledWith("/rooms/9");
    expect(mockQuickJoin).not.toHaveBeenCalled();
  });

  it("shows the composed insufficient-coins toast when a join is rejected", async () => {
    const user = userEvent.setup();
    const { toast } = await import("sonner");
    const { FetchError } = await import("@/shared/api/axiosClient");
    const { useAuthStore } = await import("@/shared/stores/authStore");
    useAuthStore.setState({
      user: makeUser({
        username: "me",
        email: "me@test.dev",
        walletBalance: 300,
        createdAt: "2026-06-18T00:00:00Z",
      }),
    });

    mockGetRooms.mockResolvedValueOnce([
      {
        id: 9,
        name: "High Stakes",
        code: "HIS123",
        ownerId: 2,
        ownerUsername: "host",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        status: "waiting",
        playerCount: 1,
        isQuickPlay: false,
        coinBuyIn: 500,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
        players: [
          { id: 1, roomId: 9, userId: 2, username: "host", seat: 0, team: "teamA", createdAt: "" },
        ],
      },
    ]);
    mockJoinRoom.mockRejectedValueOnce(
      new FetchError(409, "INSUFFICIENT_COINS", "insufficient coins"),
    );
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card-join")).toBeInTheDocument());
    await user.click(screen.getByTestId("room-card-join"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(9, undefined));
    const msg = (toast.error as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[0] as string;
    expect(msg).toContain("500");
    expect(msg).toContain("300");
    expect(mockNavigate).not.toHaveBeenCalledWith("/rooms/9");
  });

  const gatedRoom = {
    id: 9,
    name: "Veterans Table",
    code: "VET123",
    ownerId: 2,
    ownerUsername: "host",
    variant: "bitola",
    matchMode: "1001",
    timerStyle: "relaxed",
    timerDurationSeconds: null,
    status: "waiting",
    playerCount: 1,
    isQuickPlay: false,
    coinBuyIn: 0,
    isPrivate: false,
    minHonor: 85,
    allowNewPlayers: false,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    players: [
      { id: 1, roomId: 9, userId: 2, username: "host", seat: 0, team: "teamA", createdAt: "" },
    ],
  };

  it("shows a gated room's honor floor on its card", async () => {
    mockGetRooms.mockResolvedValueOnce([gatedRoom]);
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card")).toBeInTheDocument());
    expect(screen.getByTestId("room-card-honor")).toHaveTextContent("85+");
    expect(screen.getByTestId("room-card-join")).toBeInTheDocument();
  });

  it("locks Join on a room the viewer cannot qualify for, without firing a request", async () => {
    const user = userEvent.setup();
    const { useAuthStore } = await import("@/shared/stores/authStore");
    useAuthStore.setState({
      user: makeUser({ username: "me", honorScore: 42, isNewPlayer: false }),
    });

    mockGetRooms.mockResolvedValueOnce([gatedRoom]);
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card-join")).toBeInTheDocument());
    const join = screen.getByTestId("room-card-join");
    expect(screen.getByTestId("room-card")).toHaveAttribute("data-state", "disabled");
    expect(join).toBeDisabled();
    expect(join).toHaveAttribute("title", expect.stringContaining("85"));
    expect(join).toHaveAttribute("title", expect.stringContaining("42"));

    await user.click(join);
    expect(mockJoinRoom).not.toHaveBeenCalled();
  });

  it("composes the HONOR_TOO_LOW toast from the room's threshold and the viewer's score", async () => {
    const user = userEvent.setup();
    const { toast } = await import("sonner");
    const { FetchError } = await import("@/shared/api/axiosClient");
    const { useAuthStore } = await import("@/shared/stores/authStore");
    useAuthStore.setState({
      user: makeUser({ username: "me", honorScore: 42, isNewPlayer: false }),
    });

    mockGetRooms.mockResolvedValueOnce([{ ...gatedRoom, minHonor: 40 }]);
    mockJoinRoom.mockRejectedValueOnce(new FetchError(409, "HONOR_TOO_LOW", "honor too low"));
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card-join")).toBeInTheDocument());
    await user.click(screen.getByTestId("room-card-join"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(9, undefined));
    const msg = (toast.error as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[0] as string;
    expect(msg).toContain("40");
    expect(msg).toContain("42");
    expect(mockNavigate).not.toHaveBeenCalledWith("/rooms/9");
  });

  it("renders a real honor score of 0 in the HONOR_TOO_LOW toast", async () => {
    const user = userEvent.setup();
    const { toast } = await import("sonner");
    const { FetchError } = await import("@/shared/api/axiosClient");
    const { useAuthStore } = await import("@/shared/stores/authStore");
    useAuthStore.setState({
      user: makeUser({ username: "me", honorScore: 0, isNewPlayer: true }),
    });

    mockGetRooms.mockResolvedValueOnce([{ ...gatedRoom, minHonor: 50, allowNewPlayers: true }]);
    mockJoinRoom.mockRejectedValueOnce(new FetchError(409, "HONOR_TOO_LOW", "honor too low"));
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card-join")).toBeInTheDocument());
    await user.click(screen.getByTestId("room-card-join"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalled());
    const msg = (toast.error as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[0] as string;
    expect(msg).toContain("0");
    expect(msg).not.toContain("80");
  });

  it("shows the veterans-only toast for NEW_PLAYER_NOT_ALLOWED", async () => {
    const user = userEvent.setup();
    const { toast } = await import("sonner");
    const { FetchError } = await import("@/shared/api/axiosClient");
    const { useAuthStore } = await import("@/shared/stores/authStore");
    useAuthStore.setState({
      user: makeUser({ username: "me", honorScore: 80, isNewPlayer: true }),
    });

    mockGetRooms.mockResolvedValueOnce([{ ...gatedRoom, allowNewPlayers: true }]);
    mockJoinRoom.mockRejectedValueOnce(
      new FetchError(409, "NEW_PLAYER_NOT_ALLOWED", "new players not allowed"),
    );
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card-join")).toBeInTheDocument());
    await user.click(screen.getByTestId("room-card-join"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(9, undefined));
    expect(toast.error).toHaveBeenLastCalledWith(i18n.t("room.errors.newPlayerNotAllowed"));
    expect(mockNavigate).not.toHaveBeenCalledWith("/rooms/9");
  });

  it("filters the grid to rooms the viewer qualifies for", async () => {
    const user = userEvent.setup();
    const { useAuthStore } = await import("@/shared/stores/authStore");
    useAuthStore.setState({
      user: makeUser({ username: "me", honorScore: 42, isNewPlayer: false }),
    });

    mockGetRooms.mockResolvedValueOnce([
      gatedRoom,
      {
        ...gatedRoom,
        id: 10,
        name: "Open Table",
        code: "OPN123",
        minHonor: 0,
        allowNewPlayers: true,
      },
    ]);
    renderLobbyPage();

    await waitFor(() => expect(screen.getAllByTestId("room-card")).toHaveLength(2));
    await user.click(screen.getByTestId("filter-chip-qualify"));

    await waitFor(() => expect(screen.getAllByTestId("room-card")).toHaveLength(1));
    expect(screen.getByText("Open Table")).toBeInTheDocument();
  });

  const privateRoom = {
    id: 9,
    name: "Private Table",
    code: "PRV123",
    ownerId: 1,
    ownerUsername: "host",
    variant: "bitola",
    matchMode: "1001",
    timerStyle: "relaxed",
    timerDurationSeconds: null,
    status: "waiting",
    playerCount: 1,
    isQuickPlay: false,
    coinBuyIn: 0,
    isPrivate: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    players: [
      { id: 1, roomId: 9, userId: 1, username: "host", seat: 0, team: "teamA", createdAt: "" },
    ],
  };

  it("prompts for a password before joining a private room", async () => {
    const user = userEvent.setup();
    mockGetRooms.mockResolvedValueOnce([{ ...privateRoom }]);
    mockJoinRoom.mockResolvedValueOnce({ id: 9 });
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card-join")).toBeInTheDocument());
    await user.click(screen.getByTestId("room-card-join"));

    expect(await screen.findByTestId("password-prompt-dialog")).toBeInTheDocument();
    expect(mockJoinRoom).not.toHaveBeenCalled();

    await user.type(screen.getByTestId("password-prompt-input"), "secret");
    await user.click(screen.getByTestId("password-prompt-submit"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(9, "secret"));
    expect(mockNavigate).toHaveBeenCalledWith("/rooms/9");
  });

  it("keeps the password dialog open and shows an error on a wrong password", async () => {
    const user = userEvent.setup();
    const { FetchError } = await import("@/shared/api/axiosClient");
    mockGetRooms.mockResolvedValueOnce([{ ...privateRoom }]);
    mockJoinRoom.mockRejectedValueOnce(
      new FetchError(409, "WRONG_ROOM_PASSWORD", "incorrect room password"),
    );
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card-join")).toBeInTheDocument());
    await user.click(screen.getByTestId("room-card-join"));
    await user.type(await screen.findByTestId("password-prompt-input"), "nope");
    await user.click(screen.getByTestId("password-prompt-submit"));

    await waitFor(() => expect(screen.getByTestId("password-prompt-error")).toBeInTheDocument());
    expect(screen.getByTestId("password-prompt-dialog")).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalledWith("/rooms/9");
  });

  it("counts bots as seated, so a room filled with bots is full and can't be joined", async () => {
    const humans = [0, 2].map((seat) =>
      makeRoomPlayer({ id: seat + 1, userId: seat + 1, username: `p${seat}`, seat }),
    );
    const bots = [1, 3].map((seat) =>
      makeRoomPlayer({ id: 0, userId: 0, username: "", seat, isBot: true }),
    );
    mockGetRooms.mockResolvedValueOnce([
      makeRoom({
        id: 9,
        name: "Bot Table",
        code: "BOT123",
        playerCount: 2,
        players: [...humans, ...bots],
      }),
    ]);
    renderLobbyPage();

    await waitFor(() => expect(screen.getByTestId("room-card")).toBeInTheDocument());
    expect(screen.getByTestId("room-card")).toHaveAttribute("data-state", "disabled");
    expect(screen.getByTestId("room-card-join")).toBeDisabled();
    expect(screen.getByTestId("filter-chip-open")).toHaveTextContent("0");
  });

  it("lists rooms nearest to starting first and full rooms last", async () => {
    const seatedPlayers = (count: number, roomId: number) =>
      Array.from({ length: count }, (_, seat) =>
        makeRoomPlayer({
          id: roomId * 10 + seat,
          roomId,
          userId: roomId * 10 + seat,
          username: `u${roomId}${seat}`,
          seat,
        }),
      );
    mockGetRooms.mockResolvedValueOnce([
      makeRoom({
        id: 1,
        name: "Full",
        code: "FUL123",
        playerCount: 4,
        players: seatedPlayers(4, 1),
      }),
      makeRoom({
        id: 2,
        name: "One",
        code: "ONE123",
        playerCount: 1,
        players: seatedPlayers(1, 2),
      }),
      makeRoom({
        id: 3,
        name: "Three",
        code: "THR123",
        playerCount: 3,
        players: seatedPlayers(3, 3),
      }),
    ]);
    renderLobbyPage();

    await waitFor(() => expect(screen.getAllByTestId("room-card")).toHaveLength(3));
    const names = screen
      .getAllByTestId("room-card")
      .map((card) => within(card).getByRole("heading").textContent);
    expect(names).toEqual(["Three", "One", "Full"]);
  });

  it("lists live matches read-only on the Playing tab", async () => {
    const user = userEvent.setup();
    mockGetRooms.mockImplementation((status: string) =>
      Promise.resolve(
        status === "playing"
          ? [
              makeRoom({
                id: 40,
                name: "Live Table",
                code: "LIV123",
                status: "playing",
                playerCount: 4,
              }),
            ]
          : [],
      ),
    );
    render(
      <QueryWrapper>
        <BrowserRouter>
          <LobbyPage />
        </BrowserRouter>
      </QueryWrapper>,
    );

    await waitFor(() =>
      expect(screen.getByTestId("lobby-tabs-playing")).toHaveTextContent("Playing (1)"),
    );
    await user.click(screen.getByTestId("lobby-tabs-playing"));

    expect(await screen.findByText("Live Table")).toBeInTheDocument();
    expect(screen.getByTestId("room-card")).toHaveAttribute("data-state", "live");
    expect(screen.queryByTestId("room-card-join")).not.toBeInTheDocument();
    expect(screen.queryByTestId("filter-chip-all")).not.toBeInTheDocument();
  });
});

describe("LobbyPage support note", () => {
  it("renders one quiet support line, last on the page", () => {
    renderLobbyPage();

    const note = screen.getByTestId("support-note");
    expect(note.textContent).toContain("A hobby project. You can always buy me a coffee.");
    const footnote = screen.getByText(/Showing/);
    expect(footnote.compareDocumentPosition(note)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it("shows no support dialog until the line is used", async () => {
    const user = userEvent.setup();
    renderLobbyPage();

    expect(screen.queryByTestId("support-dialog")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("support-note-link"));

    expect(await screen.findByTestId("support-dialog")).toBeInTheDocument();
  });
});
