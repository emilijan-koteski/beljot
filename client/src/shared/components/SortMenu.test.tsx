import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Clock3, UsersRound } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { SortMenu, type SortOption } from "./SortMenu";

type Order = "filling" | "newest";

const OPTIONS: SortOption<Order>[] = [
  { value: "filling", label: "Filling up first", icon: UsersRound },
  { value: "newest", label: "Newest first", icon: Clock3 },
];

describe("SortMenu", () => {
  it("shows only the active order's icon until labelled", () => {
    const { unmount } = render(
      <SortMenu
        value="filling"
        options={OPTIONS}
        onValueChange={() => {}}
        ariaLabel="Sort: Filling up first"
      />,
    );
    const trigger = screen.getByTestId("sort-menu");
    expect(trigger).toHaveAccessibleName("Sort: Filling up first");
    expect(trigger).not.toHaveTextContent("Filling up first");
    unmount();

    render(
      <SortMenu
        value="newest"
        options={OPTIONS}
        onValueChange={() => {}}
        ariaLabel="Sort"
        labelled
      />,
    );
    expect(screen.getByTestId("sort-menu")).toHaveTextContent("Newest first");
  });

  it("picks another order from the menu", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(
      <SortMenu value="filling" options={OPTIONS} onValueChange={onValueChange} ariaLabel="Sort" />,
    );

    await user.click(screen.getByTestId("sort-menu"));
    await user.click(await screen.findByTestId("sort-menu-newest"));

    expect(onValueChange).toHaveBeenCalledWith("newest");
  });
});
