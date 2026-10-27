import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SummaryStrip } from "./SummaryStrip";

describe("SummaryStrip", () => {
  it("renders the label, caption and value", () => {
    render(
      <SummaryStrip
        label="Total"
        caption="Added to this hand's score"
        value="+70"
        testId="strip"
        valueTestId="strip-value"
      />,
    );

    const strip = screen.getByTestId("strip");
    expect(strip).toHaveTextContent("Total");
    expect(strip).toHaveTextContent("Added to this hand's score");
    expect(screen.getByTestId("strip-value")).toHaveTextContent("+70");
  });

  it("leaves the caption out when none is given", () => {
    render(<SummaryStrip label="Total" value="+20" testId="strip" />);

    expect(screen.getByTestId("strip")).toHaveTextContent(/^Total\+20$/);
  });
});
