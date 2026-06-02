import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DealerIndicator } from "./DealerIndicator";

describe("DealerIndicator", () => {
  it("renders the dealer's name", () => {
    render(<DealerIndicator dealerName="Ana" />);
    const indicator = screen.getByTestId("dealer-indicator");
    expect(indicator).toBeInTheDocument();
    expect(screen.getByTestId("dealer-name").textContent).toBe("Ana");
  });

  it("has aria-live polite for screen reader updates", () => {
    render(<DealerIndicator dealerName="Ana" />);
    expect(screen.getByTestId("dealer-indicator")).toHaveAttribute("aria-live", "polite");
  });

  it("has an aria-label set on the indicator", () => {
    render(<DealerIndicator dealerName="Ana" />);
    expect(screen.getByTestId("dealer-indicator")).toHaveAttribute("aria-label");
  });

  it("renders nothing when dealerName is blank", () => {
    const { container } = render(<DealerIndicator dealerName="   " />);
    expect(container.firstChild).toBeNull();
    expect(screen.queryByTestId("dealer-indicator")).not.toBeInTheDocument();
  });

  it("dealer name span has truncate clamp (AC6)", () => {
    render(<DealerIndicator dealerName="aVeryLongUsernameThatShouldClamp" />);
    const nameEl = screen.getByTestId("dealer-name");
    expect(nameEl.className).toContain("max-w-32");
    expect(nameEl.className).toContain("truncate");
  });
});
