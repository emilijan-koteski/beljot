import "@/shared/i18n/i18n";

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { SeasonArchiveRow } from "@/shared/components/season/SeasonArchiveRow";
import { i18n } from "@/shared/i18n/i18n";

function renderRow(over: Partial<Parameters<typeof SeasonArchiveRow>[0]> = {}) {
  return render(
    <ul>
      <SeasonArchiveRow
        seasonName="2026 Q2"
        sp={1800}
        tier="silver"
        division={null}
        gamesPlayed={14}
        {...over}
      />
    </ul>,
  );
}

describe("SeasonArchiveRow", () => {
  afterEach(async () => {
    cleanup();
    await i18n.changeLanguage("en");
  });

  it("renders the season token verbatim, the stored tier, the SP and the games", () => {
    renderRow();

    expect(screen.getByTestId("season-archive-name").textContent).toBe("2026 Q2");
    expect(screen.getByTestId("season-archive-tier").textContent).toBe("Silver");
    expect(screen.getByTestId("season-archive-sp").textContent).toContain((1800).toLocaleString());
    expect(screen.getByTestId("season-archive-games").textContent).toBe("14");
    expect(screen.getByTestId("season-archive-row")).toHaveAttribute("data-tier", "silver");
  });

  it("renders the tier badge at the small scale with the tier attached", () => {
    renderRow();

    const badge = screen.getByTestId("season-archive-tier-badge");
    expect(badge.className).toContain("size-7");
    expect(badge).toHaveAttribute("data-tier", "silver");
    expect(badge).toHaveAttribute("aria-hidden", "true");
  });

  it("carries one sr-only summary and hides every visible cell from AT", () => {
    renderRow();

    const summary = screen.getByTestId("season-archive-row-summary");
    expect(summary.className).toContain("sr-only");
    expect(summary.textContent).toContain("2026 Q2");
    expect(summary.textContent).toContain("Silver");
    expect(summary.textContent).toContain((1800).toLocaleString());
    expect(summary.textContent).toContain("14");

    for (const cell of [
      "season-archive-name",
      "season-archive-tier",
      "season-archive-sp",
      "season-archive-games",
    ]) {
      expect(screen.getByTestId(cell)).toHaveAttribute("aria-hidden", "true");
    }
  });

  it("renders a stored division as tier plus division", () => {
    renderRow({ seasonName: "2026 Q4", sp: 900, tier: "platinum", division: 2 });

    expect(screen.getByTestId("season-archive-tier").textContent).toBe("Platinum 2");
    expect(screen.getByTestId("season-archive-row-summary").textContent).toContain(
      "2026 Q4: Platinum 2,",
    );
  });

  it("renders a 0-SP season as Iron 1 with a real zero", () => {
    renderRow({ sp: 0, tier: "iron", division: 1, gamesPlayed: 2 });

    expect(screen.getByTestId("season-archive-tier").textContent).toBe("Iron 1");
    expect(screen.getByTestId("season-archive-sp").textContent).toContain("0");
    expect(screen.getByTestId("season-archive-row")).toHaveAttribute("data-tier", "iron");
  });

  it("falls back to the SP bucket for an unrecognised tier token", () => {
    renderRow({ tier: "mythic", sp: 700 });

    expect(screen.getByTestId("season-archive-row")).toHaveAttribute("data-tier", "gold");
    expect(screen.getByTestId("season-archive-tier").textContent).toBe("Gold");
  });

  it("renders the localized tier label in mk while keeping the token verbatim", async () => {
    await i18n.changeLanguage("mk");
    renderRow();

    expect(screen.getByTestId("season-archive-tier").textContent).toBe(
      i18n.t("season.tier.silver"),
    );
    expect(screen.getByTestId("season-archive-name").textContent).toBe("2026 Q2");
  });
});
