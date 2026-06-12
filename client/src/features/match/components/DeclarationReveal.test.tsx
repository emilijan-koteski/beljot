import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// AC10 / D120: mock react-i18next so visible-text assertions can verify the
// viewer-relative "Us declared" / "Them declared" copy. Mirrors the pattern
// used by MatchResult.test.tsx.
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, string | number>) => {
      const translations: Record<string, string> = {
        "team.us": "Us",
        "team.them": "Them",
        "match.declaration.resolved": "Declarations",
        "match.declaration.addedToHand": "Added to this hand's score",
        "match.declaration.total": "Total",
      };
      if (key === "match.declaration.teamDeclared" && opts) return `${opts.team} declared`;
      if (key === "match.declaration.headlineUs") return "We won the declarations";
      if (key === "match.declaration.headlineThem") return "They won the declarations";
      if (key === "match.declaration.tiebreaker" && opts)
        return `${opts.player}'s ${opts.label} — highest meld at the table`;
      if (key === "match.declaration.byPlayer" && opts) return `by ${opts.name}`;
      if (key === "match.declaration.tierce") return "Tierce";
      if (key === "match.declaration.quarte") return "Quarte";
      if (key === "match.declaration.quint") return "Quint";
      if (key === "match.declaration.carre") return "Carré";
      return translations[key] ?? key;
    },
  }),
}));

import type { PlayerState } from "@/shared/types/matchTypes";
import type { DeclarationsResolvedPayload } from "@/shared/types/wsEvents";

import { DeclarationReveal } from "./DeclarationReveal";

function testPlayer(seat: number, username: string): PlayerState {
  return {
    seat,
    hand: [],
    userId: seat + 1,
    username,
    team: seat % 2 === 0 ? "teamA" : "teamB",
    declarations: [],
    connected: true,
    isBot: false,
  };
}

const mockPlayers: PlayerState[] = [
  testPlayer(0, "alice"),
  testPlayer(1, "bob"),
  testPlayer(2, "carol"),
  testPlayer(3, "dave"),
];

beforeEach(() => {
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

const mockPayload: DeclarationsResolvedPayload = {
  winnerTeam: 1,
  declarations: [{ playerSeat: 1, type: "sequence", value: 50, cards: ["JD", "QD", "KD", "AD"] }],
};

describe("DeclarationReveal", () => {
  it("renders panel with winner team data-team and cards", () => {
    render(
      <DeclarationReveal
        payload={mockPayload}
        players={mockPlayers}
        viewerTeam="teamB"
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("declaration-reveal")).toBeInTheDocument();
    const label = screen.getByTestId("declaration-reveal-team");
    expect(label).toHaveAttribute("data-team", "teamB");
    expect(screen.getByTestId("playing-card-JD")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-QD")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-KD")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-AD")).toBeInTheDocument();
  });

  it("renders 'We won the declarations' when viewer's partner declares", () => {
    // viewer is teamB (seat 1 or 3); declarer is seat 1 (teamB) → both partners see Us
    render(
      <DeclarationReveal
        payload={mockPayload}
        players={mockPlayers}
        viewerTeam="teamB"
        onComplete={vi.fn()}
      />,
    );
    const label = screen.getByTestId("declaration-reveal-team");
    expect(label).toHaveAttribute("data-team", "teamB");
    expect(label).toHaveTextContent("We won the declarations");
  });

  it("renders 'They won the declarations' when viewer is teamA and opponents declare", () => {
    // viewer is teamA; declarer is on teamB → they see Them
    render(
      <DeclarationReveal
        payload={mockPayload}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={vi.fn()}
      />,
    );
    const label = screen.getByTestId("declaration-reveal-team");
    // The winner team is still teamB — data-team reflects the winner team
    // (used for styling), not the viewer-relative label.
    expect(label).toHaveAttribute("data-team", "teamB");
    expect(label).toHaveTextContent("They won the declarations");
  });

  it("centers the panel regardless of winning declarer's seat", () => {
    const eastWinner: DeclarationsResolvedPayload = {
      winnerTeam: 1,
      declarations: [
        { playerSeat: 1, type: "sequence", value: 50, cards: ["JD", "QD", "KD", "AD"] },
      ],
    };
    const northWinner: DeclarationsResolvedPayload = {
      winnerTeam: 0,
      declarations: [
        { playerSeat: 2, type: "sequence", value: 50, cards: ["JD", "QD", "KD", "AD"] },
      ],
    };
    const expectedCenterClasses = ["top-1/2", "left-1/2", "-translate-x-1/2", "-translate-y-1/2"];

    const { rerender } = render(
      <DeclarationReveal
        payload={eastWinner}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={vi.fn()}
      />,
    );
    let panel = screen.getByTestId("declaration-reveal");
    for (const cls of expectedCenterClasses) {
      expect(panel.className).toContain(cls);
    }

    rerender(
      <DeclarationReveal
        payload={northWinner}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={vi.fn()}
      />,
    );
    panel = screen.getByTestId("declaration-reveal");
    for (const cls of expectedCenterClasses) {
      expect(panel.className).toContain(cls);
    }
  });

  it("renders +value per meld and a +total in the brass strip (sum of meld values)", () => {
    const payload: DeclarationsResolvedPayload = {
      winnerTeam: 0,
      declarations: [
        { playerSeat: 0, type: "sequence", value: 50, cards: ["JS", "QS", "KS", "AS"] },
        { playerSeat: 2, type: "sequence", value: 20, cards: ["8H", "9H", "TH"] },
      ],
    };
    render(
      <DeclarationReveal
        payload={payload}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={vi.fn()}
      />,
    );
    const meldValues = screen.getAllByTestId("declaration-reveal-meld-value");
    expect(meldValues.map((n) => n.textContent)).toEqual(["+50", "+20"]);
    expect(screen.getByTestId("declaration-reveal-total-value")).toHaveTextContent("+70");
  });

  it("does not render when winnerTeam is null", () => {
    const payload: DeclarationsResolvedPayload = {
      winnerTeam: null,
      declarations: [],
    };
    render(
      <DeclarationReveal
        payload={payload}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("declaration-reveal")).not.toBeInTheDocument();
  });

  it("stacks multiple declarations from the winning team", () => {
    const payload: DeclarationsResolvedPayload = {
      winnerTeam: 0,
      declarations: [
        { playerSeat: 0, type: "sequence", value: 50, cards: ["JS", "QS", "KS", "AS"] },
        { playerSeat: 0, type: "four_of_a_kind", value: 200, cards: ["JC", "JH", "JD", "JS"] },
      ],
    };
    render(
      <DeclarationReveal
        payload={payload}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("playing-card-QS")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-JC")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-JH")).toBeInTheDocument();
    // JS appears in both declarations — multiple matches expected
    expect(screen.getAllByTestId("playing-card-JS").length).toBeGreaterThanOrEqual(1);
  });

  it("shows the declarer's username for each declaration row", () => {
    const payload: DeclarationsResolvedPayload = {
      winnerTeam: 0,
      declarations: [
        { playerSeat: 0, type: "sequence", value: 50, cards: ["JS", "QS", "KS", "AS"] },
        { playerSeat: 2, type: "four_of_a_kind", value: 200, cards: ["JC", "JH", "JD", "JS"] },
      ],
    };
    render(
      <DeclarationReveal
        payload={payload}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={vi.fn()}
      />,
    );
    const declarers = screen.getAllByTestId("declaration-reveal-declarer");
    expect(declarers).toHaveLength(2);
    expect(declarers[0]).toHaveAttribute("data-seat", "0");
    expect(declarers[0]).toHaveTextContent("by alice");
    expect(declarers[1]).toHaveAttribute("data-seat", "2");
    expect(declarers[1]).toHaveTextContent("by carol");
  });

  it("renders the tiebreaker line only when there is more than one meld", () => {
    const single: DeclarationsResolvedPayload = {
      winnerTeam: 1,
      declarations: [
        { playerSeat: 1, type: "sequence", value: 50, cards: ["JD", "QD", "KD", "AD"] },
      ],
    };
    const { rerender } = render(
      <DeclarationReveal
        payload={single}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("declaration-reveal-tiebreaker")).not.toBeInTheDocument();

    const multi: DeclarationsResolvedPayload = {
      winnerTeam: 1,
      declarations: [
        { playerSeat: 1, type: "sequence", value: 20, cards: ["8H", "9H", "TH"] },
        { playerSeat: 3, type: "four_of_a_kind", value: 200, cards: ["JC", "JH", "JD", "JS"] },
      ],
    };
    rerender(
      <DeclarationReveal
        payload={multi}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={vi.fn()}
      />,
    );
    // Highest meld is dave's four_of_a_kind (200) — that's what tipped the team.
    expect(screen.getByTestId("declaration-reveal-tiebreaker")).toHaveTextContent(
      "dave's Carré — highest meld at the table",
    );
  });

  it("renders the total label in the brass strip", () => {
    render(
      <DeclarationReveal
        payload={mockPayload}
        players={mockPlayers}
        viewerTeam="teamB"
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("declaration-reveal-total")).toHaveTextContent("Total");
    expect(screen.getByTestId("declaration-reveal-total")).toHaveTextContent(
      "Added to this hand's score",
    );
  });

  it("falls back to seat marker when player is unknown", () => {
    const payload: DeclarationsResolvedPayload = {
      winnerTeam: 1,
      declarations: [{ playerSeat: 3, type: "sequence", value: 50, cards: ["JD", "QD", "KD"] }],
    };
    render(
      <DeclarationReveal payload={payload} players={[]} viewerTeam="teamA" onComplete={vi.fn()} />,
    );
    expect(screen.getByTestId("declaration-reveal-declarer")).toHaveTextContent("by #3");
  });

  it("auto-dismisses after 8 seconds", () => {
    vi.useFakeTimers();
    const onComplete = vi.fn();
    render(
      <DeclarationReveal
        payload={mockPayload}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={onComplete}
      />,
    );
    vi.advanceTimersByTime(7900);
    expect(onComplete).not.toHaveBeenCalled();
    vi.advanceTimersByTime(200);
    expect(onComplete).toHaveBeenCalledOnce();
    vi.useRealTimers();
  });

  it("auto-dismisses faster with prefers-reduced-motion", () => {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: true,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    });
    vi.useFakeTimers();
    const onComplete = vi.fn();
    render(
      <DeclarationReveal
        payload={mockPayload}
        players={mockPlayers}
        viewerTeam="teamA"
        onComplete={onComplete}
      />,
    );
    vi.advanceTimersByTime(1600);
    expect(onComplete).toHaveBeenCalledOnce();
    vi.useRealTimers();
  });
});
