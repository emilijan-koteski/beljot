import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Badge } from "./badge";

describe("Badge", () => {
  it("renders its label", () => {
    render(<Badge data-testid="badge">Us</Badge>);

    expect(screen.getByTestId("badge")).toHaveTextContent("Us");
  });

  it("carries a tint colour as a custom property", () => {
    render(
      <Badge tint="#e8c25a" data-testid="badge">
        Us
      </Badge>,
    );

    expect(screen.getByTestId("badge").style.getPropertyValue("--badge-tint")).toBe("#e8c25a");
  });

  it("leaves the style alone without a tint", () => {
    render(
      <Badge tone="brass" style={{ marginTop: 4 }} data-testid="badge">
        Season
      </Badge>,
    );

    const badge = screen.getByTestId("badge");
    expect(badge.style.getPropertyValue("--badge-tint")).toBe("");
    expect(badge.style.marginTop).toBe("4px");
  });
});
