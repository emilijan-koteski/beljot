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
  ownerAvatarUrl: null,
  variant: "bitola",
  matchMode: "1001",
  timerStyle: "relaxed",
  timerDurationSeconds: null,
  status: "waiting",
  playerCount: 1,
  isQuickPlay: false,
  coinBuyIn: 0,
  isPrivate: false,
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
      avatarUrl: null,
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
    await i18n.changeLanguage("mk");

    render(<RoomCard room={{ ...baseRoom, matchMode: "501" }} onJoin={() => {}} />);

    expect(screen.getByText(/501 поен/)).toBeInTheDocument();
  });

  it("renders the localized bot name for bot seats in mk locale", async () => {
    await i18n.changeLanguage("mk");

    const players = [
      ...(baseRoom.players ?? []),
      {
        id: 0,
        roomId: 1,
        userId: 0,
        username: "",
        avatarUrl: null,
        seat: 1,
        team: "teamB",
        isBot: true,
        createdAt: "",
      },
    ];
    render(<RoomCard room={{ ...baseRoom, players }} onJoin={() => {}} />);

    expect(screen.getByTestId("room-1-seat-1")).toHaveTextContent("Бот 2");
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

  it("renders neither honor chip for an ungated room", () => {
    render(<RoomCard room={baseRoom} onJoin={() => {}} />);

    expect(screen.queryByTestId("room-card-min-honor")).toBeNull();
    expect(screen.queryByTestId("room-card-veterans-only")).toBeNull();
  });

  it("renders the honor chip with the room's threshold when minHonor is set", () => {
    render(<RoomCard room={{ ...baseRoom, minHonor: 85 }} onJoin={() => {}} />);

    const chip = screen.getByTestId("room-card-min-honor");
    expect(chip).toHaveAttribute("data-min-honor", "85");
    expect(chip).toHaveAccessibleName(/85/);
    expect(screen.queryByTestId("room-card-veterans-only")).toBeNull();
  });

  it("renders the veterans-only indicator independently of the threshold", () => {
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
    render(<RoomCard room={{ ...baseRoom, minHonor: 95 }} onJoin={() => {}} />);

    expect(screen.getByTestId("room-card-join")).toBeInTheDocument();
  });

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

    expect(screen.getByTestId("room-card-min-honor")).toHaveTextContent("85");
    expect(screen.getByTestId("room-card-veterans-only").textContent ?? "").toMatch(/^[Ѐ-ӿ\s]+$/);
  });

  it("leaves an ordinary room unchipped", () => {
    render(<RoomCard room={{ ...baseRoom, declarationsEnabled: true }} onJoin={() => {}} />);

    expect(screen.queryByTestId("room-card-no-declarations")).not.toBeInTheDocument();
  });

  it("chips a room that plays without declarations", () => {
    render(<RoomCard room={{ ...baseRoom, declarationsEnabled: false }} onJoin={() => {}} />);

    const chip = screen.getByTestId("room-card-no-declarations");
    expect(chip).toBeInTheDocument();
    expect(chip).toHaveAccessibleName(/without declarations and without Belote/i);
  });

  it("treats an absent declarationsEnabled as ON, never as off", () => {
    const withoutTheField: Room = { ...baseRoom, declarationsEnabled: true };
    delete withoutTheField.declarationsEnabled;

    render(<RoomCard room={withoutTheField} onJoin={() => {}} />);

    expect(screen.queryByTestId("room-card-no-declarations")).not.toBeInTheDocument();
  });

  it("localizes the no-declarations chip in every locale", async () => {
    for (const locale of ["en", "mk", "hr", "sr"] as const) {
      await i18n.changeLanguage(locale);

      const { unmount } = render(
        <RoomCard room={{ ...baseRoom, declarationsEnabled: false }} onJoin={() => {}} />,
      );

      const text = (screen.getByTestId("room-card-no-declarations").textContent ?? "").trim();
      expect(text, `${locale} chip text`).not.toBe("");
      expect(text, `${locale} must not render the raw key`).not.toContain("lobby.card");

      if (locale === "mk") {
        expect(text, "mk is all-Cyrillic").toMatch(/^[Ѐ-ӿ\s]+$/);
      } else {
        expect(text, `${locale} is Latin`).toMatch(/^[A-Za-zČčĆćĐđŠšŽž\s]+$/);
      }

      unmount();
    }
  });

  it("leaves a finish-the-hand room unchipped", () => {
    render(<RoomCard room={{ ...baseRoom, stopAtTarget: false }} onJoin={() => {}} />);

    expect(screen.queryByTestId("room-card-stop-at-target")).not.toBeInTheDocument();
  });

  it("chips a room that stops at the target", () => {
    render(<RoomCard room={{ ...baseRoom, stopAtTarget: true }} onJoin={() => {}} />);

    const chip = screen.getByTestId("room-card-stop-at-target");
    expect(chip).toBeInTheDocument();
    expect(chip).toHaveAccessibleName(/without finishing the hand/i);
  });

  it("treats an absent stopAtTarget as OFF, never as on", () => {
    const withoutTheField: Room = { ...baseRoom, stopAtTarget: false };
    delete withoutTheField.stopAtTarget;

    render(<RoomCard room={withoutTheField} onJoin={() => {}} />);

    expect(screen.queryByTestId("room-card-stop-at-target")).not.toBeInTheDocument();
  });

  it("localizes the stop-at-target chip in every locale", async () => {
    for (const locale of ["en", "mk", "hr", "sr"] as const) {
      await i18n.changeLanguage(locale);

      const { unmount } = render(
        <RoomCard room={{ ...baseRoom, stopAtTarget: true }} onJoin={() => {}} />,
      );

      const text = (screen.getByTestId("room-card-stop-at-target").textContent ?? "").trim();
      expect(text, `${locale} chip text`).not.toBe("");
      expect(text, `${locale} must not render the raw key`).not.toContain("lobby.card");

      if (locale === "mk") {
        expect(text, "mk is all-Cyrillic").toMatch(/^[Ѐ-ӿ\s]+$/);
      } else {
        expect(text, `${locale} is Latin`).toMatch(/^[A-Za-zČčĆćĐđŠšŽž\s]+$/);
      }

      unmount();
    }
  });

  it("exposes each rule chip's explanation through a named role", () => {
    render(
      <RoomCard
        room={{ ...baseRoom, declarationsEnabled: false, stopAtTarget: true }}
        onJoin={() => {}}
      />,
    );

    for (const testId of ["room-card-no-declarations", "room-card-stop-at-target"]) {
      const chip = screen.getByTestId(testId);
      expect(chip).toHaveAttribute("role", "img");
      expect(chip.getAttribute("aria-label")?.length ?? 0).toBeGreaterThan(
        chip.textContent?.length ?? 0,
      );
    }
  });
});

describe("RoomCard avatars", () => {
  const url = "https://assets.test/avatars/h/128.webp";

  it("shows the host's picture in the footer disc and the seat chip", () => {
    render(
      <RoomCard
        room={{
          ...baseRoom,
          ownerAvatarUrl: url,
          players: baseRoom.players!.map((p) => ({ ...p, avatarUrl: url })),
        }}
        onJoin={() => {}}
      />,
    );
    expect(screen.getByTestId("room-card-owner-avatar").querySelector("img")).toHaveAttribute(
      "src",
      url,
    );
    expect(screen.getByTestId("room-1-seat-0").querySelector("img")).toHaveAttribute("src", url);
  });

  it("keeps the initials when nobody has a picture, or the payload predates avatars", () => {
    const legacy: Partial<Room> = { ...baseRoom };
    delete legacy.ownerAvatarUrl;
    render(<RoomCard room={legacy as Room} onJoin={() => {}} />);
    const owner = screen.getByTestId("room-card-owner-avatar");
    expect(owner.querySelector("img")).toBeNull();
    expect(owner).toHaveTextContent("H");
    expect(screen.getByTestId("room-1-seat-0").querySelector("img")).toBeNull();
  });
});
