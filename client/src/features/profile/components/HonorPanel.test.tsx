import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { HonorPanel } from "./HonorPanel";

type HonorPanelProps = Parameters<typeof HonorPanel>[0];

function renderPanel(overrides: Partial<HonorPanelProps> = {}) {
  const props: HonorPanelProps = {
    score: 83,
    tier: "fair",
    completedTotal: 20,
    abandonedTotal: 1,
    isNewPlayer: false,
    trendDelta: 0,
    trendDirection: "flat",
    ...overrides,
  };
  return render(<HonorPanel {...props} />);
}

describe("HonorPanel (Story 9.7)", () => {
  it("exposes the score, tier, new-player flag and trend as data attributes", () => {
    renderPanel({ score: 96, tier: "exemplary", trendDelta: 4, trendDirection: "up" });

    const panel = screen.getByTestId("profile-honor");
    expect(panel).toHaveAttribute("data-honor", "96");
    expect(panel).toHaveAttribute("data-tier", "exemplary");
    expect(panel).toHaveAttribute("data-new-player", "false");
    expect(panel).toHaveAttribute("data-trend-direction", "up");
  });

  it("renders the numeric score alongside the tier word, never colour alone", () => {
    renderPanel({ score: 96, tier: "exemplary" });

    expect(screen.getByTestId("profile-honor-score")).toHaveTextContent("96");
    expect(screen.getByTestId("profile-honor-tier")).toHaveTextContent("Exemplary");
  });

  it("renders the raw completed and abandoned counts", () => {
    renderPanel({ completedTotal: 42, abandonedTotal: 3 });

    expect(screen.getByTestId("profile-honor-completed")).toHaveAttribute("data-value", "42");
    expect(screen.getByTestId("profile-honor-abandoned")).toHaveAttribute("data-value", "3");
  });

  it("fills the meter in proportion to the score", () => {
    renderPanel({ score: 50 });

    const fill = screen.getByTestId("profile-honor-meter").firstElementChild as HTMLElement;
    expect(fill.style.width).toBe("50%");
  });

  it("renders a zero score as a real 0, not as the prior", () => {
    // Go zero values serialize as real values — a legitimate 0 ("Problematic")
    // must never be coerced into the 80 fallback by a truthiness check.
    renderPanel({ score: 0, tier: "problematic" });

    expect(screen.getByTestId("profile-honor")).toHaveAttribute("data-honor", "0");
    expect(screen.getByTestId("profile-honor-score")).toHaveTextContent("0");
    expect(screen.getByTestId("profile-honor-tier")).toHaveTextContent("Problematic");
  });

  describe("trend", () => {
    it("renders a signed delta with the direction word when improving", () => {
      renderPanel({ trendDelta: 13, trendDirection: "up" });

      const trend = screen.getByTestId("profile-honor-trend");
      expect(trend).toHaveAttribute("data-trend-delta", "13");
      expect(trend).toHaveTextContent("Improving");
      expect(trend).toHaveTextContent("+13");
    });

    it("renders a signed delta when slipping", () => {
      renderPanel({ trendDelta: -9, trendDirection: "down" });

      const trend = screen.getByTestId("profile-honor-trend");
      expect(trend).toHaveTextContent("Slipping");
      expect(trend).toHaveTextContent("-9");
    });

    it("omits the number when flat", () => {
      renderPanel({ trendDelta: 1, trendDirection: "flat" });

      const trend = screen.getByTestId("profile-honor-trend");
      expect(trend).toHaveTextContent("Steady");
      expect(trend).not.toHaveTextContent("1");
    });

    it("falls back to flat for an unrecognised direction token", () => {
      renderPanel({ trendDirection: "sideways" });

      expect(screen.getByTestId("profile-honor")).toHaveAttribute("data-trend-direction", "flat");
    });
  });

  describe("new player suppression", () => {
    it("replaces the score and tier with a New Player chip", () => {
      renderPanel({ isNewPlayer: true, completedTotal: 2, abandonedTotal: 0 });

      expect(screen.getByTestId("profile-honor")).toHaveAttribute("data-new-player", "true");
      expect(screen.getByTestId("profile-honor-new")).toHaveTextContent("New Player");
      expect(screen.queryByTestId("profile-honor-score")).not.toBeInTheDocument();
      expect(screen.queryByTestId("profile-honor-tier")).not.toBeInTheDocument();
      expect(screen.queryByTestId("profile-honor-meter")).not.toBeInTheDocument();
    });

    it("still shows the raw counts (PO decision)", () => {
      renderPanel({ isNewPlayer: true, completedTotal: 2, abandonedTotal: 1 });

      expect(screen.getByTestId("profile-honor-completed")).toHaveAttribute("data-value", "2");
      expect(screen.getByTestId("profile-honor-abandoned")).toHaveAttribute("data-value", "1");
    });

    it("keeps the real score on the data attribute for downstream consumers", () => {
      renderPanel({ isNewPlayer: true, score: 86, tier: "trusted" });

      const panel = screen.getByTestId("profile-honor");
      expect(panel).toHaveAttribute("data-honor", "86");
      expect(panel).toHaveAttribute("data-tier", "trusted");
    });
  });

  it("colours by score when the server sends an unknown tier token", () => {
    // Version skew: a newer server ships a tier this bundle has never heard of.
    renderPanel({ score: 97, tier: "legendary" });

    expect(screen.getByTestId("profile-honor")).toHaveAttribute("data-tier", "exemplary");
    expect(screen.getByTestId("profile-honor-tier")).toHaveTextContent("Exemplary");
  });
});
