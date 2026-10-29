import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PageShell } from "./PageShell";

describe("PageShell", () => {
  it("renders the page alone when given no sidebar or bottom bar", () => {
    render(
      <PageShell testId="page">
        <p>content</p>
      </PageShell>,
    );
    expect(screen.getByTestId("page")).toHaveTextContent("content");
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
    expect(screen.queryByTestId("page-bottom-bar")).not.toBeInTheDocument();
  });

  it("adds a sidebar and a bottom bar when given them", () => {
    render(
      <PageShell sidebar={<p>friends</p>} bottomBar={<button type="button">New room</button>}>
        <p>content</p>
      </PageShell>,
    );
    expect(screen.getByRole("complementary")).toHaveTextContent("friends");
    expect(screen.getByTestId("page-bottom-bar")).toHaveTextContent("New room");
  });
});
