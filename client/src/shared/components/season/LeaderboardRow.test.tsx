import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
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

// The username cell is a <Link> since the row became a way into a player's
// profile, so every render needs a router around it.
function renderRow(props: Partial<typeof base> & { isSelf?: boolean } = {}) {
  return render(
    <MemoryRouter>
      <ul>
        <LeaderboardRow {...base} {...props} />
      </ul>
    </MemoryRouter>,
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
      <MemoryRouter>
        <ul>
          <LeaderboardRow {...base} gamesPlayed={undefined} />
        </ul>
      </MemoryRouter>,
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
      <MemoryRouter>
        <ul>
          <LeaderboardRow
            {...base}
            sp={undefined as unknown as number}
            gamesPlayed={undefined as unknown as number}
          />
        </ul>
      </MemoryRouter>,
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
      "leaderboard-tier",
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

  // THE ONE EXCEPTION, and it is not a slip: the username is a focusable link,
  // and aria-hidden on a focusable element hides it from the accessibility tree
  // while leaving it in the tab order — a keyboard screen-reader user would land
  // on a control that announces nothing. It carries its own label instead.
  it("leaves the username link exposed to assistive tech, with its own label", () => {
    renderRow();
    const name = screen.getByTestId("leaderboard-username");
    expect(name).not.toHaveAttribute("aria-hidden");
    expect(name).toHaveAttribute(
      "aria-label",
      i18n.t("friends.viewProfileAria", { username: "kiro" }),
    );
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

  // --- the tier NAME, beside the badge ---

  // The badge alone asked every reader to have memorised eight ramp colours.
  it("renders the tier name as text, in the tier's own colour", () => {
    renderRow();
    const tier = screen.getByTestId("leaderboard-tier");
    expect(tier.textContent).toBe(i18n.t("season.tier.gold"));
    expect(tier.getAttribute("style")).toContain("--rt4");
  });

  it("localizes the visible tier name", async () => {
    await i18n.changeLanguage("mk");
    renderRow();
    expect(screen.getByTestId("leaderboard-tier").textContent).toBe(i18n.t("season.tier.gold"));
  });

  it("shows the normalized tier name for an unknown token, never the raw string", () => {
    renderRow({ tier: "mythic" });
    expect(screen.getByTestId("leaderboard-tier").textContent).toBe(i18n.t("season.tier.gold"));
  });

  // --- the username link ---

  it("links another player's name to their public profile", () => {
    renderRow();
    expect(screen.getByTestId("leaderboard-username")).toHaveAttribute("href", "/players/42");
  });

  // THE SELF ROW GOES TO /profile, not to /players/<own id>: the self page is
  // the richer surface (linked accounts, deck picker, editable username), and
  // the public one would show the viewer a read-only copy of themselves with no
  // friend button.
  it("links the viewer's own row to their own profile page", () => {
    renderRow({ isSelf: true });
    const name = screen.getByTestId("leaderboard-username");
    expect(name).toHaveAttribute("href", "/profile");
    expect(name).toHaveAttribute("aria-label", i18n.t("season.leaderboard.yourProfileAria"));
  });

  // The pinned own-row falls back to "You" as its username when the auth store
  // is unhydrated, which is exactly why the self label is its own key rather
  // than that name interpolated into "View {{username}}'s profile" — which
  // would read "View You's profile".
  it("keeps the fixed self label even when the row's username is the generic You", () => {
    renderRow({ isSelf: true, username: i18n.t("season.leaderboard.you") });
    const name = screen.getByTestId("leaderboard-username");
    expect(name).toHaveAttribute("aria-label", i18n.t("season.leaderboard.yourProfileAria"));
    expect(name.getAttribute("aria-label")).not.toBe(
      i18n.t("friends.viewProfileAria", { username: i18n.t("season.leaderboard.you") }),
    );
  });
});
