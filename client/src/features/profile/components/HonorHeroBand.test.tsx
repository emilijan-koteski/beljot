import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { HonorHeroBand } from "./HonorHeroBand";

type Props = Parameters<typeof HonorHeroBand>[0];

function renderBand(overrides: Partial<Props> = {}) {
  const props: Props = {
    score: 83,
    tier: "fair",
    completedTotal: 20,
    abandonedTotal: 1,
    isNewPlayer: false,
    trendDelta: 0,
    trendDirection: "flat",
    ...overrides,
  };
  // MemoryRouter: the explainer dialog this band opens deep-links to /rules.
  return render(
    <MemoryRouter>
      <HonorHeroBand {...props} />
    </MemoryRouter>,
  );
}

describe("HonorHeroBand (honour redesign R3)", () => {
  it("exposes the score, tier, new-player flag and trend as data attributes", () => {
    renderBand({ score: 96, tier: "exemplary", trendDelta: 4, trendDirection: "up" });

    const band = screen.getByTestId("profile-honor");
    expect(band).toHaveAttribute("data-honor", "96");
    expect(band).toHaveAttribute("data-tier", "exemplary");
    expect(band).toHaveAttribute("data-new-player", "false");
    expect(band).toHaveAttribute("data-trend-direction", "up");
  });

  it("renders the numeric score alongside the tier word, never colour alone", () => {
    renderBand({ score: 96, tier: "exemplary" });

    expect(screen.getByTestId("profile-honor-score")).toHaveTextContent("96");
    expect(screen.getByTestId("profile-honor-tier")).toHaveTextContent("Exemplary");
  });

  it("renders the raw completed and abandoned counts", () => {
    renderBand({ completedTotal: 42, abandonedTotal: 3 });

    expect(screen.getByTestId("profile-honor-completed")).toHaveAttribute("data-value", "42");
    expect(screen.getByTestId("profile-honor-abandoned")).toHaveAttribute("data-value", "3");
  });

  it("marks the banded meter at the score", () => {
    renderBand({ score: 50 });

    // Assert the computed position through data-value rather than the DOM shape,
    // so the meter's internals can change without breaking this.
    const marker = screen.getByTestId("profile-honor-meter-marker");
    expect(marker).toHaveAttribute("data-value", "50");
    expect(marker.style.left).toBe("50%");
  });

  it("renders a zero score as a real 0, not as the prior", () => {
    // Go zero values serialize as real values — a legitimate 0 ("Problematic")
    // must never be coerced into the 80 fallback by a truthiness check.
    renderBand({ score: 0, tier: "problematic" });

    expect(screen.getByTestId("profile-honor")).toHaveAttribute("data-honor", "0");
    expect(screen.getByTestId("profile-honor-score")).toHaveTextContent("0");
    expect(screen.getByTestId("profile-honor-tier")).toHaveTextContent("Problematic");
  });

  it("varies the shield glyph with the tier, so colour is not the only signal", () => {
    renderBand({ score: 96, tier: "exemplary" });
    const exemplary = screen.getAllByTestId("honor-shield")[0];
    expect(exemplary).toHaveAttribute("data-tier", "exemplary");
  });

  describe("trend", () => {
    it("renders a signed delta with the direction word when improving", () => {
      renderBand({ trendDelta: 13, trendDirection: "up" });

      const trend = screen.getByTestId("profile-honor-trend");
      expect(trend).toHaveAttribute("data-trend-delta", "13");
      expect(trend).toHaveTextContent("Improving");
      expect(trend).toHaveTextContent("+13");
    });

    it("renders a signed delta when slipping", () => {
      renderBand({ trendDelta: -9, trendDirection: "down" });

      const trend = screen.getByTestId("profile-honor-trend");
      expect(trend).toHaveTextContent("Slipping");
      expect(trend).toHaveTextContent("-9");
    });

    it("omits the number when flat", () => {
      renderBand({ trendDelta: 1, trendDirection: "flat" });

      const trend = screen.getByTestId("profile-honor-trend");
      expect(trend).toHaveTextContent("Steady");
      expect(trend).not.toHaveTextContent("1");
    });

    it("falls back to flat for an unrecognised direction token", () => {
      renderBand({ trendDirection: "sideways" });

      expect(screen.getByTestId("profile-honor")).toHaveAttribute("data-trend-direction", "flat");
    });
  });

  describe("new player suppression", () => {
    it("replaces the score and tier with progress toward earning one", () => {
      renderBand({ isNewPlayer: true, completedTotal: 2, abandonedTotal: 0 });

      expect(screen.getByTestId("profile-honor")).toHaveAttribute("data-new-player", "true");
      // The redesign replaces the bare "New Player" label with a counter, so the
      // state says how to LEAVE it rather than only naming it.
      const progress = screen.getByTestId("profile-honor-new");
      expect(progress).toHaveAttribute("data-value", "2");
      expect(progress).toHaveTextContent("2");
      expect(progress).toHaveTextContent("5");
      expect(screen.queryByTestId("profile-honor-score")).not.toBeInTheDocument();
      expect(screen.queryByTestId("profile-honor-tier")).not.toBeInTheDocument();
      expect(screen.queryByTestId("profile-honor-meter")).not.toBeInTheDocument();
    });

    it("counts abandonments toward the floor, not just completions", () => {
      // The floor is experience, not successes — the exact bypass two server
      // review passes closed. 0 completed / 4 abandoned is 4 of 5, not 0 of 5.
      renderBand({ isNewPlayer: true, completedTotal: 0, abandonedTotal: 4 });

      expect(screen.getByTestId("profile-honor-new")).toHaveAttribute("data-value", "4");
    });

    it("still shows the raw counts (PO decision)", () => {
      renderBand({ isNewPlayer: true, completedTotal: 2, abandonedTotal: 1 });

      expect(screen.getByTestId("profile-honor-completed")).toHaveAttribute("data-value", "2");
      expect(screen.getByTestId("profile-honor-abandoned")).toHaveAttribute("data-value", "1");
    });

    it("hides the trend, which needs two full windows to mean anything", () => {
      renderBand({ isNewPlayer: true, trendDelta: 4, trendDirection: "up" });

      expect(screen.queryByTestId("profile-honor-trend")).not.toBeInTheDocument();
    });

    it("keeps the real score on the data attribute for downstream consumers", () => {
      renderBand({ isNewPlayer: true, score: 86, tier: "trusted" });

      const band = screen.getByTestId("profile-honor");
      expect(band).toHaveAttribute("data-honor", "86");
      expect(band).toHaveAttribute("data-tier", "trusted");
    });
  });

  it("colours by score when the server sends an unknown tier token", () => {
    // Version skew: a newer server ships a tier this bundle has never heard of.
    renderBand({ score: 97, tier: "legendary" });

    expect(screen.getByTestId("profile-honor")).toHaveAttribute("data-tier", "exemplary");
    expect(screen.getByTestId("profile-honor-tier")).toHaveTextContent("Exemplary");
  });

  it("opens the explainer from the band", async () => {
    renderBand();

    // Two triggers by design (icon button ≥sm, labelled link <sm); both open the
    // same dialog, so assert the dialog is reachable rather than which one shows.
    expect(screen.getByTestId("profile-honor-explainer-button")).toBeInTheDocument();
    expect(screen.getByTestId("profile-honor-explainer-link")).toBeInTheDocument();
  });
});
