import "@/shared/i18n/i18n";

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { DailyRewardDialog } from "./DailyRewardDialog";

// Controlled harness mirroring how the gate drives the dialog: `open` is state,
// and onClose flips it to false (the gate clears the reward).
function Harness({ initialOpen }: { initialOpen: boolean }) {
  const [open, setOpen] = useState(initialOpen);
  return (
    <DailyRewardDialog open={open} amount={1000} streakDay={1} onClose={() => setOpen(false)} />
  );
}

describe("DailyRewardDialog", () => {
  it("opens and renders the granted reward amount", () => {
    render(<Harness initialOpen={true} />);

    expect(screen.getByTestId("daily-reward-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("daily-reward-amount")).toHaveTextContent((1000).toLocaleString());
  });

  it("stays open without an auto-dismiss timer", async () => {
    render(<Harness initialOpen={true} />);

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(screen.getByTestId("daily-reward-dialog")).toBeInTheDocument();
  });

  it("closes when the player clicks Continue", async () => {
    const user = userEvent.setup();
    render(<Harness initialOpen={true} />);

    await user.click(screen.getByTestId("daily-reward-continue"));

    await waitFor(() => {
      expect(screen.queryByTestId("daily-reward-dialog")).not.toBeInTheDocument();
    });
  });

  it("closes on Escape like every other dialog", async () => {
    render(<Harness initialOpen={true} />);

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });

    await waitFor(() => {
      expect(screen.queryByTestId("daily-reward-dialog")).not.toBeInTheDocument();
    });
  });

  it("is titled by the reward and says nothing the coin and wallet already say", () => {
    render(<Harness initialOpen={true} />);

    expect(screen.getByRole("dialog", { name: "Daily reward" })).toBeInTheDocument();
    expect(screen.queryByText(/coins added/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/new balance/i)).not.toBeInTheDocument();
    expect(screen.getAllByRole("button")).toHaveLength(1);
  });

  it("is titled Streak maxed from day 14", () => {
    render(<DailyRewardDialog open amount={500} streakDay={14} onClose={() => {}} />);

    expect(screen.getByRole("dialog", { name: "Streak maxed" })).toBeInTheDocument();
  });

  it("does not render when no reward was granted", () => {
    render(<Harness initialOpen={false} />);

    expect(screen.queryByTestId("daily-reward-dialog")).not.toBeInTheDocument();
  });
});
