import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";

import { useLevelUpStore } from "@/shared/stores/levelUpStore";

import { LevelUpGate } from "./LevelUpGate";

describe("LevelUpGate", () => {
  beforeEach(() => useLevelUpStore.getState().clear());

  it("renders nothing when no level-up is pending", () => {
    render(<LevelUpGate />);
    expect(screen.queryByTestId("level-up-dialog")).not.toBeInTheDocument();
  });

  it("shows the dialog when a level-up is pending and clears it on Continue", async () => {
    const user = userEvent.setup();
    useLevelUpStore.getState().setPending({ newLevel: 5, newTotalXp: 1300, xpEarned: 90 });

    render(<LevelUpGate />);
    expect(screen.getByTestId("level-up-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("level-up-level")).toHaveTextContent("5");

    await user.click(screen.getByTestId("level-up-continue"));

    await waitFor(() => {
      expect(screen.queryByTestId("level-up-dialog")).not.toBeInTheDocument();
    });
    expect(useLevelUpStore.getState().pending).toBeNull();
  });
});
