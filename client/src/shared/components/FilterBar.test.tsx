import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { FilterBar } from "./FilterBar";

function Harness({ collapsed, initial = "" }: { collapsed: boolean; initial?: string }) {
  const [search, setSearch] = useState(initial);
  return (
    <FilterBar
      search={search}
      onSearchChange={setSearch}
      searchPlaceholder="Room name or code"
      openSearchLabel="Search rooms"
      closeSearchLabel="Close search"
      collapsed={collapsed}
      filters={<span data-testid="chips">chips</span>}
      trailing={<span data-testid="sort">sort</span>}
    />
  );
}

describe("FilterBar", () => {
  it("keeps the search open beside the chips when not collapsed", () => {
    render(<Harness collapsed={false} />);
    expect(screen.getByRole("textbox", { name: "Room name or code" })).toBeInTheDocument();
    expect(screen.getByTestId("chips")).toBeInTheDocument();
    expect(screen.getByTestId("sort")).toBeInTheDocument();
  });

  it("folds the search behind a button, swaps it for the chips, and clears it on close", async () => {
    const user = userEvent.setup();
    render(<Harness collapsed />);

    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Search rooms" }));

    const input = screen.getByRole("textbox", { name: "Room name or code" });
    expect(input).toHaveFocus();
    expect(screen.queryByTestId("chips")).not.toBeInTheDocument();
    expect(screen.getByTestId("sort")).toBeInTheDocument();

    await user.type(input, "fri");
    await user.click(screen.getByRole("button", { name: "Close search" }));

    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(screen.getByTestId("chips")).toBeInTheDocument();
  });

  it("starts open when a search is already active", () => {
    render(<Harness collapsed initial="fri" />);
    expect(screen.getByRole("textbox")).toHaveValue("fri");
  });
});
