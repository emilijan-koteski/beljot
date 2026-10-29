import "@/shared/i18n/i18n";

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";

import { i18n } from "@/shared/i18n/i18n";
import { makeRoom, makeRoomPlayer } from "@/test-utils";

import { matchRuleLines, roomRuleLines } from "./roomRuleLines";
import { RoomRulesContent, RoomRulesPopover } from "./RoomRules";

const t = i18n.t.bind(i18n);

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("matchRuleLines", () => {
  it("lists the four match rules in a fixed order, all standard by default", () => {
    const lines = matchRuleLines(t, makeRoom());
    expect(lines.map((l) => l.key)).toEqual(["direction", "declarations", "fall", "end"]);
    expect(lines.map((l) => l.value)).toEqual([
      "To the right",
      "On",
      "To the winners",
      "Finish the hand",
    ]);
    expect(lines.every((l) => !l.changed)).toBe(true);
  });

  it("marks each rule changed from the standard", () => {
    const lines = matchRuleLines(
      t,
      makeRoom({ clockwise: true, declarationsTransfer: false, stopAtTarget: true }),
    );
    expect(lines.map((l) => l.value)).toEqual(["To the left", "On", "Voided", "Stop at target"]);
    expect(lines.map((l) => l.changed)).toEqual([true, false, true, true]);
  });

  it("leaves declarations on a fall unused when declarations are off", () => {
    const fall = matchRuleLines(
      t,
      makeRoom({ declarationsEnabled: false, declarationsTransfer: false }),
    ).find((l) => l.key === "fall")!;
    expect(fall).toMatchObject({ value: "Not used", unused: true, changed: false });
  });
});

describe("roomRuleLines", () => {
  it("names access and new players, adding bots and Quick Play seating when they apply", () => {
    expect(roomRuleLines(t, makeRoom()).map((l) => l.key)).toEqual(["access", "newPlayers"]);

    const lines = roomRuleLines(
      t,
      makeRoom({
        isPrivate: true,
        allowNewPlayers: false,
        isQuickPlay: true,
        players: [makeRoomPlayer({ seat: 1, isBot: true })],
      }),
    );
    expect(lines.map((l) => [l.key, l.value])).toEqual([
      ["access", "Private · password"],
      ["newPlayers", "Veterans only"],
      ["bots", "1"],
      ["seating", "Quick Play fills it"],
    ]);
  });
});

describe("RoomRulesContent", () => {
  it("explains bots and, with a buy-in, where the stakes go", () => {
    const players = [makeRoomPlayer({ seat: 1, isBot: true })];
    const { unmount } = render(<RoomRulesContent room={makeRoom({ players, coinBuyIn: 500 })} />);
    expect(screen.getByTestId("rules-bots-note")).toHaveTextContent(
      "Bots hold these seats until the host removes them. They pay no buy-in",
    );
    unmount();

    render(<RoomRulesContent room={makeRoom({ players, coinBuyIn: 0 })} live />);
    expect(screen.getByTestId("rules-bots-note")).toHaveTextContent("Bots play these seats.");
    expect(screen.getByTestId("rules-bots-note")).not.toHaveTextContent("buy-in");
  });

  it("says nothing about bots in a room without them", () => {
    render(<RoomRulesContent room={makeRoom()} />);
    expect(screen.queryByTestId("rules-bots-note")).not.toBeInTheDocument();
  });

  it("localizes the rules in mk", async () => {
    await i18n.changeLanguage("mk");
    render(<RoomRulesContent room={makeRoom({ clockwise: true })} />);
    expect(screen.getByTestId("rules-match")).toHaveTextContent("Правила на мечот");
    expect(screen.getByTestId("rule-direction")).toHaveTextContent("Налево");
  });
});

describe("RoomRulesPopover", () => {
  it("names every match rule on its trigger and opens the rules in words", async () => {
    const user = userEvent.setup();
    render(<RoomRulesPopover room={makeRoom({ stopAtTarget: true })} />);

    const trigger = screen.getByTestId("room-rules-trigger");
    expect(trigger).toHaveAccessibleName(/Match end: Stop at target/);

    await user.click(trigger);

    const popup = await screen.findByTestId("room-rules-popup");
    expect(within(popup).getByTestId("rules-match")).toBeInTheDocument();
    expect(within(popup).getByTestId("rules-room")).toBeInTheDocument();
    expect(within(popup).getByTestId("rule-end")).toHaveAttribute("data-changed", "true");
  });
});
