import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { formatCoins } from "@/shared/lib/formatCoins";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => (key === "match.stake.label" ? "Stake" : key),
  }),
}));

import { StakePill } from "./StakePill";

describe("StakePill", () => {
  it("renders the formatted stake amount", () => {
    render(<StakePill amount={2000} />);
    expect(screen.getByTestId("match-stake-amount")).toHaveTextContent(formatCoins(2000));
  });

  it("exposes a localized aria-label including the amount", () => {
    render(<StakePill amount={2000} />);
    expect(screen.getByTestId("match-stake")).toHaveAttribute(
      "aria-label",
      `Stake: ${formatCoins(2000)}`,
    );
  });

  it("renders a coin glyph alongside the amount", () => {
    const { container } = render(<StakePill amount={500} />);
    // lucide-react renders the coin glyph as an <svg>.
    expect(container.querySelector("svg")).not.toBeNull();
    expect(screen.getByTestId("match-stake-amount")).toHaveTextContent(formatCoins(500));
  });
});
