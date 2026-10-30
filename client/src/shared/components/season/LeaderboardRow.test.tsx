import "@/shared/i18n/i18n";

import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it } from "vitest";

import { LeaderboardRow, LeaderboardTableHeader } from "@/shared/components/season/LeaderboardRow";
import { i18n } from "@/shared/i18n/i18n";

const base = {
  position: 4,
  userId: 42,
  username: "kiro",
  sp: 700,
  tier: "gold",
  division: 2 as number | null,
  gamesPlayed: 31,
  gamesWon: 19,
};

type RowProps = Partial<typeof base> & {
  isSelf?: boolean;
  avatarUrl?: string | null;
  layout?: "list" | "table";
  showRank?: boolean;
};

// The username cell is a <Link> since the row became a way into a player's
// profile, so every render needs a router around it.
function renderRow(props: RowProps = {}) {
  return render(
    <MemoryRouter>
      <ul>
        <LeaderboardRow {...base} {...props} />
      </ul>
    </MemoryRouter>,
  );
}

afterEach(async () => {
  cleanup();
  await i18n.changeLanguage("en");
});

describe("LeaderboardRow", () => {
  it("renders position, username, SP and the season record", () => {
    renderRow();
    const row = screen.getByTestId("leaderboard-row");
    expect(row.querySelector('[data-testid="leaderboard-position"]')).toHaveTextContent("4");
    expect(row.querySelector('[data-testid="leaderboard-username"]')).toHaveTextContent("kiro");
    expect(row.querySelector('[data-testid="leaderboard-sp"]')).toHaveTextContent(
      (700).toLocaleString(),
    );
    expect(row.querySelector('[data-testid="leaderboard-record"]')?.textContent).toBe("19–12");
    expect(row.querySelector('[data-testid="leaderboard-win-rate"]')).toHaveTextContent("61%");
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

  it("omits the record when no games count is supplied", () => {
    renderRow({ gamesPlayed: undefined });
    expect(screen.queryByTestId("leaderboard-record")).not.toBeInTheDocument();
    expect(screen.queryByTestId("leaderboard-win-rate")).not.toBeInTheDocument();
  });

  it("renders a zero record rather than dropping it", () => {
    renderRow({ gamesPlayed: 0, gamesWon: 0 });
    expect(screen.getByTestId("leaderboard-record").textContent).toBe("0–0");
    expect(screen.getByTestId("leaderboard-win-rate")).toHaveTextContent("0%");
  });

  it("never counts more wins than games played", () => {
    renderRow({ gamesPlayed: 31, gamesWon: 40 });
    expect(screen.getByTestId("leaderboard-record").textContent).toBe("31–0");
    expect(screen.getByTestId("leaderboard-win-rate")).toHaveTextContent("100%");
  });

  it("survives absent numbers from a newer server without printing NaN", () => {
    renderRow({
      sp: undefined as unknown as number,
      gamesPlayed: undefined as unknown as number,
      gamesWon: undefined as unknown as number,
    });
    const row = screen.getByTestId("leaderboard-row");
    expect(row.textContent).not.toContain("NaN");
    expect(row.querySelector('[data-testid="leaderboard-sp"]')).toHaveTextContent("0");
  });

  it("exposes one screen-reader summary naming position, player, rank, SP and record", () => {
    renderRow();
    const summary = screen.getByTestId("leaderboard-row-summary");
    expect(summary.className).toContain("sr-only");
    expect(summary).toHaveTextContent("4");
    expect(summary).toHaveTextContent("kiro, Gold 2,");
    expect(summary).toHaveTextContent((700).toLocaleString());
    expect(summary).toHaveTextContent("Wins: 19, losses: 12");
  });

  it("names the viewer's own row as theirs in the summary", () => {
    renderRow({ isSelf: true });
    expect(screen.getByTestId("leaderboard-row-summary")).toHaveTextContent(
      i18n.t("season.leaderboard.you"),
    );
  });

  it("hides every visible cell from assistive tech in the list layout", () => {
    renderRow({ isSelf: true });
    const row = screen.getByTestId("leaderboard-row");
    for (const id of [
      "leaderboard-position",
      "leaderboard-tier",
      "leaderboard-sp",
      "leaderboard-record",
      "leaderboard-win-rate",
      "leaderboard-you",
    ]) {
      const cell = row.querySelector(`[data-testid="${id}"]`);
      expect(cell, id).not.toBeNull();
      expect(cell?.closest('[aria-hidden="true"]'), id).not.toBeNull();
    }
    expect(row).not.toHaveAttribute("aria-label");
  });

  it("hides every visible cell and the badge from assistive tech in the table layout", () => {
    renderRow({ isSelf: true, layout: "table", showRank: true });
    const row = screen.getByTestId("leaderboard-row");
    for (const id of [
      "leaderboard-position",
      "leaderboard-tier",
      "leaderboard-tier-badge",
      "leaderboard-sp",
      "leaderboard-record",
      "leaderboard-win-rate",
      "leaderboard-you",
    ]) {
      const cell = row.querySelector(`[data-testid="${id}"]`);
      expect(cell, id).not.toBeNull();
      expect(cell?.closest('[aria-hidden="true"]'), id).not.toBeNull();
    }
  });

  it("leaves the username link exposed to assistive tech, with its own label", () => {
    renderRow();
    const name = screen.getByTestId("leaderboard-username");
    expect(name.closest('[aria-hidden="true"]')).toBeNull();
    expect(name).toHaveAttribute(
      "aria-label",
      i18n.t("friends.viewProfileAria", { username: "kiro" }),
    );
  });

  it("puts the player's own name in the username tooltip, not the tier", () => {
    renderRow();
    const name = screen.getByTestId("leaderboard-username");
    expect(name).toHaveAttribute("title", "kiro");
    expect(name.getAttribute("title")).not.toBe(i18n.t("season.tier.gold"));
  });

  it("normalizes an unknown tier token from the SP bucket", () => {
    renderRow({ tier: "mythic", sp: 700, layout: "table", showRank: true });
    expect(screen.getByTestId("leaderboard-tier-badge")).toHaveAttribute("data-tier", "gold");
    expect(screen.getByTestId("leaderboard-row-summary")).toHaveTextContent("Gold 2");
  });

  it("keeps the tier badge out of the phone list", () => {
    renderRow();
    expect(screen.queryByTestId("leaderboard-tier-badge")).not.toBeInTheDocument();
  });

  it("renders the tier badge at the compact scale in the table's rank column", () => {
    renderRow({ layout: "table", showRank: true });
    expect(screen.getByTestId("leaderboard-tier-badge").className).toContain("size-7");
  });

  it("renders the record and win rate as table cells on wider screens", () => {
    renderRow({ layout: "table" });
    expect(screen.getByTestId("leaderboard-record").textContent).toBe("19–12");
    expect(screen.getByTestId("leaderboard-win-rate")).toHaveTextContent("61%");
  });

  it("localizes the summary", async () => {
    await i18n.changeLanguage("mk");
    renderRow();
    expect(screen.getByTestId("leaderboard-row-summary")).toHaveTextContent("Злато 2");
  });

  it("writes the SP unit in the viewer's language", async () => {
    await i18n.changeLanguage("mk");
    renderRow();
    expect(screen.getByTestId("leaderboard-sp")).toHaveTextContent("СП");
  });

  it("renders the rank as text, tier plus division, in the tier's own colour", () => {
    renderRow();
    const tier = screen.getByTestId("leaderboard-tier");
    expect(tier.textContent).toBe("Gold 2");
    expect(tier.getAttribute("style")).toContain("--rt4");
  });

  it("renders a single tier by name alone", () => {
    renderRow({ sp: 1250, tier: "master", division: null });
    expect(screen.getByTestId("leaderboard-tier").textContent).toBe("Master");
  });

  it("renders an ended pre-division row as its stored tier, with no division", () => {
    renderRow({ sp: 3500, tier: "gold", division: null, layout: "table", showRank: true });
    expect(screen.getByTestId("leaderboard-tier").textContent).toBe("Gold");
    expect(screen.getByTestId("leaderboard-tier-badge")).toHaveAttribute("data-tier", "gold");
  });

  it("localizes the visible rank", async () => {
    await i18n.changeLanguage("mk");
    renderRow();
    expect(screen.getByTestId("leaderboard-tier").textContent).toBe("Злато 2");
  });

  it("shows the normalized tier name for an unknown token, never the raw string", () => {
    renderRow({ tier: "mythic", sp: 700 });
    expect(screen.getByTestId("leaderboard-tier").textContent).toBe("Gold 2");
  });

  it("links another player's name to their public profile", () => {
    renderRow();
    expect(screen.getByTestId("leaderboard-username")).toHaveAttribute("href", "/players/42");
  });

  it("links the viewer's own row to their own profile page", () => {
    renderRow({ isSelf: true });
    const name = screen.getByTestId("leaderboard-username");
    expect(name).toHaveAttribute("href", "/profile");
    expect(name).toHaveAttribute("aria-label", i18n.t("season.leaderboard.yourProfileAria"));
  });

  it("keeps the fixed self label even when the row's username is the generic You", () => {
    renderRow({ isSelf: true, username: i18n.t("season.leaderboard.you") });
    const name = screen.getByTestId("leaderboard-username");
    expect(name).toHaveAttribute("aria-label", i18n.t("season.leaderboard.yourProfileAria"));
    expect(name.getAttribute("aria-label")).not.toBe(
      i18n.t("friends.viewProfileAria", { username: i18n.t("season.leaderboard.you") }),
    );
  });
});

describe("LeaderboardTableHeader", () => {
  it("adds the rank column only when the table shows ranks", () => {
    const { rerender } = render(<LeaderboardTableHeader />);
    expect(screen.queryByText(i18n.t("season.leaderboard.columns.tier"))).not.toBeInTheDocument();
    rerender(<LeaderboardTableHeader showRank />);
    expect(screen.getByText(i18n.t("season.leaderboard.columns.tier"))).toBeInTheDocument();
    expect(screen.getByText(i18n.t("season.leaderboard.columns.record"))).toBeInTheDocument();
  });
});

describe("LeaderboardRow avatar", () => {
  it("draws the player's picture in a lazy disc beside the name", () => {
    const url = "https://assets.test/avatars/k/128.webp";
    renderRow({ avatarUrl: url });
    const img = screen.getByTestId("leaderboard-row").querySelector("img");
    expect(img).toHaveAttribute("src", url);
    expect(img).toHaveAttribute("loading", "lazy");
    expect(img).toHaveAttribute("width", "34");
  });

  it("falls back to the initial without one", () => {
    renderRow({ avatarUrl: null });
    const row = screen.getByTestId("leaderboard-row");
    expect(row.querySelector("img")).toBeNull();
  });
});
