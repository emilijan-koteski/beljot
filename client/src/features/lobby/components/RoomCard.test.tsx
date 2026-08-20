import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { RoomCard } from "@/features/lobby/components/RoomCard";
import { i18n } from "@/shared/i18n/i18n";
import type { Room } from "@/shared/types/apiTypes";

const baseRoom: Room = {
  id: 1,
  name: "Table One",
  code: "ABC123",
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
  isPrivate: false,
  // Ungated: the default every room carries unless its owner opts in (Story 9.8).
  minHonor: 0,
  allowNewPlayers: true,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  players: [
    {
      id: 1,
      roomId: 1,
      userId: 1,
      username: "host",
      seat: 0,
      team: "teamA",
      isBot: false,
      createdAt: "",
    },
  ],
};

describe("RoomCard", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("renders the localized 501 match-mode label in mk locale", async () => {
    // mk distinguishes the i18n label ("501 поен") from the unlocalized
    // "501 pts" fallback, which is identical to the en label.
    await i18n.changeLanguage("mk");

    render(<RoomCard room={{ ...baseRoom, matchMode: "501" }} onJoin={() => {}} />);

    expect(screen.getByText(/501 поен/)).toBeInTheDocument();
  });

  it("renders the localized bot name for bot seats in mk locale", async () => {
    // mk renders "Бот 2" — all-Cyrillic, seat-derived — never a blank chip
    // from the bot's empty wire username.
    await i18n.changeLanguage("mk");

    const players = [
      ...(baseRoom.players ?? []),
      {
        id: 0,
        roomId: 1,
        userId: 0,
        username: "",
        seat: 1,
        team: "teamB",
        isBot: true,
        createdAt: "",
      },
    ];
    render(<RoomCard room={{ ...baseRoom, players }} onJoin={() => {}} />);

    expect(screen.getByTestId("room-1-seat-1")).toHaveTextContent("Бот 2");
    // The chip disc shows the bot glyph, not a name initial.
    expect(screen.getByTestId("seat-chip-bot-icon")).toBeInTheDocument();
  });

  it("labels a quick-play room with the Quick Play badge and a 'Join queue' action", () => {
    render(<RoomCard room={{ ...baseRoom, isQuickPlay: true }} onJoin={() => {}} />);

    expect(screen.getByTestId("quick-play-badge")).toBeInTheDocument();
    expect(screen.getByTestId("room-card-join")).toHaveTextContent("Join queue");
  });

  it("renders a custom room with the plain Join action and no badge", () => {
    render(<RoomCard room={baseRoom} onJoin={() => {}} />);

    expect(screen.queryByTestId("quick-play-badge")).not.toBeInTheDocument();
    const join = screen.getByTestId("room-card-join");
    expect(join).toHaveTextContent("Join");
    expect(join).not.toHaveTextContent("Join queue");
  });

  it("renders a lock indicator for a private room", () => {
    render(<RoomCard room={{ ...baseRoom, isPrivate: true }} onJoin={() => {}} />);
    expect(screen.getByTestId("room-card-lock")).toBeInTheDocument();
  });

  it("does not render the lock indicator for a public room", () => {
    render(<RoomCard room={baseRoom} onJoin={() => {}} />);
    expect(screen.queryByTestId("room-card-lock")).toBeNull();
  });

  // Honor gate (Story 9.8 AC5). Both chips are conditional, so an ungated card
  // is visually unchanged from before the story.
  it("renders neither honor chip for an ungated room", () => {
    render(<RoomCard room={baseRoom} onJoin={() => {}} />);

    expect(screen.queryByTestId("room-card-min-honor")).toBeNull();
    expect(screen.queryByTestId("room-card-veterans-only")).toBeNull();
  });

  it("renders the honor chip with the room's threshold when minHonor is set", () => {
    render(<RoomCard room={{ ...baseRoom, minHonor: 85 }} onJoin={() => {}} />);

    const chip = screen.getByTestId("room-card-min-honor");
    // Assert the computed number through data-*, so the test never depends on
    // i18n wording.
    expect(chip).toHaveAttribute("data-min-honor", "85");
    expect(chip).toHaveAccessibleName(/85/);
    // The two gates are independent: a threshold alone is not veterans-only.
    expect(screen.queryByTestId("room-card-veterans-only")).toBeNull();
  });

  it("renders the veterans-only indicator independently of the threshold", () => {
    // minHonor 0 with allowNewPlayers false — "anyone experienced, any score".
    // The epic's scoping would have made this combination render nothing.
    render(<RoomCard room={{ ...baseRoom, allowNewPlayers: false }} onJoin={() => {}} />);

    expect(screen.getByTestId("room-card-veterans-only")).toBeInTheDocument();
    expect(screen.queryByTestId("room-card-min-honor")).toBeNull();
  });

  it("renders both honor chips when both gates are set", () => {
    render(
      <RoomCard room={{ ...baseRoom, minHonor: 90, allowNewPlayers: false }} onJoin={() => {}} />,
    );

    expect(screen.getByTestId("room-card-min-honor")).toHaveAttribute("data-min-honor", "90");
    expect(screen.getByTestId("room-card-veterans-only")).toBeInTheDocument();
  });

  it("still renders the join action for a gated room", () => {
    // AC5: gated rooms stay listed and labelled, never filtered out — the player
    // sees the requirement and decides (9.6's "listed but locked" precedent).
    render(<RoomCard room={{ ...baseRoom, minHonor: 95 }} onJoin={() => {}} />);

    expect(screen.getByTestId("room-card-join")).toBeInTheDocument();
  });

  // Story 12.8: before lobby.card.variantCroatia existed, variantLabel fell
  // through to title-casing the raw server string, so every locale rendered the
  // English-looking "Croatia". mk is the discriminator that catches it — a
  // Latin-script label in a Cyrillic bundle is unmistakable.
  it.each([
    ["mk", "Хрватска"],
    ["hr", "Hrvatska"],
    ["sr", "Hrvatska"],
    ["en", "Croatian"],
  ] as const)("renders the localized Croatian variant label in %s locale", async (lang, label) => {
    await i18n.changeLanguage(lang);

    render(<RoomCard room={{ ...baseRoom, variant: "croatia" }} onJoin={() => {}} />);

    expect(screen.getByText(new RegExp(label))).toBeInTheDocument();
    expect(screen.queryByText(/Croatia\b/)).not.toBeInTheDocument();
  });

  it("keeps both honor chips all-Cyrillic in the mk locale", async () => {
    await i18n.changeLanguage("mk");

    render(
      <RoomCard room={{ ...baseRoom, minHonor: 85, allowNewPlayers: false }} onJoin={() => {}} />,
    );

    // The threshold chip is the number plus a glyph, so only the veterans-only
    // label carries translatable words.
    expect(screen.getByTestId("room-card-min-honor")).toHaveTextContent("85");
    expect(screen.getByTestId("room-card-veterans-only").textContent ?? "").toMatch(/^[Ѐ-ӿ\s]+$/);
  });
});
