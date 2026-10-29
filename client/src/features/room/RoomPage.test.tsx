import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { queryKeys } from "@/shared/api/queryKeys";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import type { RoomPlayer } from "@/shared/types/apiTypes";
import { makeRoom, makeRoomDetail, makeRoomPlayer, makeUser, QueryWrapper } from "@/test-utils";

import { RoomPage } from "./RoomPage";

const mockNavigate = vi.fn();
const { routeParams } = vi.hoisted(() => ({ routeParams: { id: "1" } }));
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useParams: () => ({ id: routeParams.id }),
    useNavigate: () => mockNavigate,
  };
});

vi.mock("@/shared/api/rooms", () => ({
  addBot: vi.fn(),
  removeBot: vi.fn(),
  getRoom: vi.fn(),
  joinRoom: vi.fn(),
  leaveRoom: vi.fn(),
  selectSeat: vi.fn(),
  startMatch: vi.fn(),
  kickPlayer: vi.fn(),
  swapSeats: vi.fn(),
  leaveSeat: vi.fn(),
  transferOwnership: vi.fn(),
}));

const mockGetRoomLastMatch = vi.fn();
vi.mock("@/shared/api/matches", () => ({
  getRoomLastMatch: (roomId: number) => mockGetRoomLastMatch(roomId),
}));

const mockGetFriendshipStatus = vi.fn();
vi.mock("@/shared/api/friends", () => ({
  getFriendshipStatus: (id: number) => mockGetFriendshipStatus(id),
  sendFriendRequest: vi.fn(),
  acceptFriendRequest: vi.fn(),
  declineFriendRequest: vi.fn(),
  removeFriend: vi.fn(),
}));

vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsConnectionState: () => "connected",
  useWsSendMessage: () => vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import { toast } from "sonner";

import {
  getRoom,
  joinRoom,
  kickPlayer,
  leaveRoom,
  leaveSeat,
  selectSeat,
  startMatch,
  swapSeats,
  transferOwnership,
} from "@/shared/api/rooms";

const mockGetRoom = vi.mocked(getRoom);
const mockJoinRoom = vi.mocked(joinRoom);
const mockLeaveRoom = vi.mocked(leaveRoom);
const mockSelectSeat = vi.mocked(selectSeat);
const mockStartGame = vi.mocked(startMatch);
const mockKickPlayer = vi.mocked(kickPlayer);
const mockSwapSeats = vi.mocked(swapSeats);
const mockLeaveSeat = vi.mocked(leaveSeat);
const mockTransferOwnership = vi.mocked(transferOwnership);

function renderRoomPage() {
  render(
    <QueryWrapper>
      <BrowserRouter>
        <RoomPage />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

const defaultRoom = makeRoom({
  id: 1,
  name: "Test Room",
  code: "XYZ123",
  ownerId: 10,
  ownerUsername: "owner",
  variant: "bitola",
  matchMode: "1001",
  timerStyle: "relaxed",
  timerDurationSeconds: null,
  status: "waiting",
  playerCount: 2,
  isQuickPlay: false,
  coinBuyIn: 0,
  isPrivate: false,
  minHonor: 0,
  allowNewPlayers: true,
  createdAt: "",
  updatedAt: "",
});

const defaultUser = makeUser({
  id: 10,
  username: "alice",
  email: "a@b.com",
  loginStreakDays: 1,
  createdAt: "",
});

beforeEach(() => {
  mockLeaveRoom.mockResolvedValue(undefined);
  mockJoinRoom.mockResolvedValue(defaultRoom);
  mockGetRoomLastMatch.mockReset();
  mockGetRoomLastMatch.mockRejectedValue(new FetchError(404, "NOT_FOUND", "not found"));
  mockGetFriendshipStatus.mockReset();
  mockGetFriendshipStatus.mockResolvedValue({ status: "none", requestId: null });
  routeParams.id = "1";
  Element.prototype.scrollIntoView = vi.fn();
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  useAuthStore.setState({ user: null, token: null });
  useChatStore.setState({ lobbyMessages: [], matchMessages: [], roomMessages: [] });
});

describe("RoomPage", () => {
  it("renders room name and configuration", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByText("Test Room")).toBeInTheDocument();
    });

    expect(screen.getByTestId("room-page")).toBeInTheDocument();
    expect(screen.getByText(/Bitola/)).toBeInTheDocument();
    expect(screen.getByText(/1001 pts/)).toBeInTheDocument();
    expect(screen.getByText(/Relaxed/)).toBeInTheDocument();
  });

  it("draws a seated player's picture on their seat tile", async () => {
    const url = "https://assets.test/avatars/bob/128.webp";
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 2 },
        players: [
          makeRoomPlayer({ id: 1, userId: 10, username: "alice", seat: 0, team: "teamA" }),
          makeRoomPlayer({
            id: 2,
            userId: 20,
            username: "bob",
            seat: 1,
            team: "teamB",
            avatarUrl: url,
          }),
        ],
      }),
    );

    renderRoomPage();

    const bobSeat = await screen.findByTestId("player-seat-1");
    expect(bobSeat.querySelector("img")).toHaveAttribute("src", url);
    expect(screen.getByTestId("player-seat-0").querySelector("img")).toBeNull();
  });

  it("displays occupied seats with player usernames", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 2 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 2,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: 1,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    });

    expect(within(screen.getByTestId("player-seat-0")).getByText("alice")).toBeInTheDocument();
    expect(within(screen.getByTestId("player-seat-1")).getByText("bob")).toBeInTheDocument();
  });

  it("displays empty seats with take-a-seat prompts", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    });

    expect(screen.getAllByText("Take this seat")).toHaveLength(3);
  });

  it("highlights current user seat with You indicator", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByText("You")).toBeInTheDocument();
    });
  });

  it("calls clipboard API on copy link click", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: writeTextMock },
      writable: true,
      configurable: true,
    });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("copy-link")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("copy-link"));

    expect(writeTextMock).toHaveBeenCalledWith("XYZ123");
    expect(toast.success).toHaveBeenCalled();
  });

  it("calls leaveRoom and navigates to lobby on leave click", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockLeaveRoom.mockResolvedValue(undefined);

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("leave-room")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("leave-room"));

    expect(mockLeaveRoom).toHaveBeenCalledWith(1);
    expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
  });

  it("auto-joins when arriving via direct URL as a non-member (deep link)", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, ownerId: 20, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );
    mockJoinRoom.mockResolvedValue({ ...defaultRoom, ownerId: 20, playerCount: 2 });

    renderRoomPage();

    await waitFor(() => {
      expect(mockJoinRoom).toHaveBeenCalledWith(1, undefined);
    });
  });

  it("does not auto-join when the viewer is already a member", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("room-page")).toBeInTheDocument();
    });
    expect(mockJoinRoom).not.toHaveBeenCalled();
  });

  it("prompts for a password instead of blind-joining on a deep link to a private room", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, ownerId: 20, playerCount: 1, isPrivate: true },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );
    mockJoinRoom.mockResolvedValue({
      ...defaultRoom,
      ownerId: 20,
      playerCount: 2,
      isPrivate: true,
    });

    const user = userEvent.setup();
    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("password-prompt-dialog")).toBeInTheDocument();
    });
    expect(mockJoinRoom).not.toHaveBeenCalled();

    await user.type(screen.getByTestId("password-prompt-input"), "sesame");
    await user.click(screen.getByTestId("password-prompt-submit"));

    await waitFor(() => {
      expect(mockJoinRoom).toHaveBeenCalledWith(1, "sesame");
    });
  });

  const bobSeated = {
    id: 1,
    roomId: 1,
    userId: 20,
    username: "bob",
    avatarUrl: null,
    seat: 0,
    team: "teamA",
    isBot: false,
    createdAt: "",
  };

  it("toasts the composed honor message and bounces to the lobby when the public auto-join is gated", async () => {
    useAuthStore.setState({
      user: makeUser({ id: 10, username: "alice", honorScore: 42, isNewPlayer: false }),
      token: "tok",
    });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, ownerId: 20, playerCount: 1, minHonor: 85 },
        players: [bobSeated],
      }),
    );
    mockJoinRoom.mockRejectedValue(new FetchError(409, "HONOR_TOO_LOW", "honor too low"));

    renderRoomPage();

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(1, undefined));
    await waitFor(() => expect(toast.error).toHaveBeenCalled());

    const msg = vi.mocked(toast.error).mock.calls.at(-1)?.[0] as string;
    expect(msg).toContain("85");
    expect(msg).toContain("42");
    expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
  });

  it("toasts the veterans-only message when the public auto-join hits the newcomer gate", async () => {
    useAuthStore.setState({
      user: makeUser({ id: 10, username: "alice", isNewPlayer: true }),
      token: "tok",
    });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, ownerId: 20, playerCount: 1, allowNewPlayers: false },
        players: [bobSeated],
      }),
    );
    mockJoinRoom.mockRejectedValue(
      new FetchError(409, "NEW_PLAYER_NOT_ALLOWED", "new players not allowed"),
    );

    renderRoomPage();

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(1, undefined));
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(i18n.t("room.errors.newPlayerNotAllowed")),
    );
    expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
  });

  it("surfaces the honor gate on the private deep-link path too, not just the public one", async () => {
    useAuthStore.setState({
      user: makeUser({ id: 10, username: "alice", honorScore: 30, isNewPlayer: false }),
      token: "tok",
    });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, ownerId: 20, playerCount: 1, isPrivate: true, minHonor: 90 },
        players: [bobSeated],
      }),
    );
    mockJoinRoom.mockRejectedValue(new FetchError(409, "HONOR_TOO_LOW", "honor too low"));

    const user = userEvent.setup();
    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("password-prompt-dialog")).toBeInTheDocument());
    await user.type(screen.getByTestId("password-prompt-input"), "sesame");
    await user.click(screen.getByTestId("password-prompt-submit"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(1, "sesame"));
    await waitFor(() => expect(toast.error).toHaveBeenCalled());

    const msg = vi.mocked(toast.error).mock.calls.at(-1)?.[0] as string;
    expect(msg).toContain("90");
    expect(msg).toContain("30");
    expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
  });

  it("surfaces the newcomer gate on the private deep-link path too", async () => {
    useAuthStore.setState({
      user: makeUser({ id: 10, username: "alice", isNewPlayer: true }),
      token: "tok",
    });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: {
          ...defaultRoom,
          ownerId: 20,
          playerCount: 1,
          isPrivate: true,
          allowNewPlayers: false,
        },
        players: [bobSeated],
      }),
    );
    mockJoinRoom.mockRejectedValue(
      new FetchError(409, "NEW_PLAYER_NOT_ALLOWED", "new players not allowed"),
    );

    const user = userEvent.setup();
    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("password-prompt-dialog")).toBeInTheDocument());
    await user.type(screen.getByTestId("password-prompt-input"), "sesame");
    await user.click(screen.getByTestId("password-prompt-submit"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(1, "sesame"));
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(i18n.t("room.errors.newPlayerNotAllowed")),
    );
    expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
  });

  it("does not re-prompt for the password when a stale cache omits a just-joined member", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    const privateRoom = { ...defaultRoom, ownerId: 20, isPrivate: true, playerCount: 2 };
    const bob = {
      id: 1,
      roomId: 1,
      userId: 20,
      username: "bob",
      avatarUrl: null,
      seat: 0,
      team: "teamA",
      isBot: false,
      createdAt: "",
    };
    const aliceMember = {
      id: 2,
      roomId: 1,
      userId: 10,
      username: "alice",
      avatarUrl: null,
      seat: null,
      team: null,
      isBot: false,
      createdAt: "",
    };

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    client.setQueryData(queryKeys.rooms.detail(1), {
      room: privateRoom,
      players: [bob],
      returnedUserIds: [],
    });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: privateRoom,
        players: [bob, aliceMember],
        returnedUserIds: [],
      }),
    );

    render(
      <QueryClientProvider client={client}>
        <BrowserRouter>
          <RoomPage />
        </BrowserRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => expect(mockGetRoom).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByTestId("room-page")).toBeInTheDocument());

    expect(screen.queryByTestId("password-prompt-dialog")).not.toBeInTheDocument();
    expect(mockJoinRoom).not.toHaveBeenCalled();
  });

  it("sends a keepalive leave on hard page unload (direct URL navigation) when a member", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    const fetchSpy = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("fetch", fetchSpy);

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("room-page")).toBeInTheDocument();
    });

    window.dispatchEvent(new Event("pagehide"));

    expect(fetchSpy).toHaveBeenCalledWith(
      expect.stringContaining("/rooms/1/leave"),
      expect.objectContaining({ method: "POST", keepalive: true }),
    );

    vi.unstubAllGlobals();
  });

  it("displays loading skeleton while fetching", () => {
    mockGetRoom.mockReturnValue(new Promise(() => {}));

    renderRoomPage();

    expect(screen.getByTestId("room-page-loading")).toBeInTheDocument();
  });

  it("displays error state for non-existent room", async () => {
    mockGetRoom.mockRejectedValue(new Error("not found"));

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("room-page-error")).toBeInTheDocument();
    });

    expect(screen.getByText("Room not found")).toBeInTheDocument();
  });

  it("renders Us and Them legend labels once seated", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByText("Us")).toBeInTheDocument();
    });
    expect(screen.getByText("Them")).toBeInTheDocument();
  });

  it("calls selectSeat API when clicking an empty seat", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 2 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 2,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: null,
            team: null,
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    mockSelectSeat.mockResolvedValue({
      matchStarted: false,
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          avatarUrl: null,
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
          avatarUrl: null,
          seat: 1,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-1"));

    expect(mockSelectSeat).toHaveBeenCalledWith(1, 1);
  });

  it("does not call selectSeat when clicking seat occupied by another player", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 2 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 2,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: 1,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-1"));

    expect(mockSelectSeat).not.toHaveBeenCalled();
  });

  it("calls selectSeat when clicking own occupied seat (for switching)", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    mockSelectSeat.mockResolvedValue({
      matchStarted: false,
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          avatarUrl: null,
          seat: 1,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-1"));

    expect(mockSelectSeat).toHaveBeenCalledWith(1, 1);
  });

  it("shows toast when seat selection fails", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    mockSelectSeat.mockRejectedValue(new FetchError(409, "SEAT_TAKEN", "seat is taken"));

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-1"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled();
    });
  });

  it("renders Start Game button for room owner", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 4 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 2,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: 1,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
          {
            id: 3,
            roomId: 1,
            userId: 30,
            username: "carol",
            avatarUrl: null,
            seat: 2,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 4,
            roomId: 1,
            userId: 40,
            username: "dave",
            avatarUrl: null,
            seat: 3,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("start-game")).toBeInTheDocument();
    });

    expect(screen.getByTestId("start-game")).toBeEnabled();
  });

  it("renders disabled Start Game button when fewer than 4 players seated", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 2 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 2,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: null,
            team: null,
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("start-game")).toBeInTheDocument();
    });

    expect(screen.getByTestId("start-game")).toBeDisabled();
  });

  it("disables Start and flags absent seats until every seated human has returned", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    const fourSeated = [
      {
        id: 1,
        roomId: 1,
        userId: 10,
        username: "alice",
        avatarUrl: null,
        seat: 0,
        team: "teamA",
        isBot: false,
        createdAt: "",
      },
      {
        id: 2,
        roomId: 1,
        userId: 20,
        username: "bob",
        avatarUrl: null,
        seat: 1,
        team: "teamB",
        isBot: false,
        createdAt: "",
      },
      {
        id: 3,
        roomId: 1,
        userId: 30,
        username: "carol",
        avatarUrl: null,
        seat: 2,
        team: "teamA",
        isBot: false,
        createdAt: "",
      },
      {
        id: 4,
        roomId: 1,
        userId: 40,
        username: "dave",
        avatarUrl: null,
        seat: 3,
        team: "teamB",
        isBot: false,
        createdAt: "",
      },
    ];

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 4 },
        players: fourSeated,
        returnedUserIds: [10, 20, 30],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("start-game")).toBeInTheDocument();
    });
    expect(screen.getByTestId("start-game")).toBeDisabled();
    expect(screen.getByTestId("waiting-to-return-3")).toBeInTheDocument();
    expect(screen.queryByTestId("waiting-to-return-0")).not.toBeInTheDocument();
  });

  it("enables Start once every seated human has returned", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 4 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 2,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: 1,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
          {
            id: 3,
            roomId: 1,
            userId: 30,
            username: "carol",
            avatarUrl: null,
            seat: 2,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 4,
            roomId: 1,
            userId: 40,
            username: "dave",
            avatarUrl: null,
            seat: 3,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
        ],
        returnedUserIds: [10, 20, 30, 40],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("start-game")).toBeInTheDocument();
    });
    expect(screen.getByTestId("start-game")).toBeEnabled();
    expect(screen.queryByTestId("waiting-to-return-3")).not.toBeInTheDocument();
  });

  it("calls startMatch API and navigates on success", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 4 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 2,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: 1,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
          {
            id: 3,
            roomId: 1,
            userId: 30,
            username: "carol",
            avatarUrl: null,
            seat: 2,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 4,
            roomId: 1,
            userId: 40,
            username: "dave",
            avatarUrl: null,
            seat: 3,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    mockStartGame.mockResolvedValue({
      ...defaultRoom,
      status: "in_progress",
    });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("start-game")).toBeEnabled();
    });

    await user.click(screen.getByTestId("start-game"));

    expect(mockStartGame).toHaveBeenCalledWith(1);
    expect(mockNavigate).toHaveBeenCalledWith("/match/1", {
      replace: true,
      state: { fromRoom: true },
    });
  });

  it("renders waiting message for non-owner when 4 players seated", async () => {
    useAuthStore.setState({
      user: makeUser({
        id: 20,
        username: "bob",
        email: "b@b.com",
        loginStreakDays: 1,
        createdAt: "",
      }),
      token: "tok",
    });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 4 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 2,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: 1,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
          {
            id: 3,
            roomId: 1,
            userId: 30,
            username: "carol",
            avatarUrl: null,
            seat: 2,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 4,
            roomId: 1,
            userId: 40,
            username: "dave",
            avatarUrl: null,
            seat: 3,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("room-cta")).toBeInTheDocument();
    });

    expect(screen.getByTestId("room-cta")).toHaveTextContent(/Waiting for host to start the match/);
    expect(screen.queryByTestId("start-game")).not.toBeInTheDocument();
  });

  it("does not render Start Game button for non-owner", async () => {
    useAuthStore.setState({
      user: makeUser({
        id: 20,
        username: "bob",
        email: "b@b.com",
        loginStreakDays: 1,
        createdAt: "",
      }),
      token: "tok",
    });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 2 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 2,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: 1,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByText("bob")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("start-game")).not.toBeInTheDocument();
  });

  it("shows toast when game start fails", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 4 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 2,
            roomId: 1,
            userId: 20,
            username: "bob",
            avatarUrl: null,
            seat: 1,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
          {
            id: 3,
            roomId: 1,
            userId: 30,
            username: "carol",
            avatarUrl: null,
            seat: 2,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
          {
            id: 4,
            roomId: 1,
            userId: 40,
            username: "dave",
            avatarUrl: null,
            seat: 3,
            team: "teamB",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    mockStartGame.mockRejectedValue(new FetchError(400, "NOT_ALL_SEATED", "not all seated"));

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("start-game")).toBeEnabled();
    });

    await user.click(screen.getByTestId("start-game"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled();
    });
  });

  it("renders the room chat panel once the room is loaded", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("room-page")).toBeInTheDocument();
    });
    expect(screen.getByTestId("room-chat-dock")).toBeInTheDocument();
  });

  it("clears chatStore.roomMessages on unmount", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    useChatStore.setState({
      roomMessages: [
        {
          userId: 20,
          username: "bob",
          message: "hi",
          timestamp: "2026-04-22T10:00:00Z",
          scope: "room",
        },
      ],
    });

    const { unmount } = render(
      <QueryWrapper>
        <BrowserRouter>
          <RoomPage />
        </BrowserRouter>
      </QueryWrapper>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("room-page")).toBeInTheDocument();
    });

    unmount();

    expect(useChatStore.getState().roomMessages).toHaveLength(0);
  });

  function fourSeatedRoomQuery(ownerSeat = 0) {
    return makeRoomDetail({
      room: { ...defaultRoom, ownerId: 10, playerCount: 4 },
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          avatarUrl: null,
          seat: ownerSeat,
          team: ownerSeat % 2 === 0 ? "teamA" : "teamB",
          isBot: false,
          createdAt: "",
        },
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
          avatarUrl: null,
          seat: 1,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
        {
          id: 3,
          roomId: 1,
          userId: 30,
          username: "carol",
          avatarUrl: null,
          seat: 2,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
        {
          id: 4,
          roomId: 1,
          userId: 40,
          username: "dave",
          avatarUrl: null,
          seat: 3,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    });
  }

  it("renders kick icons only for owner on non-owner seated tiles in waiting status", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("kick-player-0")).not.toBeInTheDocument();
    expect(screen.getByTestId("kick-player-1")).toBeInTheDocument();
    expect(screen.getByTestId("kick-player-2")).toBeInTheDocument();
    expect(screen.getByTestId("kick-player-3")).toBeInTheDocument();
  });

  it("hides kick icons for non-owner viewers", async () => {
    useAuthStore.setState({
      user: { ...defaultUser, id: 20, username: "bob" },
      token: "tok",
    });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("kick-player-0")).not.toBeInTheDocument();
    expect(screen.queryByTestId("kick-player-1")).not.toBeInTheDocument();
    expect(screen.queryByTestId("kick-player-2")).not.toBeInTheDocument();
    expect(screen.queryByTestId("kick-player-3")).not.toBeInTheDocument();
  });

  it("hides kick icons once room status transitions away from waiting", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    const data = fourSeatedRoomQuery();
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        ...data,
        room: { ...data.room, status: "playing" },
      }),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("kick-player-1")).not.toBeInTheDocument();
    expect(screen.queryByTestId("kick-player-2")).not.toBeInTheDocument();
    expect(screen.queryByTestId("kick-player-3")).not.toBeInTheDocument();
  });

  it("opens kick confirmation dialog and fires kick mutation on confirm", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());
    mockKickPlayer.mockResolvedValue({ playerCount: 3 });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("kick-player-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("kick-player-1"));

    await waitFor(() => {
      expect(screen.getByTestId("kick-confirm")).toBeInTheDocument();
    });
    expect(screen.getByTestId("kick-dialog-title")).toHaveTextContent("Kick bob");

    await user.click(screen.getByTestId("kick-confirm"));

    expect(mockKickPlayer).toHaveBeenCalledWith(1, 20);
  });

  it("cancels kick on dialog cancel — no API call", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("kick-player-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("kick-player-1"));

    await waitFor(() => {
      expect(screen.getByTestId("kick-cancel")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("kick-cancel"));

    expect(mockKickPlayer).not.toHaveBeenCalled();
  });

  it("enters swap mode on first seated-seat click and swaps on second click", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());
    mockSwapSeats.mockResolvedValue({
      players: fourSeatedRoomQuery().players,
    });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-1"));
    await waitFor(() => {
      expect(screen.getByTestId("swap-mode-banner")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-3"));

    expect(mockSwapSeats).toHaveBeenCalledWith(1, 1, 3);
  });

  it("fires the swap when target seat is empty (move-to-empty)", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    const partial = makeRoomDetail({
      room: { ...defaultRoom, ownerId: 10, playerCount: 2 },
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          avatarUrl: null,
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
          avatarUrl: null,
          seat: 1,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    });
    mockGetRoom.mockResolvedValue(partial);
    mockSwapSeats.mockResolvedValue({ players: partial.players });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-1"));
    await waitFor(() => {
      expect(screen.getByTestId("swap-mode-banner")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-2"));

    expect(mockSwapSeats).toHaveBeenCalledWith(1, 1, 2);
  });

  it("cancels swap mode when clicking the source tile again", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-1"));
    await waitFor(() => {
      expect(screen.getByTestId("swap-mode-banner")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-1"));

    expect(mockSwapSeats).not.toHaveBeenCalled();
  });

  it("non-owner clicking their own seat leaves the seat (non-quick-play)", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: { ...defaultUser, id: 20, username: "bob" },
      token: "tok",
    });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());
    mockLeaveSeat.mockResolvedValue({ players: fourSeatedRoomQuery().players });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-1"));

    expect(mockLeaveSeat).toHaveBeenCalledWith(1);
    expect(mockSelectSeat).not.toHaveBeenCalled();
  });

  it("owner clicking their own seat does not unseat them", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-0"));
    expect(mockLeaveSeat).not.toHaveBeenCalled();
  });

  it("shows error toast on 409 OWNER_CANNOT_LEAVE_SEAT from leave-seat", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: { ...defaultUser, id: 20, username: "bob" },
      token: "tok",
    });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());
    mockLeaveSeat.mockRejectedValue(
      new FetchError(409, "OWNER_CANNOT_LEAVE_SEAT", "owner cannot leave seat"),
    );

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("player-seat-1"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(expect.stringContaining("host"));
    });
  });

  function partialRoomQuery() {
    return makeRoomDetail({
      room: { ...defaultRoom, ownerId: 10, playerCount: 3 },
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
          avatarUrl: null,
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
          avatarUrl: null,
          seat: 1,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
        {
          id: 3,
          roomId: 1,
          userId: 30,
          username: "carol",
          avatarUrl: null,
          seat: null,
          team: null,
          isBot: false,
          createdAt: "",
        },
      ],
    });
  }

  it("owner can kick an unseated player from the in-room dropdown", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(partialRoomQuery());
    mockKickPlayer.mockResolvedValue({ playerCount: 2 });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("in-room-count")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("in-room-count"));

    const kickFromList = await screen.findByTestId("kick-list-30");
    await user.click(kickFromList);

    await waitFor(() => {
      expect(screen.getByTestId("kick-confirm")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("kick-confirm"));

    expect(mockKickPlayer).toHaveBeenCalledWith(1, 30);
  });

  it("hides kick-from-list affordance for non-owner viewers", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: { ...defaultUser, id: 20, username: "bob" },
      token: "tok",
    });
    mockGetRoom.mockResolvedValue(partialRoomQuery());

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("in-room-count")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("in-room-count"));

    expect(screen.queryByTestId("kick-list-30")).not.toBeInTheDocument();
    expect(screen.queryByTestId("kick-list-10")).not.toBeInTheDocument();
  });

  it("transfer-ownership confirm fires the API", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());
    mockTransferOwnership.mockResolvedValue({ ownerId: 20 });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("in-room-count")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("in-room-count"));

    await user.click(await screen.findByTestId("promote-player-20"));
    await user.click(await screen.findByTestId("transfer-confirm"));

    expect(mockTransferOwnership).toHaveBeenCalledWith(1, 20);
  });

  it("transfer-ownership cancel closes the dialog without firing the API", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("in-room-count")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("in-room-count"));

    await user.click(await screen.findByTestId("promote-player-20"));
    await user.click(await screen.findByTestId("transfer-cancel"));

    expect(mockTransferOwnership).not.toHaveBeenCalled();
  });

  it("does not render promote affordance for unseated targets", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(partialRoomQuery());

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("in-room-count")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("in-room-count"));

    expect(await screen.findByTestId("promote-player-20")).toBeInTheDocument();
    expect(screen.queryByTestId("promote-player-30")).not.toBeInTheDocument();
  });

  it("shows error toast on 409 ROOM_NOT_WAITING from kick", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());
    mockKickPlayer.mockRejectedValue(new FetchError(409, "ROOM_NOT_WAITING", "room not waiting"));

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("kick-player-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("kick-player-1"));
    await waitFor(() => {
      expect(screen.getByTestId("kick-confirm")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("kick-confirm"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        expect.stringContaining("match has already started"),
      );
    });
  });

  it("fires kickedToast and navigates to /lobby when kickedFromRoomId matches", async () => {
    useAuthStore.setState({
      user: { ...defaultUser, id: 20, username: "bob" },
      token: "tok",
    });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("room-page")).toBeInTheDocument();
    });

    const { useRoomStore } = await import("@/shared/stores/roomStore");
    act(() => {
      useRoomStore.getState().setKickedFromRoom(1);
    });

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        expect.stringContaining("removed from room Test Room"),
      );
    });
    expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
    expect(useRoomStore.getState().kickedFromRoomId).toBeNull();
  });

  it("redirects a quick-play room to the matchmaking screen without leaving the queue", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, isQuickPlay: true, playerCount: 1 },
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            avatarUrl: null,
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
      }),
    );

    const { unmount } = render(
      <QueryWrapper>
        <BrowserRouter>
          <RoomPage />
        </BrowserRouter>
      </QueryWrapper>,
    );

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/matchmaking/1", { replace: true });
    });

    expect(screen.queryByTestId("room-page")).not.toBeInTheDocument();

    unmount();
    expect(mockLeaveRoom).not.toHaveBeenCalled();
  });
});

describe("RoomPage honour", () => {
  const seat = (
    userId: number,
    username: string,
    s: number,
    honor?: Partial<Pick<RoomPlayer, "honorScore" | "honorTier" | "level">>,
  ): RoomPlayer => ({
    id: userId,
    roomId: 1,
    userId,
    username,
    avatarUrl: null,
    seat: s,
    team: s % 2 === 0 ? "teamA" : "teamB",
    isBot: false,
    createdAt: "",
    ...honor,
  });

  it("shows the gate in the badges row, and not at all when ungated", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1, minHonor: 85, allowNewPlayers: false },
        players: [seat(10, "alice", 0)],
      }),
    );

    renderRoomPage();

    await waitFor(() =>
      expect(screen.getByTestId("badge-min-honor")).toHaveAttribute("data-min-honor", "85"),
    );
    expect(screen.getByTestId("badge-veterans-only")).toBeInTheDocument();
  });

  it("names every standard match rule on the Rules badge, absent fields included", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [seat(10, "alice", 0)],
      }),
    );

    renderRoomPage();

    const trigger = await screen.findByTestId("room-rules-trigger");
    expect(trigger).toHaveTextContent("Rules");
    expect(trigger).toHaveAccessibleName(
      "Rules: Direction of play: Counter-clockwise, Declarations: On, Declarations on a fall: To the winners, Match end: Finish the hand",
    );
  });

  it.each([
    [{ declarationsEnabled: false }, "Declarations: Off"],
    [{ stopAtTarget: true }, "Match end: Stop at target"],
    [{ declarationsTransfer: false }, "Declarations on a fall: Voided"],
    [{ clockwise: true }, "Direction of play: Clockwise"],
  ])("names a changed rule on the Rules badge (%o)", async (rule, named) => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1, ...rule },
        players: [seat(10, "alice", 0)],
      }),
    );

    renderRoomPage();

    const trigger = await screen.findByTestId("room-rules-trigger");
    expect(trigger).toHaveAccessibleName(new RegExp(named));
  });

  it("opens the match and room rules from the Rules badge", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1, clockwise: true, allowNewPlayers: false },
        players: [seat(10, "alice", 0)],
      }),
    );

    renderRoomPage();

    await user.click(await screen.findByTestId("room-rules-trigger"));

    const popup = await screen.findByTestId("room-rules-popup");
    expect(within(popup).getByTestId("rule-direction")).toHaveAttribute("data-changed", "true");
    expect(within(popup).getByTestId("rule-newPlayers")).toHaveTextContent("Veterans only");
  });

  it("renders no honour badges on an ungated room", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [seat(10, "alice", 0)],
      }),
    );

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("player-seat-0")).toBeInTheDocument());
    expect(screen.queryByTestId("badge-min-honor")).toBeNull();
    expect(screen.queryByTestId("badge-veterans-only")).toBeNull();
  });

  it("shows each seat its own tier, and rings only the seat under the room's bar", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 2, minHonor: 85 },
        players: [
          seat(10, "alice", 0, { honorScore: 96, honorTier: "exemplary" }),
          seat(20, "bob", 1, { honorScore: 71, honorTier: "fair" }),
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("seat-honor-0")).toBeInTheDocument());
    expect(screen.getByTestId("seat-honor-0")).toHaveAttribute("data-honor", "96");
    expect(screen.getByTestId("seat-honor-1")).toHaveAttribute("data-honor", "71");
    expect(within(screen.getByTestId("seat-honor-1")).getByTestId("honor-shield")).toHaveAttribute(
      "data-tier",
      "fair",
    );

    expect(screen.getByTestId("seat-position-east")).toHaveAttribute(
      "data-below-honor-bar",
      "true",
    );
    expect(screen.getByTestId("seat-position-south")).not.toHaveAttribute("data-below-honor-bar");
  });

  it("renders a real honour score of 0 rather than omitting it", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [seat(10, "alice", 0, { honorScore: 0, honorTier: "problematic" })],
      }),
    );

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("seat-honor-0")).toBeInTheDocument());
    expect(screen.getByTestId("seat-honor-0")).toHaveAttribute("data-honor", "0");
  });

  it("renders the seat's level before the honour", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [seat(10, "alice", 0, { level: 7, honorScore: 96, honorTier: "exemplary" })],
      }),
    );

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("seat-level-0")).toBeInTheDocument());
    expect(screen.getByTestId("seat-level-0")).toHaveAttribute("data-level", "7");
    expect(screen.getByTestId("seat-level-0")).toHaveTextContent("Lvl 7");
    const row = screen.getByTestId("seat-level-0").parentElement;
    expect(within(row as HTMLElement).getByTestId("seat-honor-0")).toBeInTheDocument();
    expect(row?.firstElementChild).toBe(screen.getByTestId("seat-level-0"));
  });

  it("renders a real level 0, and no level chip for a seat whose level did not arrive", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 2 },
        players: [
          seat(10, "alice", 0, { level: 0, honorScore: 80, honorTier: "fair" }),
          seat(20, "bob", 1, { honorScore: 80, honorTier: "fair" }),
        ],
      }),
    );

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("seat-level-0")).toBeInTheDocument());
    expect(screen.getByTestId("seat-level-0")).toHaveAttribute("data-level", "0");
    expect(screen.queryByTestId("seat-level-1")).toBeNull();
    expect(screen.getByTestId("seat-honor-1")).toBeInTheDocument();
  });

  it("shows no shield for a seat whose honour did not arrive", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1, minHonor: 85 },
        players: [seat(10, "alice", 0)],
      }),
    );

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("player-seat-0")).toBeInTheDocument());
    expect(screen.queryByTestId("seat-honor-0")).toBeNull();
    expect(screen.getByTestId("seat-position-south")).not.toHaveAttribute("data-below-honor-bar");
  });
});

describe("RoomPage last match", () => {
  const roomPlayer = (userId: number, username: string, s: number): RoomPlayer => ({
    id: userId,
    roomId: 1,
    userId,
    username,
    avatarUrl: null,
    seat: s,
    team: s % 2 === 0 ? "teamA" : "teamB",
    isBot: false,
    createdAt: "",
  });

  const lastMatch = {
    id: 42,
    variant: "bitola" as const,
    matchMode: "1001" as const,
    startedAt: "2026-08-01T12:00:00Z",
    completedAt: "2026-08-01T12:25:00Z",
    status: "completed",
    winnerTeam: 0,
    teamAScore: 1010,
    teamBScore: 640,
    hasBots: false,
    viewerSeat: 0,
    outcome: "win" as const,
    endReason: "natural" as const,
    players: [
      { seat: 0, userId: 10, username: "alice", isBot: false },
      { seat: 1, userId: 11, username: "bob", isBot: false },
      { seat: 2, userId: 12, username: "carol", isBot: false },
      { seat: 3, userId: 13, username: "dave", isBot: false },
    ],
    hands: [],
  };

  function seedRoom() {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [roomPlayer(10, "alice", 0)],
      }),
    );
  }

  it("hides the button when the room's last match 404s", async () => {
    seedRoom();
    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("action-bar")).toBeInTheDocument());
    await waitFor(() => expect(mockGetRoomLastMatch).toHaveBeenCalledWith(1));
    expect(screen.queryByTestId("open-last-match")).toBeNull();
  });

  it("shows the button once the match arrives and opens the dialog", async () => {
    seedRoom();
    mockGetRoomLastMatch.mockResolvedValue(lastMatch);
    renderRoomPage();

    const button = await screen.findByTestId("open-last-match");
    expect(screen.queryByTestId("last-match-dialog")).toBeNull();

    await userEvent.click(button);
    expect(await screen.findByTestId("last-match-dialog")).toBeInTheDocument();
  });

  it("hides Reinvite for a co-player who is already seated in the room", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 2 },
        players: [roomPlayer(10, "alice", 0), roomPlayer(11, "bob", 1)],
      }),
    );
    mockGetRoomLastMatch.mockResolvedValue(lastMatch);
    mockGetFriendshipStatus.mockResolvedValue({ status: "friends", requestId: 1 });
    renderRoomPage();

    await userEvent.click(await screen.findByTestId("open-last-match"));
    await screen.findByTestId("last-match-dialog");

    expect(await screen.findByTestId("match-player-reinvite-12")).toBeInTheDocument();
    expect(screen.queryByTestId("match-player-reinvite-11")).toBeNull();
  });

  it("does not read the last match for a malformed route id", async () => {
    routeParams.id = "abc";
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, playerCount: 1 },
        players: [roomPlayer(10, "alice", 0)],
      }),
    );
    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("action-bar")).toBeInTheDocument());
    expect(mockGetRoomLastMatch).not.toHaveBeenCalled();
    expect(screen.queryByTestId("open-last-match")).toBeNull();
  });

  it("drops the button when the room leaves waiting, even with the row still cached", async () => {
    seedRoom();
    mockGetRoomLastMatch.mockResolvedValue(lastMatch);
    renderRoomPage();

    await screen.findByTestId("open-last-match");

    const { useRoomStore } = await import("@/shared/stores/roomStore");
    act(() => {
      useRoomStore.getState().setRoom({ ...defaultRoom, status: "playing" });
    });

    expect(screen.queryByTestId("open-last-match")).toBeNull();
  });

  it("does not read the last match for a quick-play room", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, isQuickPlay: true, playerCount: 1 },
        players: [roomPlayer(10, "alice", 0)],
      }),
    );
    renderRoomPage();

    await waitFor(() => expect(mockNavigate).toHaveBeenCalled());
    expect(mockGetRoomLastMatch).not.toHaveBeenCalled();
  });

  it("does not read the last match while the room is playing", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: { ...defaultRoom, status: "playing", playerCount: 1 },
        players: [roomPlayer(10, "alice", 0)],
      }),
    );
    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("action-bar")).toBeInTheDocument());
    expect(mockGetRoomLastMatch).not.toHaveBeenCalled();
    expect(screen.queryByTestId("open-last-match")).toBeNull();
  });
});
