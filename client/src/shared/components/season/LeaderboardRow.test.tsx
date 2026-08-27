import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { LeaderboardRow } from "@/shared/components/season/LeaderboardRow";
import { i18n } from "@/shared/i18n/i18n";

const base = {
  position: 4,
  userId: 42,
  username: "kiro",
  sp: 4000,
  tier: "gold",
  gamesPlayed: 31,
};

function renderRow(props: Partial<typeof base> & { isSelf?: boolean } = {}) {
  return render(
    <ul>
      <LeaderboardRow {...base} {...props} />
    </ul>,
  );
}

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("LeaderboardRow", () => {
  it("renders position, username, SP and games played", () => {
    renderRow();
    const row = screen.getByTestId("leaderboard-row");
    expect(row.querySelector('[data-testid="leaderboard-position"]')).toHaveTextContent("4");
    expect(row.querySelector('[data-testid="leaderboard-username"]')).toHaveTextContent("kiro");
    expect(row.querySelector('[data-testid="leaderboard-sp"]')).toHaveTextContent(
      (4000).toLocaleString(),
    );
    expect(row.querySelector('[data-testid="leaderboard-games"]')).toHaveTextContent("31");
  });

  it("carries the row identity attributes the surfaces select on", () => {
    renderRow();
    expect(screen.getByTestId("leaderboard-row")).toHaveAttribute("data-user-id", "42");
    expect(screen.getByTestId("leaderboard-row")).not.toHaveAttribute("data-self");
  });

  it("marks the viewer's own row with data-self and a you pill", () => {
    renderRow({ isSelf: true });
    expect(screen.getByTestId("leaderboard-row")).toHaveAttribute("data-self", "true");
    expect(screen.getByTestId("leaderboard-you")).toBeInTheDocument();
  });

  it("omits the games cell when no count is supplied (the lobby widget)", () => {
    render(
      <ul>
        <LeaderboardRow {...base} gamesPlayed={undefined} />
      </ul>,
    );
    expect(screen.queryByTestId("leaderboard-games")).not.toBeInTheDocument();
  });

  // Go zero values are real values: 0 games played must render as "0", not vanish.
  it("renders a zero games count rather than dropping the cell", () => {
    renderRow({ gamesPlayed: 0 });
    expect(screen.getByTestId("leaderboard-games")).toHaveTextContent("0");
  });

  // P15: the games count is sanitized by a NEUTRAL clamp, not by the SP ladder's
  // seasonSpOrZero. Both are generic today; only one stays generic by contract.
  it("survives absent numbers from a newer server without printing NaN", () => {
    render(
      <ul>
        <LeaderboardRow
          {...base}
          sp={undefined as unknown as number}
          gamesPlayed={undefined as unknown as number}
        />
      </ul>,
    );
    const row = screen.getByTestId("leaderboard-row");
    expect(row.textContent).not.toContain("NaN");
    expect(row.querySelector('[data-testid="leaderboard-sp"]')).toHaveTextContent("0");
  });

  // P13, part 1: ONE spoken summary. The visible cells are terse by design —
  // "4", "kiro", "4,000 SP", "31" — which is a meaningless number soup read cell
  // by cell, so a single sr-only sentence names every value.
  it("exposes one screen-reader summary naming position, player, tier, SP and games", () => {
    renderRow();
    const summary = screen.getByTestId("leaderboard-row-summary");
    expect(summary.className).toContain("sr-only");
    expect(summary).toHaveTextContent("4");
    expect(summary).toHaveTextContent("kiro");
    // THE TIER, as a word. TierBadge hides itself on the grounds that the tier
    // name is rendered as text nearby; on this surface that text is here and
    // nowhere else.
    expect(summary).toHaveTextContent(i18n.t("season.tier.gold"));
    expect(summary).toHaveTextContent((4000).toLocaleString());
    expect(summary).toHaveTextContent("31");
  });

  it("names the viewer's own row as theirs in the summary", () => {
    renderRow({ isSelf: true });
    expect(screen.getByTestId("leaderboard-row-summary")).toHaveTextContent(
      i18n.t("season.leaderboard.you"),
    );
  });

  // P13, part 2: nothing competes with that summary. The earlier version put an
  // aria-label on the <li> while leaving the children exposed, which is the one
  // arrangement that can be announced twice.
  it("hides every visible cell and the badge from assistive tech", () => {
    renderRow({ isSelf: true });
    const row = screen.getByTestId("leaderboard-row");

    for (const id of [
      "leaderboard-position",
      "leaderboard-username",
      "leaderboard-sp",
      "leaderboard-games",
      "leaderboard-you",
      "leaderboard-tier-badge",
    ]) {
      expect(row.querySelector(`[data-testid="${id}"]`), id).toHaveAttribute("aria-hidden", "true");
    }
    // And no competing label on the list item itself.
    expect(row).not.toHaveAttribute("aria-label");
  });

  // P13, part 3: the tier name used to be the USERNAME cell's tooltip, so
  // hovering a player's name reported "Gold" — while the truncated name itself
  // had no tooltip at all.
  it("puts the player's own name in the username tooltip, not the tier", () => {
    renderRow();
    const name = screen.getByTestId("leaderboard-username");
    expect(name).toHaveAttribute("title", "kiro");
    expect(name.getAttribute("title")).not.toBe(i18n.t("season.tier.gold"));
  });

  it("normalizes an unknown tier token from the SP bucket", () => {
    renderRow({ tier: "mythic" });
    // 4000 SP is Gold.
    expect(screen.getByTestId("leaderboard-tier-badge")).toHaveAttribute("data-tier", "gold");
    expect(screen.getByTestId("leaderboard-row-summary")).toHaveTextContent(
      i18n.t("season.tier.gold"),
    );
  });

  it("renders the tier badge at the compact list scale", () => {
    renderRow();
    expect(screen.getByTestId("leaderboard-tier-badge").className).toContain("size-7");
  });

  it("localizes the summary", async () => {
    await i18n.changeLanguage("mk");
    renderRow();
    expect(screen.getByTestId("leaderboard-row-summary")).toHaveTextContent(
      i18n.t("season.tier.gold"),
    );
  });
});
