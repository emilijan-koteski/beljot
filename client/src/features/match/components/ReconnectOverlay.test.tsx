import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ReconnectOverlay } from "./ReconnectOverlay";

// Mock react-i18next
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      const translations: Record<string, string> = {
        "match.disconnect.reconnecting": "Reconnecting...",
        "match.disconnect.countdownLabel": "Time remaining",
        "match.disconnect.returningToLobby": "Returning to lobby...",
        "match.disconnect.matchEnded": "Match has ended",
        "match.disconnect.reconnectFailed": "Reconnection failed — the match may have ended",
        "match.disconnect.abandonCountsWin": "Counts as a win for your team",
        "match.disconnect.abandonCountsLoss": "Counts as a loss for your team",
        "match.matchResult.title": "Match Complete",
        "team.us": "Us",
        "team.them": "Them",
      };
      if (key === "match.disconnect.waitingMessage" && opts?.player) {
        return `The game is on hold while we wait for ${opts.player} to reconnect.`;
      }
      if (key === "match.disconnect.matchAbandoned" && opts?.player) {
        return `${opts.player} disconnected — match ended`;
      }
      if (key === "match.disconnect.matchAbandonedScores" && opts) {
        return `Final: Us ${opts.us} : Them ${opts.them}`;
      }
      return translations[key] ?? key;
    },
  }),
}));

describe("ReconnectOverlay", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders player name and countdown", () => {
    const expiresAt = new Date(Date.now() + 120_000).toISOString(); // 2 minutes from now
    render(<ReconnectOverlay disconnectedPlayerName="Alice" reconnectExpiresAt={expiresAt} />);

    expect(screen.getByText("Reconnecting...")).toBeInTheDocument();
    expect(screen.getByText(/wait for Alice to reconnect/)).toBeInTheDocument();
    expect(screen.getByTestId("reconnect-countdown")).toBeInTheDocument();
    // Should show roughly 2:00
    expect(screen.getByTestId("reconnect-countdown").textContent).toBe("2:00");
  });

  it("countdown decrements over time", () => {
    const expiresAt = new Date(Date.now() + 65_000).toISOString(); // 65 seconds
    render(<ReconnectOverlay disconnectedPlayerName="Bob" reconnectExpiresAt={expiresAt} />);

    expect(screen.getByTestId("reconnect-countdown").textContent).toBe("1:05");

    // Advance 10 seconds. Ticks align to the deadline's whole-second
    // boundaries with a small slack (TICK_SLACK_MS), so cross the boundary
    // by a comfortable margin rather than landing exactly on it.
    act(() => {
      vi.advanceTimersByTime(10_100);
    });

    expect(screen.getByTestId("reconnect-countdown").textContent).toBe("0:55");
  });

  it("falls back to a 'match ended' panel when the window expired without an abandon event", () => {
    const expiresAt = new Date(Date.now() - 5000).toISOString(); // already expired
    render(<ReconnectOverlay disconnectedPlayerName="Carol" reconnectExpiresAt={expiresAt} />);

    // Countdown is replaced by the match-ended fallback view, so the user
    // isn't stuck on a 0:00 timer when the server already tore down the
    // session and the abandon broadcast never reached this client.
    expect(screen.queryByTestId("reconnect-countdown")).not.toBeInTheDocument();
    expect(screen.getByTestId("match-ended-title")).toBeInTheDocument();
    expect(screen.getByText("Returning to lobby...")).toBeInTheDocument();
  });

  it("auto-redirects to lobby when the reconnect window expires without an abandon event", () => {
    const onReturnToLobby = vi.fn();
    const expiresAt = new Date(Date.now() - 5000).toISOString();
    render(
      <ReconnectOverlay
        disconnectedPlayerName="Henry"
        reconnectExpiresAt={expiresAt}
        onReturnToLobby={onReturnToLobby}
      />,
    );

    expect(onReturnToLobby).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(onReturnToLobby).toHaveBeenCalledOnce();
  });

  it("has correct accessibility attributes", () => {
    const expiresAt = new Date(Date.now() + 60_000).toISOString();
    render(<ReconnectOverlay disconnectedPlayerName="Dave" reconnectExpiresAt={expiresAt} />);

    const overlay = screen.getByTestId("reconnect-overlay");
    expect(overlay).toHaveAttribute("aria-live", "assertive");
  });

  it("renders abandonment message viewer-relative when abandonedData is provided (viewer teamA)", () => {
    const expiresAt = new Date(Date.now() - 5000).toISOString();
    const abandonedData = {
      abandonedByPlayer: 2,
      teamAFinalScore: 450,
      teamBFinalScore: 380,
      matchDurationSec: 600,
    };

    render(
      <ReconnectOverlay
        disconnectedPlayerName="Eve"
        reconnectExpiresAt={expiresAt}
        abandonedData={abandonedData}
        viewerTeam="teamA"
        onReturnToLobby={vi.fn()}
      />,
    );

    expect(screen.getByTestId("abandon-title")).toHaveTextContent("Eve disconnected — match ended");
    // Viewer on teamA → Us=450 (a), Them=380 (b)
    expect(screen.getByTestId("abandon-scores")).toHaveTextContent("Final: Us 450 : Them 380");
    expect(screen.getByText("Returning to lobby...")).toBeInTheDocument();
    // Countdown should NOT be visible
    expect(screen.queryByTestId("reconnect-countdown")).not.toBeInTheDocument();
  });

  it("flips abandonment scores when viewer is on teamB", () => {
    const expiresAt = new Date(Date.now() - 5000).toISOString();
    const abandonedData = {
      abandonedByPlayer: 2,
      teamAFinalScore: 450,
      teamBFinalScore: 380,
      matchDurationSec: 600,
    };

    render(
      <ReconnectOverlay
        disconnectedPlayerName="Eve"
        reconnectExpiresAt={expiresAt}
        abandonedData={abandonedData}
        viewerTeam="teamB"
        onReturnToLobby={vi.fn()}
      />,
    );

    // Viewer on teamB → Us=380 (b), Them=450 (a)
    expect(screen.getByTestId("abandon-scores")).toHaveTextContent("Final: Us 380 : Them 450");
  });

  it("shows the loss result line when the viewer shares the abandoner's team", () => {
    const expiresAt = new Date(Date.now() - 5000).toISOString();
    // Abandoner seat 2 → team A; viewer seat 0 → team A → partner → loss.
    const abandonedData = {
      abandonedByPlayer: 2,
      teamAFinalScore: 450,
      teamBFinalScore: 380,
      matchDurationSec: 600,
    };

    render(
      <ReconnectOverlay
        disconnectedPlayerName="Eve"
        reconnectExpiresAt={expiresAt}
        abandonedData={abandonedData}
        viewerTeam="teamA"
        viewerSeat={0}
        onReturnToLobby={vi.fn()}
      />,
    );

    const line = screen.getByTestId("abandon-result-line");
    expect(line).toHaveAttribute("data-result", "loss");
    expect(line).toHaveTextContent("Counts as a loss for your team");
  });

  it("shows the win result line when the viewer is on the opposing team", () => {
    const expiresAt = new Date(Date.now() - 5000).toISOString();
    // Abandoner seat 1 → team B; viewer seat 0 → team A → opponent → win.
    const abandonedData = {
      abandonedByPlayer: 1,
      teamAFinalScore: 450,
      teamBFinalScore: 380,
      matchDurationSec: 600,
    };

    render(
      <ReconnectOverlay
        disconnectedPlayerName="Eve"
        reconnectExpiresAt={expiresAt}
        abandonedData={abandonedData}
        viewerTeam="teamA"
        viewerSeat={0}
        onReturnToLobby={vi.fn()}
      />,
    );

    const line = screen.getByTestId("abandon-result-line");
    expect(line).toHaveAttribute("data-result", "win");
    expect(line).toHaveTextContent("Counts as a win for your team");
  });

  it("flips the result line for a teamB viewer (seat parity, not raw team A/B)", () => {
    const expiresAt = new Date(Date.now() - 5000).toISOString();
    // Abandoner seat 0 → team A; viewer seat 1 → team B → opponents win.
    const abandonedData = {
      abandonedByPlayer: 0,
      teamAFinalScore: 450,
      teamBFinalScore: 380,
      matchDurationSec: 600,
    };

    render(
      <ReconnectOverlay
        disconnectedPlayerName="Eve"
        reconnectExpiresAt={expiresAt}
        abandonedData={abandonedData}
        viewerTeam="teamB"
        viewerSeat={1}
        onReturnToLobby={vi.fn()}
      />,
    );

    expect(screen.getByTestId("abandon-result-line")).toHaveAttribute("data-result", "win");
  });

  it("omits the result line when the viewer IS the abandoner (late reconnect)", () => {
    const expiresAt = new Date(Date.now() - 5000).toISOString();
    // Viewer seat 2 abandoned — their record says "abandoned", not "loss",
    // so no win/loss line even though the rest of the panel renders.
    const abandonedData = {
      abandonedByPlayer: 2,
      teamAFinalScore: 450,
      teamBFinalScore: 380,
      matchDurationSec: 600,
    };

    render(
      <ReconnectOverlay
        disconnectedPlayerName="Eve"
        reconnectExpiresAt={expiresAt}
        abandonedData={abandonedData}
        viewerTeam="teamA"
        viewerSeat={2}
        onReturnToLobby={vi.fn()}
      />,
    );

    expect(screen.getByTestId("abandon-title")).toBeInTheDocument();
    expect(screen.queryByTestId("abandon-result-line")).not.toBeInTheDocument();
  });

  it("omits the result line when abandonedByPlayer is out of range", () => {
    const expiresAt = new Date(Date.now() - 5000).toISOString();
    const abandonedData = {
      abandonedByPlayer: 7,
      teamAFinalScore: 450,
      teamBFinalScore: 380,
      matchDurationSec: 600,
    };

    render(
      <ReconnectOverlay
        disconnectedPlayerName="Eve"
        reconnectExpiresAt={expiresAt}
        abandonedData={abandonedData}
        viewerTeam="teamA"
        viewerSeat={0}
        onReturnToLobby={vi.fn()}
      />,
    );

    expect(screen.queryByTestId("abandon-result-line")).not.toBeInTheDocument();
  });

  it("omits the result line when the viewer seat is unknown", () => {
    const expiresAt = new Date(Date.now() - 5000).toISOString();
    const abandonedData = {
      abandonedByPlayer: 0,
      teamAFinalScore: 200,
      teamBFinalScore: 300,
      matchDurationSec: 120,
    };

    render(
      <ReconnectOverlay
        disconnectedPlayerName="Frank"
        reconnectExpiresAt={expiresAt}
        abandonedData={abandonedData}
        viewerTeam="teamA"
        onReturnToLobby={vi.fn()}
      />,
    );

    expect(screen.queryByTestId("abandon-result-line")).not.toBeInTheDocument();
  });

  it("auto-redirects to lobby after 3 seconds when abandoned", () => {
    const onReturnToLobby = vi.fn();
    const expiresAt = new Date(Date.now() - 5000).toISOString();
    const abandonedData = {
      abandonedByPlayer: 0,
      teamAFinalScore: 200,
      teamBFinalScore: 300,
      matchDurationSec: 120,
    };

    render(
      <ReconnectOverlay
        disconnectedPlayerName="Frank"
        reconnectExpiresAt={expiresAt}
        abandonedData={abandonedData}
        onReturnToLobby={onReturnToLobby}
      />,
    );

    expect(onReturnToLobby).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(onReturnToLobby).toHaveBeenCalledOnce();
  });

  it("renders one chip per disconnected player when multiple are offline", () => {
    const earliest = new Date(Date.now() + 60_000).toISOString();
    const later = new Date(Date.now() + 120_000).toISOString();
    render(
      <ReconnectOverlay
        disconnectedPlayerName="Alice"
        disconnectedPlayers={[
          { name: "Alice", expiresAt: earliest },
          { name: "Bob", expiresAt: later },
        ]}
        reconnectExpiresAt={earliest}
      />,
    );

    const chipContainer = screen.getByTestId("reconnect-player-name");
    expect(chipContainer).toHaveTextContent("Alice");
    expect(chipContainer).toHaveTextContent("Bob");
  });

  it("falls back to a single chip when disconnectedPlayers is omitted", () => {
    const expiresAt = new Date(Date.now() + 60_000).toISOString();
    render(<ReconnectOverlay disconnectedPlayerName="Solo" reconnectExpiresAt={expiresAt} />);

    const chipContainer = screen.getByTestId("reconnect-player-name");
    expect(chipContainer).toHaveTextContent("Solo");
  });

  it("renders independent per-row countdowns derived from each player's own expiry", () => {
    const earliest = new Date(Date.now() + 30_000).toISOString(); // 30 s
    const later = new Date(Date.now() + 90_000).toISOString(); // 1:30
    render(
      <ReconnectOverlay
        disconnectedPlayerName="Alice"
        disconnectedPlayers={[
          { name: "Alice", expiresAt: earliest },
          { name: "Bob", expiresAt: later },
        ]}
        reconnectExpiresAt={earliest}
      />,
    );

    expect(screen.getByTestId("reconnect-row-countdown-Alice").textContent).toBe("0:30");
    expect(screen.getByTestId("reconnect-row-countdown-Bob").textContent).toBe("1:30");
    // Center ring tracks the earliest (Alice's 0:30), not Bob's later expiry.
    expect(screen.getByTestId("reconnect-countdown").textContent).toBe("0:30");
  });

  it("does not auto-redirect without abandonedData", () => {
    const onReturnToLobby = vi.fn();
    const expiresAt = new Date(Date.now() + 60_000).toISOString();

    render(
      <ReconnectOverlay
        disconnectedPlayerName="Grace"
        reconnectExpiresAt={expiresAt}
        onReturnToLobby={onReturnToLobby}
      />,
    );

    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(onReturnToLobby).not.toHaveBeenCalled();
    expect(screen.getByTestId("reconnect-countdown")).toBeInTheDocument();
  });
});
