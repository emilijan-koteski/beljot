import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PauseOverlay } from "./PauseOverlay";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: { minutes?: number }) => {
      const translations: Record<string, string> = {
        "match.pause.title": "Match paused",
        "match.pause.resume": "Resume",
        "match.pause.waitingToResume": "Waiting to resume…",
        "match.pause.pausedNow": "Paused",
        "match.pause.countdownLabel": "Until play resumes",
        "match.pause.pauseButton": `Pause ${opts?.minutes ?? "?"} min`,
      };
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
    avatarUrl: null,
    team: "teamA",
    declarations: [],
    connected: true,
    isBot: false,
    level: 1,
    faceDownCount: 0,
    handCount: 0,
    declarationAnswered: false,
  },
  {
    hand: [],
    seat: 1,
    userId: 2,
    username: "Bob",
    avatarUrl: null,
    team: "teamB",
    declarations: [],
    connected: true,
    isBot: false,
    level: 1,
    faceDownCount: 0,
    handCount: 0,
    declarationAnswered: false,
  },
  {
    hand: [],
    seat: 2,
    userId: 3,
    username: "Carol",
    avatarUrl: null,
    team: "teamA",
    declarations: [],
    connected: true,
    isBot: false,
    level: 1,
    faceDownCount: 0,
    handCount: 0,
    declarationAnswered: false,
  },
  {
    hand: [],
    seat: 3,
    userId: 4,
    username: "Dave",
    avatarUrl: null,
    team: "teamB",
    declarations: [],
    connected: true,
    isBot: false,
    level: 1,
    faceDownCount: 0,
    handCount: 0,
    declarationAnswered: false,
  },
] as PauseOverlayProps["players"];

type PauseOverlayProps = Parameters<typeof PauseOverlay>[0];

const NO_CLOCKS: [string | null, string | null, string | null, string | null] = [
  null,
  null,
  null,
  null,
];

function inSeconds(seconds: number): string {
  return new Date(Date.now() + seconds * 1000).toISOString();
}

function renderOverlay(overrides: Partial<PauseOverlayProps> = {}) {
  const props: PauseOverlayProps = {
    pausedPlayers: [true, false, false, false],
    pauseUsed: [true, false, false, false],
    playerPauseExpiresAt: [inSeconds(90), null, null, null],
    pauseWindowSec: 120,
    players: mockPlayers,
    myPlayerSeat: 0,
    onResume: vi.fn(),
    onPause: vi.fn(),
    ...overrides,
  };
  render(<PauseOverlay {...props} />);
  return props;
}

describe("PauseOverlay", () => {
  it("counts down to when play resumes", () => {
    renderOverlay();
    expect(screen.getByTestId("pause-countdown")).toHaveTextContent(/^1:(29|30)$/);
    expect(screen.getByText("Until play resumes")).toBeInTheDocument();
  });

  it("gives each running pause its own clock, and the ring the one that ends last", () => {
    renderOverlay({
      pausedPlayers: [true, false, true, false],
      pauseUsed: [true, false, true, false],
      playerPauseExpiresAt: [inSeconds(30), null, inSeconds(110), null],
    });
    expect(screen.getByTestId("pause-player-0-countdown")).toHaveTextContent(/^0:(29|30)$/);
    expect(screen.getByTestId("pause-player-2-countdown")).toHaveTextContent(/^1:(49|50)$/);
    expect(screen.getByTestId("pause-countdown")).toHaveTextContent(/^1:(49|50)$/);
    expect(screen.getByTestId("pause-player-1")).toHaveAttribute("data-holding", "false");
    expect(screen.queryByTestId("pause-player-1-countdown")).not.toBeInTheDocument();
  });

  it("lets a player end their own pause", () => {
    const props = renderOverlay();
    fireEvent.click(screen.getByTestId("pause-resume-button"));
    expect(props.onResume).toHaveBeenCalledTimes(1);
  });

  it("lets a player with an unspent pause start their own", () => {
    const props = renderOverlay({ myPlayerSeat: 1 });
    fireEvent.click(screen.getByRole("button", { name: "Pause 2 min" }));
    expect(props.onPause).toHaveBeenCalledTimes(1);
  });

  it("offers no second pause to a player who already used theirs", () => {
    renderOverlay();
    expect(screen.queryByTestId("pause-own-button")).not.toBeInTheDocument();
    expect(screen.getByTestId("pause-resume-button")).toBeInTheDocument();
  });

  it("gives a player whose pause is over no way to end someone else's", () => {
    renderOverlay({ myPlayerSeat: 2, pauseUsed: [true, false, true, false] });
    expect(screen.getByTestId("pause-waiting")).toHaveTextContent("Waiting to resume…");
    expect(screen.queryByTestId("pause-resume-button")).not.toBeInTheDocument();
    expect(screen.queryByTestId("pause-own-button")).not.toBeInTheDocument();
    expect(screen.queryByText(/resume all/i)).not.toBeInTheDocument();
  });

  it("hides the ring when no pause clock is known", () => {
    renderOverlay({ playerPauseExpiresAt: NO_CLOCKS });
    expect(screen.queryByTestId("pause-countdown")).not.toBeInTheDocument();
  });
});
