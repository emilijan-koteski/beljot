import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { type ComponentProps, useState } from "react";
import { afterEach, describe, expect, it } from "vitest";

import { i18n } from "@/shared/i18n/i18n";

import { RankChangeDialog } from "./RankChangeDialog";

type Props = ComponentProps<typeof RankChangeDialog>;

const promotion: Omit<Props, "open" | "onClose"> = {
  direction: "promoted",
  tier: "gold",
  division: 2,
  spChange: 40,
  sp: 690,
  progress: { spIntoStep: 23, spForNextStep: 67 },
};

const demotion: Omit<Props, "open" | "onClose"> = {
  direction: "demoted",
  tier: "silver",
  division: 3,
  spChange: -20,
  sp: 590,
  progress: { spIntoStep: 90, spForNextStep: 100 },
};

// Controlled harness mirroring how the gate drives the dialog: `open` is state,
// and onClose flips it to false (the gate clears the pending change).
function Harness(props: Omit<Props, "open" | "onClose"> & { initialOpen?: boolean }) {
  const { initialOpen = true, ...rest } = props;
  const [open, setOpen] = useState(initialOpen);
  return <RankChangeDialog {...rest} open={open} onClose={() => setOpen(false)} />;
}

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("RankChangeDialog", () => {
  it("celebrates a promotion with the rank, the SP change and the step bar", () => {
    render(<Harness {...promotion} />);

    const dialog = screen.getByTestId("rank-change-dialog");
    expect(dialog).toHaveAttribute("data-direction", "promoted");
    expect(dialog).toHaveTextContent("Rank Up!");
    expect(screen.getByTestId("rank-change-rank")).toHaveTextContent("Gold 2");
    expect(screen.getByTestId("rank-change-sp")).toHaveTextContent("+40 SP this match");
    expect(screen.getByTestId("rank-change-progress")).toHaveAttribute("aria-valuenow", "34");
    // The level-up celebration: the gold halo and a lit medallion.
    expect(screen.getByTestId("rank-change-halo")).toBeInTheDocument();
    expect(screen.getByTestId("rank-change-badge")).toHaveAttribute("data-glow", "true");
  });

  // Story 13.5: a demotion never shows the rank-up celebration.
  it("announces a demotion in the same layout without the celebration", () => {
    render(<Harness {...demotion} />);

    const dialog = screen.getByTestId("rank-change-dialog");
    expect(dialog).toHaveAttribute("data-direction", "demoted");
    expect(dialog).toHaveTextContent("Rank Down");
    expect(dialog).not.toHaveTextContent("Rank Up!");
    expect(screen.getByTestId("rank-change-rank")).toHaveTextContent("Silver 3");
    expect(screen.getByTestId("rank-change-sp")).toHaveTextContent("−20 SP this match");
    expect(screen.queryByTestId("rank-change-halo")).not.toBeInTheDocument();
    expect(screen.getByTestId("rank-change-badge")).toHaveAttribute("data-glow", "false");
  });

  it("names a single tier without a division and fills the bar toward Grandmaster", () => {
    render(
      <Harness
        {...promotion}
        tier="master"
        division={null}
        sp={1210}
        progress={{ spIntoStep: 10, spForNextStep: 200 }}
      />,
    );

    expect(screen.getByTestId("rank-change-rank")).toHaveTextContent(/^Master$/);
    expect(screen.getByTestId("rank-change-progress")).toHaveAttribute("aria-valuenow", "5");
  });

  it("shows a full bar and the top-of-ladder label at Grandmaster", () => {
    render(
      <Harness
        {...promotion}
        tier="grandmaster"
        division={null}
        sp={1450}
        progress={{ spIntoStep: 50, spForNextStep: 0 }}
      />,
    );

    const bar = screen.getByTestId("rank-change-progress");
    expect(bar).toHaveAttribute("aria-valuenow", "100");
    expect(bar).toHaveAttribute("aria-label", "Grandmaster, 1,450 SP, top of the ladder");
    expect(screen.getByTestId("rank-change-dialog")).toHaveTextContent("Top of the ladder");
  });

  it("reserves the bar's row, silently, when no step progress is known yet", () => {
    render(<Harness {...promotion} progress={null} />);

    expect(screen.getByTestId("rank-change-rank")).toHaveTextContent("Gold 2");
    expect(screen.queryByTestId("rank-change-progress")).not.toBeInTheDocument();
    // Same box as the bar, so the centred card does not jump when it lands.
    expect(screen.getByTestId("rank-change-progress-placeholder")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
  });

  it("names the new rank in the dialog's accessible title", () => {
    render(<Harness {...demotion} />);

    expect(screen.getByRole("dialog", { name: "Rank Down Silver 3" })).toBeInTheDocument();
  });

  it("closes when the player clicks Continue", async () => {
    const user = userEvent.setup();
    render(<Harness {...promotion} />);

    await user.click(screen.getByTestId("rank-change-continue"));

    await waitFor(() => {
      expect(screen.queryByTestId("rank-change-dialog")).not.toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(<Harness {...promotion} initialOpen={false} />);

    expect(screen.queryByTestId("rank-change-dialog")).not.toBeInTheDocument();
  });

  it("localizes the demotion copy, with СП in Macedonian", async () => {
    await i18n.changeLanguage("mk");
    render(<Harness {...demotion} />);

    const dialog = screen.getByTestId("rank-change-dialog");
    expect(dialog).toHaveTextContent("Пад на рангот");
    expect(screen.getByTestId("rank-change-rank")).toHaveTextContent("Сребро 3");
    expect(screen.getByTestId("rank-change-sp")).toHaveTextContent("−20 СП во овој меч");
    expect(screen.getByTestId("rank-change-continue")).toHaveTextContent("Продолжи");
  });
});
