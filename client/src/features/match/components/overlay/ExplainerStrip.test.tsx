import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ExplainerStrip } from "./ExplainerStrip";

describe("ExplainerStrip", () => {
  it("renders its text as a note in the brass tone by default", () => {
    render(<ExplainerStrip>Scores 20 points</ExplainerStrip>);

    const note = screen.getByRole("note");
    expect(note).toHaveTextContent("Scores 20 points");
    expect(note).toHaveAttribute("data-tone", "brass");
  });

  it("takes the danger tone for a consequence", () => {
    render(<ExplainerStrip tone="danger">Uses your one surrender</ExplainerStrip>);

    expect(screen.getByRole("note")).toHaveAttribute("data-tone", "danger");
  });
});
