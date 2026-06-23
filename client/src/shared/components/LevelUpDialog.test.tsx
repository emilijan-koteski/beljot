import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { LevelUpDialog } from "./LevelUpDialog";

// Controlled harness mirroring how the gate drives the dialog: `open` is state,
// and onClose flips it to false (the gate clears the pending level-up).
function Harness({ initialOpen }: { initialOpen: boolean }) {
  const [open, setOpen] = useState(initialOpen);
  return (
    <LevelUpDialog
      open={open}
      level={3}
      newTotalXp={520}
      xpEarned={120}
      onClose={() => setOpen(false)}
    />
  );
}

describe("LevelUpDialog", () => {
  it("renders the reached level and the XP earned", () => {
    render(<Harness initialOpen={true} />);

    expect(screen.getByTestId("level-up-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("level-up-level")).toHaveTextContent("3");
    expect(screen.getByTestId("level-up-earned")).toHaveTextContent("120");
    expect(screen.getByTestId("level-up-xp-bar")).toBeInTheDocument();
  });

  it("closes when the player clicks Continue", async () => {
    const user = userEvent.setup();
    render(<Harness initialOpen={true} />);

    await user.click(screen.getByTestId("level-up-continue"));

    await waitFor(() => {
      expect(screen.queryByTestId("level-up-dialog")).not.toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(<Harness initialOpen={false} />);

    expect(screen.queryByTestId("level-up-dialog")).not.toBeInTheDocument();
  });
});
