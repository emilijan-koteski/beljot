import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { PlayerState } from "@/shared/types/matchTypes";

import { PlayerSeat } from "./PlayerSeat";

function makePlayer(overrides: Partial<PlayerState> = {}): PlayerState {
  return {
    hand: [],
    seat: 0,
    userId: 10,
    username: "Alice",
    team: "teamA",
    declarations: [],
    connected: true,
    isBot: false,
    level: 1,
    ...overrides,
  };
}

describe("PlayerSeat", () => {
  it("renders the localized seat-derived bot name for bot seats", () => {
    const bot = makePlayer({ seat: 2, userId: 0, username: "", isBot: true });
    render(<PlayerSeat player={bot} isSelf={false} isActive={false} seatTeam="gold" />);
    expect(screen.getByText("Bot 3")).toBeInTheDocument();
    // The avatar disc shows the bot glyph, not a name initial.
    expect(screen.getByTestId("avatar-icon")).toBeInTheDocument();
    expect(screen.getByTestId("player-seat-avatar")).not.toHaveTextContent("B");
  });

  it("renders empty seat with 'Waiting…' text when player is null", () => {
    render(<PlayerSeat player={null} isSelf={false} isActive={false} seatTeam="gold" />);

    expect(screen.getByText("Waiting…")).toBeInTheDocument();
    expect(screen.getByTestId("player-seat-empty")).toHaveAttribute("data-seat-team", "gold");
  });

  it("renders occupied seat with username and gold seat-team", () => {
    render(
      <PlayerSeat
        player={makePlayer({ username: "Bob" })}
        isSelf={false}
        isActive={false}
        seatTeam="gold"
      />,
    );

    expect(screen.getByText("Bob")).toBeInTheDocument();
    const seat = screen.getByTestId("player-seat-0");
    expect(seat).toHaveAttribute("data-seat-team", "gold");
    expect(seat).toHaveAttribute("data-active", "false");
  });

  it("renders the player's level in the name pill (not the team label)", () => {
    render(
      <PlayerSeat
        player={makePlayer({ level: 7 })}
        isSelf={false}
        isActive={false}
        seatTeam="gold"
      />,
    );

    expect(screen.getByTestId("player-seat-level")).toHaveTextContent("Lvl 7");
    // The visible pill no longer shows the team label "Us".
    expect(screen.queryByTestId("player-seat-name-pill")).not.toHaveTextContent("Us");
    // Team is still announced to screen readers via the seat aria-label.
    expect(screen.getByTestId("player-seat-0")).toHaveAttribute("aria-label", "Alice, Us, waiting");
  });

  it("renders silver seat-team for opponents", () => {
    render(
      <PlayerSeat
        player={makePlayer({ team: "teamB", seat: 1 })}
        isSelf={false}
        isActive={false}
        seatTeam="silver"
      />,
    );

    expect(screen.getByTestId("player-seat-1")).toHaveAttribute("data-seat-team", "silver");
  });

  it("marks seat as active and lights the lime channel on the name pill", () => {
    render(<PlayerSeat player={makePlayer()} isSelf={false} isActive={true} seatTeam="gold" />);

    const seat = screen.getByTestId("player-seat-0");
    expect(seat).toHaveAttribute("data-active", "true");
    const namePill = screen.getByTestId("player-seat-name-pill");
    expect(namePill.getAttribute("style")).toContain("--turn-lime");
  });

  it("renders self seat with 'You' label and scaled-up transform", () => {
    render(<PlayerSeat player={makePlayer()} isSelf={true} isActive={false} seatTeam="gold" />);

    expect(screen.getByText("You")).toBeInTheDocument();
    const seat = screen.getByTestId("player-seat-0");
    expect(seat).toHaveAttribute("data-self", "true");
    expect(seat.getAttribute("style")).toContain("scale(1.08)");
  });

  it("uses viewer-relative aria-label (Us/Them, not teamA/teamB)", () => {
    render(
      <PlayerSeat
        player={makePlayer({ username: "Alice" })}
        isSelf={false}
        isActive={true}
        seatTeam="gold"
      />,
    );

    expect(screen.getByTestId("player-seat-0")).toHaveAttribute("aria-label", "Alice, Us, active");
  });

  it("uses 'Them' aria-label for the silver team", () => {
    render(
      <PlayerSeat
        player={makePlayer({ username: "Bob", team: "teamB", seat: 3 })}
        isSelf={false}
        isActive={false}
        seatTeam="silver"
      />,
    );

    expect(screen.getByTestId("player-seat-3")).toHaveAttribute("aria-label", "Bob, Them, waiting");
  });

  it("shows a card-back stack for opponents when cardCount > 0", () => {
    render(
      <PlayerSeat
        player={makePlayer()}
        isSelf={false}
        isActive={false}
        seatTeam="gold"
        cardCount={8}
      />,
    );

    expect(screen.getByTestId("player-seat-card-stack")).toBeInTheDocument();
    expect(screen.getByText(/×8/)).toBeInTheDocument();
  });

  it("does not render the card-back stack for the self seat", () => {
    render(
      <PlayerSeat
        player={makePlayer()}
        isSelf={true}
        isActive={false}
        seatTeam="gold"
        cardCount={8}
      />,
    );

    expect(screen.queryByTestId("player-seat-card-stack")).not.toBeInTheDocument();
  });

  it("renders the avatar initial from the player's username", () => {
    render(
      <PlayerSeat
        player={makePlayer({ username: "Charlie" })}
        isSelf={false}
        isActive={false}
        seatTeam="gold"
      />,
    );

    expect(screen.getByTestId("player-seat-avatar")).toHaveTextContent("C");
  });

  it("renders the dealer chip when isDealer is true", () => {
    render(
      <PlayerSeat player={makePlayer()} isSelf={false} isActive={false} seatTeam="gold" isDealer />,
    );

    expect(screen.getByTestId("player-seat-dealer-chip")).toBeInTheDocument();
  });

  it("renders the trump-caller chip when trumpCallerSuit is set", () => {
    render(
      <PlayerSeat
        player={makePlayer()}
        isSelf={false}
        isActive={false}
        seatTeam="silver"
        trumpCallerSuit="H"
      />,
    );

    expect(screen.getByTestId("player-seat-caller-chip")).toHaveTextContent("♥");
  });

  it("renders both chips stacked when the seat is dealer AND trump caller", () => {
    render(
      <PlayerSeat
        player={makePlayer()}
        isSelf={false}
        isActive={false}
        seatTeam="gold"
        isDealer
        trumpCallerSuit="S"
      />,
    );

    expect(screen.getByTestId("player-seat-dealer-chip")).toBeInTheDocument();
    expect(screen.getByTestId("player-seat-caller-chip")).toBeInTheDocument();
  });

  it("dims and grayscales a disconnected player", () => {
    render(
      <PlayerSeat
        player={makePlayer({ connected: false })}
        isSelf={false}
        isActive={false}
        seatTeam="gold"
      />,
    );

    const seat = screen.getByTestId("player-seat-0");
    expect(seat.getAttribute("style")).toContain("grayscale");
    expect(seat.getAttribute("style")).toContain("opacity: 0.5");
    expect(seat).toHaveAttribute("aria-label", "Alice, Us, disconnected");
  });
});
