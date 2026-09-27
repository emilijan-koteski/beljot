import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { i18n } from "@/shared/i18n/i18n";
import type { SeasonSettlement } from "@/shared/stores/matchStore";

import { SeasonSpLine } from "./SeasonSpLine";

const MINUS = "−";

function renderLine(over: Partial<SeasonSettlement> = {}) {
  const settlement: SeasonSettlement = {
    spChange: 24,
    newSeasonSp: 690,
    rankTier: "gold",
    rankDivision: 2,
    reason: "normal",
    ...over,
  };
  return render(<SeasonSpLine settlement={settlement} />);
}

describe("SeasonSpLine", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("shows a signed gain and the rank it lands on, in the accent colour", () => {
    renderLine();

    const line = screen.getByTestId("season-sp-line");
    expect(line).toHaveTextContent("+24 SP · Gold 2");
    expect(line).toHaveAttribute("data-trend", "gain");
    expect(line.getAttribute("style")).toContain("var(--accent)");
    expect(screen.getByTestId("season-sp-line-badge")).toHaveAttribute("data-tier", "gold");
  });

  // The matrix's tier-down row. The change is NEVER clamped to 0, and a loss is
  // muted, never red.
  it("shows a loss as a real negative with U+2212, muted", () => {
    renderLine({ spChange: -20, newSeasonSp: 590, rankTier: "silver", rankDivision: 3 });

    const line = screen.getByTestId("season-sp-line");
    expect(line).toHaveTextContent(`${MINUS}20 SP · Silver 3`);
    expect(line.textContent).not.toContain("-20");
    expect(line).toHaveAttribute("data-trend", "loss");
    const style = line.getAttribute("style") ?? "";
    expect(style).toContain("var(--ink-mute)");
    expect(style).not.toContain("var(--accent)");
    expect(style).not.toMatch(/#ff|red|destructive|danger/i);
  });

  // The matrix's loss-at-0 row: a real zero, a real rank.
  it("shows a loss at 0 SP as 0 SP at Iron 1", () => {
    renderLine({ spChange: 0, newSeasonSp: 0, rankTier: "iron", rankDivision: 1 });

    const line = screen.getByTestId("season-sp-line");
    expect(line).toHaveTextContent("0 SP · Iron 1");
    expect(line).toHaveAttribute("data-trend", "none");
    expect(line.getAttribute("style")).toContain("var(--ink-mute)");
  });

  it("names a single tier without a division", () => {
    renderLine({ spChange: 10, newSeasonSp: 1250, rankTier: "master", rankDivision: null });
    expect(screen.getByTestId("season-sp-line")).toHaveTextContent("+10 SP · Master");
  });

  // The matrix's partner-abandoned row.
  it("tells the abandoner's teammate it was half a loss", () => {
    renderLine({ spChange: -12, reason: "partner_abandoned" });

    const line = screen.getByTestId("season-sp-line");
    expect(line).toHaveTextContent(`Partner abandoned: ${MINUS}12 SP (half loss)`);
    expect(line).toHaveAttribute("data-reason", "partner_abandoned");
    // The copy names no rank, so the decorative badge stays out.
    expect(screen.queryByTestId("season-sp-line-badge")).not.toBeInTheDocument();
  });

  // The half-loss copy needs a real loss. A teammate on the 0 floor applies 0.
  it("gives a partner whose applied change is 0 the ordinary line", () => {
    renderLine({
      spChange: 0,
      newSeasonSp: 0,
      rankTier: "iron",
      rankDivision: 1,
      reason: "partner_abandoned",
    });

    const line = screen.getByTestId("season-sp-line");
    expect(line).toHaveTextContent("0 SP · Iron 1");
    expect(line.textContent).not.toContain("half loss");
    expect(line).toHaveAttribute("data-trend", "none");
  });

  // A Capot bonus (+5) over a small half loss can leave the partner positive.
  it("gives a partner with a positive change the ordinary line, never 'half loss'", () => {
    renderLine({ spChange: 3, newSeasonSp: 703, reason: "partner_abandoned" });

    const line = screen.getByTestId("season-sp-line");
    expect(line).toHaveTextContent("+3 SP · Gold 2");
    expect(line.textContent).not.toContain("Partner abandoned");
    expect(line).toHaveAttribute("data-trend", "gain");
  });

  it("is a live region, since it usually lands after its result screen", () => {
    renderLine();
    expect(screen.getByRole("status")).toBe(screen.getByTestId("season-sp-line"));
  });

  it("states the abandoner's own penalty plainly", () => {
    renderLine({ spChange: -120, newSeasonSp: 380, reason: "abandoned" });

    expect(screen.getByTestId("season-sp-line")).toHaveTextContent(`You abandoned: ${MINUS}120 SP`);
  });

  it("localizes the line, with SP as СП in Macedonian", async () => {
    await i18n.changeLanguage("mk");
    renderLine({ spChange: -13, newSeasonSp: 590, rankTier: "silver", rankDivision: 3 });

    expect(screen.getByTestId("season-sp-line")).toHaveTextContent(`${MINUS}13 СП · Сребро 3`);
  });

  it("uses the noun-phrase abandonment copy in Croatian", async () => {
    await i18n.changeLanguage("hr");
    renderLine({ spChange: -120, reason: "abandoned" });

    expect(screen.getByTestId("season-sp-line")).toHaveTextContent(
      `Napuštanje meča: ${MINUS}120 SP`,
    );
  });
});
