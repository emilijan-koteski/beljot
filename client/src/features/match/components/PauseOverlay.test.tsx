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
        "match.pause.extend": `Add ${opts?.minutes ?? "?"} min`,
        "match.pause.extendedRow": `+${opts?.minutes ?? "?"} min`,
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

const NONE: [boolean, boolean, boolean, boolean] = [false, false, false, false];

function renderOverlay(overrides: Partial<PauseOverlayProps> = {}) {
  const props: PauseOverlayProps = {
    pausedPlayers: [true, false, false, false],
    pauseExtended: NONE,
    pauseExpiresAt: new Date(Date.now() + 90_000).toISOString(),
    pauseTotalSec: 120,
    players: mockPlayers,
    myPlayerSeat: 0,
    onResume: vi.fn(),
    onExtend: vi.fn(),
    ...overrides,
  };
  render(<PauseOverlay {...props} />);
  return props;
}

describe("PauseOverlay", () => {
  it("counts down to the automatic resume", () => {
    renderOverlay();
    expect(screen.getByTestId("pause-countdown")).toHaveTextContent(/^1:(29|30)$/);
    expect(screen.getByText("Until play resumes")).toBeInTheDocument();
  });

  it("marks who holds the pause and who added time", () => {
    renderOverlay({
      pausedPlayers: [true, false, true, false],
      pauseExtended: [false, false, true, false],
      pauseTotalSec: 240,
    });
    expect(screen.getByTestId("pause-player-0")).toHaveAttribute("data-holding", "true");
    expect(screen.getByTestId("pause-player-1")).toHaveAttribute("data-holding", "false");
    expect(screen.getByTestId("pause-player-2")).toHaveTextContent("+2 min");
    expect(screen.getByTestId("pause-player-2")).toHaveTextContent("Paused");
  });

  it("lets the player holding the pause resume it", () => {
    const props = renderOverlay();
    fireEvent.click(screen.getByTestId("pause-resume-button"));
    expect(props.onResume).toHaveBeenCalledTimes(1);
  });

  it("lets any player add time once per pause", () => {
    const props = renderOverlay({ myPlayerSeat: 1 });
    fireEvent.click(screen.getByRole("button", { name: "Add 2 min" }));
    expect(props.onExtend).toHaveBeenCalledTimes(1);
  });

  it("hides the add-time button once the player has used it", () => {
    renderOverlay({ myPlayerSeat: 1, pauseExtended: [false, true, false, false] });
    expect(screen.queryByTestId("pause-extend-button")).not.toBeInTheDocument();
  });

  it("asks a player who holds no pause to wait, with no way to resume for others", () => {
    renderOverlay({ myPlayerSeat: 2 });
    expect(screen.getByTestId("pause-waiting")).toHaveTextContent("Waiting to resume…");
    expect(screen.queryByTestId("pause-resume-button")).not.toBeInTheDocument();
    expect(screen.queryByText(/resume all/i)).not.toBeInTheDocument();
  });

  it("names the extension in whole minutes of one pause window", () => {
    renderOverlay({
      pauseExtended: [true, true, false, false],
      pauseTotalSec: 360,
      myPlayerSeat: 3,
    });
    expect(screen.getByTestId("pause-extend-button")).toHaveTextContent("Add 2 min");
  });
});
