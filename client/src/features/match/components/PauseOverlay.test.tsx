import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PauseOverlay } from "./PauseOverlay";

// Mock react-i18next
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, string>) => {
      const translations: Record<string, string> = {
        "match.pause.title": "Game Paused",
        "match.pause.resume": "Resume",
        "match.pause.pauseButton": "Pause",
        "match.pause.waitingToResume": "Waiting for players to resume...",
        "match.pause.resumeAll": "Resume All",
      };
      if ((key === "match.pause.pausedBy" || key === "match.pause.pausedByMany") && opts?.player) {
        return `${opts.player} paused the game`;
      }
      return translations[key] ?? key;
    },
  }),
}));

const mockPlayers = [
  {
    hand: [],
    seat: 0,
    userId: 1,
    username: "Alice",
    team: "teamA",
    declarations: [],
    connected: true,
    isBot: false,
    level: 1,
  },
  {
    hand: [],
    seat: 1,
    userId: 2,
    username: "Bob",
    team: "teamB",
    declarations: [],
    connected: true,
    isBot: false,
    level: 1,
  },
  {
    hand: [],
    seat: 2,
    userId: 3,
    username: "Carol",
    team: "teamA",
    declarations: [],
    connected: true,
    isBot: false,
    level: 1,
  },
  {
    hand: [],
    seat: 3,
    userId: 4,
    username: "Dave",
    team: "teamB",
    declarations: [],
    connected: true,
    isBot: false,
    level: 1,
  },
] as PauseOverlayProps["players"];

type PauseOverlayProps = Parameters<typeof PauseOverlay>[0];

describe("PauseOverlay", () => {
  it("renders paused player names", () => {
    render(
      <PauseOverlay
        pausedPlayers={[true, false, false, false]}
        pauseUsed={[true, false, false, false]}
        players={mockPlayers}
        myPlayerSeat={1}
        isRoomOwner={false}
        onResume={vi.fn()}
        onPause={vi.fn()}
        onOwnerResume={vi.fn()}
      />,
    );
    expect(screen.getByText("Alice paused the game")).toBeInTheDocument();
    expect(screen.getByText("Game Paused")).toBeInTheDocument();
  });

  it("shows Resume button for player who paused", () => {
    render(
      <PauseOverlay
        pausedPlayers={[true, false, false, false]}
        pauseUsed={[true, false, false, false]}
        players={mockPlayers}
        myPlayerSeat={0}
        isRoomOwner={false}
        onResume={vi.fn()}
        onPause={vi.fn()}
        onOwnerResume={vi.fn()}
      />,
    );
    expect(screen.getByTestId("pause-resume-button")).toBeInTheDocument();
  });

  it("shows Pause button for player who can stack their pause", () => {
    render(
      <PauseOverlay
        pausedPlayers={[true, false, false, false]}
        pauseUsed={[true, false, false, false]}
        players={mockPlayers}
        myPlayerSeat={2}
        isRoomOwner={false}
        onResume={vi.fn()}
        onPause={vi.fn()}
        onOwnerResume={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("pause-resume-button")).not.toBeInTheDocument();
    expect(screen.getByTestId("pause-stack-button")).toBeInTheDocument();
  });

  it("shows waiting message for player who already used their pause", () => {
    render(
      <PauseOverlay
        pausedPlayers={[true, false, false, false]}
        pauseUsed={[true, false, true, false]}
        players={mockPlayers}
        myPlayerSeat={2}
        isRoomOwner={false}
        onResume={vi.fn()}
        onPause={vi.fn()}
        onOwnerResume={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("pause-resume-button")).not.toBeInTheDocument();
    expect(screen.queryByTestId("pause-stack-button")).not.toBeInTheDocument();
    expect(screen.getByTestId("pause-waiting")).toBeInTheDocument();
  });

  it("fires onResume on Resume button click", () => {
    const onResume = vi.fn();
    render(
      <PauseOverlay
        pausedPlayers={[true, false, false, false]}
        pauseUsed={[true, false, false, false]}
        players={mockPlayers}
        myPlayerSeat={0}
        isRoomOwner={false}
        onResume={onResume}
        onPause={vi.fn()}
        onOwnerResume={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByTestId("pause-resume-button"));
    expect(onResume).toHaveBeenCalledOnce();
  });

  it("fires onPause on stack Pause button click", () => {
    const onPause = vi.fn();
    render(
      <PauseOverlay
        pausedPlayers={[true, false, false, false]}
        pauseUsed={[true, false, false, false]}
        players={mockPlayers}
        myPlayerSeat={1}
        isRoomOwner={false}
        onResume={vi.fn()}
        onPause={onPause}
        onOwnerResume={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByTestId("pause-stack-button"));
    expect(onPause).toHaveBeenCalledOnce();
  });

  it("shows multiple paused players", () => {
    render(
      <PauseOverlay
        pausedPlayers={[true, false, true, false]}
        pauseUsed={[true, false, true, false]}
        players={mockPlayers}
        myPlayerSeat={1}
        isRoomOwner={false}
        onResume={vi.fn()}
        onPause={vi.fn()}
        onOwnerResume={vi.fn()}
      />,
    );
    expect(screen.getByTestId("pause-player-0")).toBeInTheDocument();
    expect(screen.getByTestId("pause-player-2")).toBeInTheDocument();
  });

  it("shows Resume All button for room owner", () => {
    render(
      <PauseOverlay
        pausedPlayers={[true, false, false, false]}
        pauseUsed={[true, false, false, false]}
        players={mockPlayers}
        myPlayerSeat={1}
        isRoomOwner={true}
        onResume={vi.fn()}
        onPause={vi.fn()}
        onOwnerResume={vi.fn()}
      />,
    );
    expect(screen.getByTestId("pause-owner-resume-button")).toBeInTheDocument();
    expect(screen.getByText("Resume All")).toBeInTheDocument();
  });

  it("does not show Resume All button for non-owner", () => {
    render(
      <PauseOverlay
        pausedPlayers={[true, false, false, false]}
        pauseUsed={[true, false, false, false]}
        players={mockPlayers}
        myPlayerSeat={1}
        isRoomOwner={false}
        onResume={vi.fn()}
        onPause={vi.fn()}
        onOwnerResume={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("pause-owner-resume-button")).not.toBeInTheDocument();
  });

  it("fires onOwnerResume on Resume All button click", () => {
    const onOwnerResume = vi.fn();
    render(
      <PauseOverlay
        pausedPlayers={[true, false, false, false]}
        pauseUsed={[true, false, false, false]}
        players={mockPlayers}
        myPlayerSeat={1}
        isRoomOwner={true}
        onResume={vi.fn()}
        onPause={vi.fn()}
        onOwnerResume={onOwnerResume}
      />,
    );
    fireEvent.click(screen.getByTestId("pause-owner-resume-button"));
    expect(onOwnerResume).toHaveBeenCalledOnce();
  });

  it("owner with active pause sees both Resume and Resume All buttons", () => {
    render(
      <PauseOverlay
        pausedPlayers={[true, false, false, false]}
        pauseUsed={[true, false, false, false]}
        players={mockPlayers}
        myPlayerSeat={0}
        isRoomOwner={true}
        onResume={vi.fn()}
        onPause={vi.fn()}
        onOwnerResume={vi.fn()}
      />,
    );
    expect(screen.getByTestId("pause-resume-button")).toBeInTheDocument();
    expect(screen.getByTestId("pause-owner-resume-button")).toBeInTheDocument();
  });
});
