import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, string>) => {
      const translations: Record<string, string> = {
        "match.matchResult.title": "Match Complete",
        "match.matchResult.duration": "Match Duration",
        "match.matchResult.returnToLobby": "Return to Lobby",
        "match.matchResult.returnToRoom": "Return to Room",
        "team.us": "Us",
        "team.them": "Them",
        "match.surrender.unknownProposer": "your opponent",
      };
      if (key === "match.matchResult.winnerUs") return "We Won!";
      if (key === "match.matchResult.winnerThem") return "They Won!";
      if (key === "match.matchResult.surrenderNote" && opts) {
        return `${opts.username} surrendered the match`;
      }
      if (key === "match.settlement.won" && opts) return `You won ${opts.amount} coins`;
      if (key === "match.settlement.lost" && opts) return `You lost ${opts.amount} coins`;
      if (key === "xp.xpGained" && opts) return `You earned ${opts.amount} XP`;
      if (key === "xp.levelUp" && opts) return `Leveled up to ${opts.level}!`;
      return translations[key] ?? key;
    },
  }),
}));

import type { TeamString } from "@/shared/types/matchTypes";
import type { MatchEndPayload } from "@/shared/types/wsEvents";

import { MatchResult } from "./MatchResult";

const matchData: MatchEndPayload = {
  winnerTeam: 0,
  teamAFinalScore: 1020,
  teamBFinalScore: 850,
  matchDurationSec: 725,
};

interface RenderOverrides {
  data?: MatchEndPayload;
  viewerTeam?: TeamString;
  onReturnToLobby?: () => void;
  onReturnToRoom?: () => void;
  surrenderedByUsername?: string;
  coinDelta?: number;
  xpEarned?: number;
  newLevel?: number;
  leveledUp?: boolean;
}

function renderResult(overrides: RenderOverrides = {}) {
  const props = {
    data: overrides.data ?? matchData,
    viewerTeam: overrides.viewerTeam ?? ("teamA" as TeamString),
    onReturnToLobby: overrides.onReturnToLobby ?? vi.fn(),
    onReturnToRoom: overrides.onReturnToRoom ?? vi.fn(),
    surrenderedByUsername: overrides.surrenderedByUsername,
    coinDelta: overrides.coinDelta,
    xpEarned: overrides.xpEarned,
    newLevel: overrides.newLevel,
    leveledUp: overrides.leveledUp,
  };
  return render(<MatchResult {...props} />);
}

describe("MatchResult", () => {
  it("renders winner banner with 'We Won!' when viewer is on the winning team", () => {
    renderResult({ viewerTeam: "teamA" });

    expect(screen.getByTestId("match-result")).toBeInTheDocument();
    expect(screen.getByTestId("match-result-title")).toHaveTextContent("Match Complete");
    const winner = screen.getByTestId("match-result-winner");
    expect(winner).toHaveTextContent("We Won!");
    expect(winner).toHaveAttribute("data-team", "teamA");
  });

  it("renders winner banner with 'They Won!' when viewer is NOT on the winning team", () => {
    renderResult({ viewerTeam: "teamB" });

    const winner = screen.getByTestId("match-result-winner");
    expect(winner).toHaveTextContent("They Won!");
    expect(winner).toHaveAttribute("data-team", "teamA");
  });

  it("renders final scores and column data-team attributes", () => {
    renderResult({ viewerTeam: "teamA" });

    expect(screen.getByTestId("match-result-team-a-score")).toHaveTextContent("1020");
    expect(screen.getByTestId("match-result-team-b-score")).toHaveTextContent("850");
    expect(screen.getByTestId("match-result-team-a-column")).toHaveAttribute("data-team", "teamA");
    expect(screen.getByTestId("match-result-team-b-column")).toHaveAttribute("data-team", "teamB");
  });

  it("renders score column labels viewer-relative — viewer on teamA sees Us / Them", () => {
    renderResult({ viewerTeam: "teamA" });

    expect(screen.getByTestId("match-result-team-a-column")).toHaveTextContent("Us");
    expect(screen.getByTestId("match-result-team-b-column")).toHaveTextContent("Them");
  });

  it("renders score column labels viewer-relative — viewer on teamB sees Them / Us", () => {
    renderResult({ viewerTeam: "teamB" });

    expect(screen.getByTestId("match-result-team-a-column")).toHaveTextContent("Them");
    expect(screen.getByTestId("match-result-team-b-column")).toHaveTextContent("Us");
  });

  it("renders viewer's team column first — viewer on teamA", () => {
    const { container } = renderResult({ viewerTeam: "teamA" });
    const cols = container.querySelectorAll<HTMLElement>('[data-testid$="-column"]');
    expect(cols).toHaveLength(2);
    expect(cols[0]).toHaveAttribute("data-team", "teamA");
    expect(cols[1]).toHaveAttribute("data-team", "teamB");
  });

  it("renders viewer's team column first — viewer on teamB", () => {
    const { container } = renderResult({ viewerTeam: "teamB" });
    const cols = container.querySelectorAll<HTMLElement>('[data-testid$="-column"]');
    expect(cols).toHaveLength(2);
    expect(cols[0]).toHaveAttribute("data-team", "teamB");
    expect(cols[1]).toHaveAttribute("data-team", "teamA");
  });

  it("formats match duration correctly", () => {
    renderResult({ viewerTeam: "teamA" });

    // 725 seconds = 12m 5s
    expect(screen.getByTestId("match-result-duration")).toHaveTextContent("12m 5s");
  });

  it("renders teamB winner correctly", () => {
    renderResult({ data: { ...matchData, winnerTeam: 1 }, viewerTeam: "teamB" });

    const winner = screen.getByTestId("match-result-winner");
    expect(winner).toHaveTextContent("We Won!");
    expect(winner).toHaveAttribute("data-team", "teamB");
  });

  it("renders both the Return-to-room and Return-to-lobby actions", () => {
    renderResult();

    expect(screen.getByTestId("match-result-room-btn")).toHaveTextContent("Return to Room");
    expect(screen.getByTestId("match-result-lobby-btn")).toHaveTextContent("Return to Lobby");
  });

  it("calls onReturnToLobby when the lobby button is clicked", async () => {
    const onReturnToLobby = vi.fn();
    renderResult({ onReturnToLobby });

    await userEvent.click(screen.getByTestId("match-result-lobby-btn"));
    expect(onReturnToLobby).toHaveBeenCalledOnce();
  });

  it("calls onReturnToRoom when the room button is clicked", async () => {
    const onReturnToRoom = vi.fn();
    renderResult({ onReturnToRoom });

    await userEvent.click(screen.getByTestId("match-result-room-btn"));
    expect(onReturnToRoom).toHaveBeenCalledOnce();
  });

  it("does NOT render surrender note for natural match-end", () => {
    renderResult({ viewerTeam: "teamA" });
    expect(screen.queryByTestId("match-result-surrender-note")).toBeNull();
  });

  it("renders surrender note when outcomeReason is 'surrender'", () => {
    renderResult({
      data: { ...matchData, outcomeReason: "surrender", surrenderedBySeat: 1 },
      viewerTeam: "teamA",
      surrenderedByUsername: "alice",
    });
    const note = screen.getByTestId("match-result-surrender-note");
    expect(note).toBeInTheDocument();
    expect(note).toHaveTextContent("alice surrendered the match");
  });

  it("falls back to unknownProposer when surrender username is missing", () => {
    renderResult({
      data: { ...matchData, outcomeReason: "surrender", surrenderedBySeat: 1 },
      viewerTeam: "teamA",
    });
    const note = screen.getByTestId("match-result-surrender-note");
    expect(note).toHaveTextContent(/your opponent/);
  });

  // Story 9.2 — coin outcome moved from a transient toast into this dialog.
  it("renders the won amount with a positive coin delta", () => {
    renderResult({ coinDelta: 500 });
    const coins = screen.getByTestId("match-result-coins");
    expect(coins).toHaveTextContent("You won 500 coins");
    expect(coins).toHaveAttribute("data-coin-delta", "500");
  });

  it("renders the lost amount (as a positive number) with a negative coin delta", () => {
    renderResult({ coinDelta: -500 });
    const coins = screen.getByTestId("match-result-coins");
    expect(coins).toHaveTextContent("You lost 500 coins");
    expect(coins).toHaveAttribute("data-coin-delta", "-500");
  });

  it("renders no coin line on a zero delta (lone winner who only recovers their stake)", () => {
    renderResult({ coinDelta: 0 });
    expect(screen.queryByTestId("match-result-coins")).toBeNull();
  });

  it("renders no coin line for a free match (coinDelta undefined)", () => {
    renderResult({});
    expect(screen.queryByTestId("match-result-coins")).toBeNull();
  });

  // --- XP flourish (Story 9.5) ---

  it("renders the XP earned line when xpEarned > 0 and no level-up", () => {
    renderResult({ xpEarned: 101, newLevel: 1, leveledUp: false });
    const xp = screen.getByTestId("match-result-xp");
    expect(xp).toHaveTextContent("You earned 101 XP");
    expect(xp).toHaveAttribute("data-xp-earned", "101");
    expect(xp).toHaveAttribute("data-leveled-up", "false");
  });

  it("shows the level-up message when the match pushed the viewer to a new level", () => {
    renderResult({ xpEarned: 120, newLevel: 4, leveledUp: true });
    const xp = screen.getByTestId("match-result-xp");
    expect(xp).toHaveTextContent("Leveled up to 4!");
    expect(xp).toHaveAttribute("data-leveled-up", "true");
  });

  it("renders no XP line when xpEarned is 0 (abandoning team forfeit)", () => {
    renderResult({ xpEarned: 0 });
    expect(screen.queryByTestId("match-result-xp")).toBeNull();
  });

  it("renders no XP line for a match still awaiting the xp_awarded event", () => {
    renderResult({});
    expect(screen.queryByTestId("match-result-xp")).toBeNull();
  });
});
