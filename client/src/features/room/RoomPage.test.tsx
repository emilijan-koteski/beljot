import "@/shared/i18n/i18n";

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { QueryWrapper } from "@/test-utils";

import { RoomPage } from "./RoomPage";

// Mock react-router useParams and useNavigate
const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useParams: () => ({ id: "1" }),
    useNavigate: () => mockNavigate,
  };
});

// Mock API
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

// Mock WebSocket connection state + sendMessage hook (ChatDock pulls both)
vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsConnectionState: () => "connected",
  useWsSendMessage: () => vi.fn(),
}));

// Mock sonner toast
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

const defaultRoom = {
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
  createdAt: "",
  updatedAt: "",
};

const defaultUser = {
  id: 10,
  username: "alice",
  email: "a@b.com",
  languagePreference: "en",
  walletBalance: 5000,
  loginStreakDays: 1,
  createdAt: "",
};

beforeEach(() => {
  mockLeaveRoom.mockResolvedValue(undefined);
  mockJoinRoom.mockResolvedValue(defaultRoom);
  // jsdom has no scrollIntoView — the nested ChatDock calls it on mount.
  Element.prototype.scrollIntoView = vi.fn();
  // jsdom has no matchMedia — RoomPage reads `prefers-reduced-motion` to
  // decide between the rotation transition + delayed "your seat" label and
  // the snap path. Default matches=false (animations allowed).
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
  vi.clearAllMocks();
  useAuthStore.setState({ user: null, token: null });
  useChatStore.setState({ lobbyMessages: [], matchMessages: [], roomMessages: [] });
});

describe("RoomPage", () => {
  it("renders room name and configuration", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByText("Test Room")).toBeInTheDocument();
    });

    expect(screen.getByTestId("room-page")).toBeInTheDocument();
    expect(screen.getByText(/Bitola/)).toBeInTheDocument();
    expect(screen.getByText(/1001 pts/)).toBeInTheDocument();
    expect(screen.getByText(/Relaxed/)).toBeInTheDocument();
  });

  it("displays occupied seats with player usernames", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 2 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
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

    // Names also surface in the team legend, so scope to the seat tiles.
    expect(within(screen.getByTestId("player-seat-0")).getByText("alice")).toBeInTheDocument();
    expect(within(screen.getByTestId("player-seat-1")).getByText("bob")).toBeInTheDocument();
  });

  it("displays empty seats with take-a-seat prompts", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    });

    // Three empty seats each show the "Take this seat" prompt.
    expect(screen.getAllByText("Take this seat")).toHaveLength(3);
  });

  it("highlights current user seat with You indicator", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("leave-room")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("leave-room"));

    expect(mockLeaveRoom).toHaveBeenCalledWith(1);
    expect(mockNavigate).toHaveBeenCalledWith("/lobby");
  });

  it("auto-joins when arriving via direct URL as a non-member (deep link)", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    // Viewer alice (id 10) is NOT among the players — she pasted the room URL
    // directly instead of going through the lobby's Join, so she never became
    // a member. Without auto-join the server rejects seat selection with
    // ErrNotInRoom and sends her no room WS events.
    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, ownerId: 20, playerCount: 1 },
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 20,
          username: "bob",
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
      ],
    });
    mockJoinRoom.mockResolvedValue({ ...defaultRoom, ownerId: 20, playerCount: 2 });

    renderRoomPage();

    await waitFor(() => {
      expect(mockJoinRoom).toHaveBeenCalledWith(1);
    });
  });

  it("does not auto-join when the viewer is already a member", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("room-page")).toBeInTheDocument();
    });
    expect(mockJoinRoom).not.toHaveBeenCalled();
  });

  it("sends a keepalive leave on hard page unload (direct URL navigation) when a member", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    const fetchSpy = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("fetch", fetchSpy);

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("room-page")).toBeInTheDocument();
    });

    // A hard navigation (typing /lobby in the address bar, refresh, tab close)
    // unloads the document — React never runs the unmount-cleanup leave, so the
    // leave must go out via a keepalive request fired on `pagehide`.
    window.dispatchEvent(new Event("pagehide"));

    expect(fetchSpy).toHaveBeenCalledWith(
      expect.stringContaining("/rooms/1/leave"),
      expect.objectContaining({ method: "POST", keepalive: true }),
    );

    vi.unstubAllGlobals();
  });

  it("displays loading skeleton while fetching", () => {
    mockGetRoom.mockReturnValue(new Promise(() => {})); // never resolves

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

  // --- Team color indicators ---

  it("renders Us and Them legend labels once seated", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByText("Us")).toBeInTheDocument();
    });
    expect(screen.getByText("Them")).toBeInTheDocument();
  });

  // --- Seat selection ---

  it("calls selectSeat API when clicking an empty seat", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 2 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
          seat: null,
          team: null,
          isBot: false,
          createdAt: "",
        },
      ],
    });

    mockSelectSeat.mockResolvedValue({
      matchStarted: false,
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
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

    // Seat 1 is empty — click it
    await user.click(screen.getByTestId("player-seat-1"));

    expect(mockSelectSeat).toHaveBeenCalledWith(1, 1);
  });

  it("does not call selectSeat when clicking seat occupied by another player", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 2 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
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

    // Seat 1 is occupied by bob — clicking it never fires a seat selection
    // (for the owner it enters swap mode instead; for others it's inert).
    await user.click(screen.getByTestId("player-seat-1"));

    expect(mockSelectSeat).not.toHaveBeenCalled();
  });

  it("calls selectSeat when clicking own occupied seat (for switching)", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    mockSelectSeat.mockResolvedValue({
      matchStarted: false,
      players: [
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

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    });

    // Click empty seat 1 to switch
    await user.click(screen.getByTestId("player-seat-1"));

    expect(mockSelectSeat).toHaveBeenCalledWith(1, 1);
  });

  it("shows toast when seat selection fails", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

  // --- Start Game ---

  it("renders Start Game button for room owner", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 4 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
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
          seat: 3,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("start-game")).toBeInTheDocument();
    });

    expect(screen.getByTestId("start-game")).toBeEnabled();
  });

  it("renders disabled Start Game button when fewer than 4 players seated", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 2 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
          seat: null,
          team: null,
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("start-game")).toBeInTheDocument();
    });

    expect(screen.getByTestId("start-game")).toBeDisabled();
  });

  // v2 presence gate: a reopened room with all four seats occupied but a
  // not-yet-returned human keeps Start disabled and flags the absent seat.
  it("disables Start and flags absent seats until every seated human has returned", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    const fourSeated = [
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
      {
        id: 2,
        roomId: 1,
        userId: 20,
        username: "bob",
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
        seat: 3,
        team: "teamB",
        isBot: false,
        createdAt: "",
      },
    ];

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 4 },
      players: fourSeated,
      returnedUserIds: [10, 20, 30], // dave (seat 3) is still away
    });

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

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 4 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
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
          seat: 3,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
      returnedUserIds: [10, 20, 30, 40],
    });

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

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 4 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
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
          seat: 3,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    });

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
    expect(mockNavigate).toHaveBeenCalledWith("/match/1", { state: { fromRoom: true } });
  });

  it("renders waiting message for non-owner when 4 players seated", async () => {
    useAuthStore.setState({
      user: {
        id: 20,
        username: "bob",
        email: "b@b.com",
        languagePreference: "en",
        walletBalance: 5000,
        loginStreakDays: 1,
        createdAt: "",
      },
      token: "tok",
    });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 4 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
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
          seat: 3,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("room-cta")).toBeInTheDocument();
    });

    expect(screen.getByTestId("room-cta")).toHaveTextContent(/Waiting for host to start the match/);
    expect(screen.queryByTestId("start-game")).not.toBeInTheDocument();
  });

  it("does not render Start Game button for non-owner", async () => {
    useAuthStore.setState({
      user: {
        id: 20,
        username: "bob",
        email: "b@b.com",
        languagePreference: "en",
        walletBalance: 5000,
        loginStreakDays: 1,
        createdAt: "",
      },
      token: "tok",
    });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 2 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
          seat: 1,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByText("bob")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("start-game")).not.toBeInTheDocument();
  });

  it("shows toast when game start fails", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 4 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
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
          seat: 3,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    });

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

  // --- Quick Play Room Behavior ---

  // NOTE: Quick Play rooms no longer render in RoomPage — they redirect to the
  // dedicated /matchmaking/:id screen (see the redirect test below and
  // MatchmakingPage.test.tsx). The former QP CTA/seat tests here moved with that
  // behaviour.

  // --- Room-scoped chat ---

  it("renders the room chat panel once the room is loaded", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("room-page")).toBeInTheDocument();
    });
    expect(screen.getByTestId("room-chat-dock")).toBeInTheDocument();
  });

  it("clears chatStore.roomMessages on unmount", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, playerCount: 1 },
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

    // Seed roomMessages to verify they get cleared
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

  // --- Owner pre-game controls (Story 8.1) ---

  function fourSeatedRoomQuery(ownerSeat = 0) {
    return {
      room: { ...defaultRoom, ownerId: 10, playerCount: 4 },
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "alice",
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
          seat: 3,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    };
  }

  it("renders kick icons only for owner on non-owner seated tiles in waiting status", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    mockGetRoom.mockResolvedValue(fourSeatedRoomQuery());

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    });

    // Owner's own tile (seat 0) does NOT render a kick icon
    expect(screen.queryByTestId("kick-player-0")).not.toBeInTheDocument();
    // Non-owner seated tiles (seats 1, 2, 3) all render a kick icon
    expect(screen.getByTestId("kick-player-1")).toBeInTheDocument();
    expect(screen.getByTestId("kick-player-2")).toBeInTheDocument();
    expect(screen.getByTestId("kick-player-3")).toBeInTheDocument();
  });

  it("hides kick icons for non-owner viewers", async () => {
    // Sign in as user 20 (bob), who is NOT the owner
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
    mockGetRoom.mockResolvedValue({
      ...data,
      room: { ...data.room, status: "playing" },
    });

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

    // Dialog renders with the target player's username embedded in the prompt
    await waitFor(() => {
      expect(screen.getByTestId("kick-confirm")).toBeInTheDocument();
    });
    expect(screen.getByTestId("kick-dialog-title")).toHaveTextContent("Kick bob from the room?");

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

    // First click on seat 1 (bob, non-owner) — enter swap mode
    await user.click(screen.getByTestId("player-seat-1"));
    await waitFor(() => {
      expect(screen.getByTestId("swap-mode-banner")).toBeInTheDocument();
    });

    // Second click on seat 3 (dave, non-owner) — fires the swap
    await user.click(screen.getByTestId("player-seat-3"));

    expect(mockSwapSeats).toHaveBeenCalledWith(1, 1, 3);
  });

  it("fires the swap when target seat is empty (move-to-empty)", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({ user: defaultUser, token: "tok" });
    // Owner alice at seat 0, bob at seat 1, seats 2 & 3 empty.
    const partial = {
      room: { ...defaultRoom, ownerId: 10, playerCount: 2 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
          seat: 1,
          team: "teamB",
          isBot: false,
          createdAt: "",
        },
      ],
    };
    mockGetRoom.mockResolvedValue(partial);
    mockSwapSeats.mockResolvedValue({ players: partial.players });

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("player-seat-1")).toBeInTheDocument();
    });

    // First click on seat 1 (bob, non-owner) — enter swap mode
    await user.click(screen.getByTestId("player-seat-1"));
    await waitFor(() => {
      expect(screen.getByTestId("swap-mode-banner")).toBeInTheDocument();
    });

    // Click empty seat 2 — should fire swap (move bob into seat 2)
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

    // First click — enter swap mode
    await user.click(screen.getByTestId("player-seat-1"));
    await waitFor(() => {
      expect(screen.getByTestId("swap-mode-banner")).toBeInTheDocument();
    });

    // Click the same seat again — cancels (no swap call)
    await user.click(screen.getByTestId("player-seat-1"));

    expect(mockSwapSeats).not.toHaveBeenCalled();
  });

  it("non-owner clicking their own seat leaves the seat (non-quick-play)", async () => {
    const user = userEvent.setup();
    // Sign in as bob (user 20), who sits at seat 1 in fourSeatedRoomQuery and is NOT the owner.
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

    // Clicking the own seat tile fires leaveSeat(roomId).
    await user.click(screen.getByTestId("player-seat-1"));

    expect(mockLeaveSeat).toHaveBeenCalledWith(1);
    // Self-click must NOT also fire selectSeat for the same seat.
    expect(mockSelectSeat).not.toHaveBeenCalled();
  });

  it("owner clicking their own seat does not unseat them", async () => {
    // alice is the owner at seat 0. The owner cannot unseat themselves —
    // their only exit is leave-room. Clicking the own tile must be a no-op.
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

  // --- In-room dropdown owner controls (kick from list, transfer ownership) ---

  function partialRoomQuery() {
    // alice (owner, seat 0), bob (seat 1), carol (in room but unseated).
    // Used to exercise the kick-from-list path on an unseated player.
    return {
      room: { ...defaultRoom, ownerId: 10, playerCount: 3 },
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
        {
          id: 2,
          roomId: 1,
          userId: 20,
          username: "bob",
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
          seat: null,
          team: null,
          isBot: false,
          createdAt: "",
        },
      ],
    };
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

    // Carol is unseated, so she has no diamond tile — only the dropdown row
    // can host the kick affordance for her.
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

    // Bob is not the owner — no kick affordance on any list row.
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

    // Bob is seated and not the owner — the promote affordance must render.
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

    // Bob is seated — affordance renders.
    expect(await screen.findByTestId("promote-player-20")).toBeInTheDocument();
    // Carol is unseated — affordance hidden so the owner can't try a server
    // round-trip that the backend would reject with CANNOT_PROMOTE_UNSEATED.
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

    // Wait for the seed effect to set currentRoomId, then simulate the
    // dispatcher setting kickedFromRoomId from a system:room_kicked event.
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
    expect(mockNavigate).toHaveBeenCalledWith("/lobby");
    // The flag is cleared so the effect doesn't re-fire on a future mount.
    expect(useRoomStore.getState().kickedFromRoomId).toBeNull();
  });

  it("redirects a quick-play room to the matchmaking screen without leaving the queue", async () => {
    useAuthStore.setState({ user: defaultUser, token: "tok" });

    mockGetRoom.mockResolvedValue({
      room: { ...defaultRoom, isQuickPlay: true, playerCount: 1 },
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

    // The seat grid never renders — this surface is for custom rooms only.
    expect(screen.queryByTestId("room-page")).not.toBeInTheDocument();

    // hasLeftRef is set before the redirect, so unmounting must NOT fire the
    // cleanup `/leave` that would eject the player from the queue.
    unmount();
    expect(mockLeaveRoom).not.toHaveBeenCalled();
  });
});
