import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { i18n } from "@/shared/i18n/i18n";
import type { Variant } from "@/shared/types/matchTypes";

import { RulesDialog } from "./RulesDialog";

// `getAllByTestId` already throws when nothing matches, so the guard is here
// only to narrow away `undefined` under `noUncheckedIndexedAccess`.
function firstDiffMarker(): HTMLElement {
  const [marker] = screen.getAllByTestId("rules-diff-marker");
  if (!marker) throw new Error("no difference marker rendered");
  return marker;
}

beforeEach(async () => {
  Element.prototype.scrollIntoView = vi.fn();
  await act(async () => {
    await i18n.changeLanguage("en");
  });
});

afterEach(async () => {
  await act(async () => {
    await i18n.changeLanguage("en");
  });
});

describe("RulesDialog (in-game)", () => {
  it("renders nothing when closed", () => {
    render(<RulesDialog open={false} onOpenChange={() => {}} />);
    expect(screen.queryByTestId("rules-dialog")).not.toBeInTheDocument();
  });

  it("renders the full rules reference with a chapter index", () => {
    render(<RulesDialog open onOpenChange={() => {}} />);

    expect(screen.getByTestId("rules-dialog")).toBeInTheDocument();
    expect(screen.getByText("Belote rules")).toBeInTheDocument();
    expect(screen.getByTestId("rules-toc-goal")).toHaveTextContent("The goal");
    expect(screen.getByTestId("rules-toc-scoring")).toHaveTextContent("Scoring");
    expect(
      screen.getByRole("heading", { name: "Trump plays by its own rules" }),
    ).toBeInTheDocument();
    const carreJ = screen.getByTestId("meld-carreJ");
    expect(within(carreJ).getByText("Carré of Jacks")).toBeInTheDocument();
    expect(within(carreJ).getByText("+200")).toBeInTheDocument();
  });

  it("does not render a language toggle", () => {
    render(<RulesDialog open onOpenChange={() => {}} />);
    const dialog = screen.getByTestId("rules-dialog");
    expect(within(dialog).queryByRole("button", { name: /^mk$/i })).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: /^en$/i })).not.toBeInTheDocument();
  });

  it("closes via the footer button", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<RulesDialog open onOpenChange={onOpenChange} />);
    await user.click(screen.getByTestId("rules-dialog-close"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("jumps to a chapter from the index", async () => {
    const user = userEvent.setup();
    render(<RulesDialog open onOpenChange={() => {}} />);
    await user.click(screen.getByTestId("rules-toc-melds"));
    expect(Element.prototype.scrollIntoView).toHaveBeenCalled();
  });

  const BITOLA_STEP = "Deal five each, then turn one up";
  const CROATIA_STEP = "Deal all eight up front";

  it("pre-selects the Croatian tab in a Croatian match", () => {
    render(<RulesDialog open onOpenChange={() => {}} variant="croatia" />);
    expect(screen.getByTestId("rules-dialog-variant-croatia")).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText(CROATIA_STEP)).toBeInTheDocument();
    expect(screen.queryByText(BITOLA_STEP)).not.toBeInTheDocument();
  });

  it("pre-selects the Bitola tab in a Bitola match", () => {
    render(<RulesDialog open onOpenChange={() => {}} variant="bitola" />);
    expect(screen.getByTestId("rules-dialog-variant-bitola")).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText(BITOLA_STEP)).toBeInTheDocument();
    expect(screen.queryByText(CROATIA_STEP)).not.toBeInTheDocument();
  });

  it("falls back to Bitola for a missing or unknown variant", () => {
    const { unmount } = render(<RulesDialog open onOpenChange={() => {}} />);
    expect(screen.getByText(BITOLA_STEP)).toBeInTheDocument();
    unmount();
    render(<RulesDialog open onOpenChange={() => {}} variant={"pelagonia" as Variant} />);
    expect(screen.getByTestId("rules-dialog-variant-bitola")).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText(BITOLA_STEP)).toBeInTheDocument();
  });

  it("swaps only the scoped blocks when the tab is switched by hand", async () => {
    const user = userEvent.setup();
    render(<RulesDialog open onOpenChange={() => {}} variant="bitola" />);
    const shared = "Trump plays by its own rules";
    expect(screen.getByRole("heading", { name: shared })).toBeInTheDocument();

    await user.click(screen.getByTestId("rules-dialog-variant-croatia"));

    expect(screen.getByText(CROATIA_STEP)).toBeInTheDocument();
    expect(screen.queryByText(BITOLA_STEP)).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: shared })).toBeInTheDocument();
  });

  it("resets the tab to the match variant when reopened", () => {
    const { rerender } = render(<RulesDialog open onOpenChange={() => {}} variant="bitola" />);
    fireEvent.click(screen.getByTestId("rules-dialog-variant-croatia"));
    expect(screen.getByText(CROATIA_STEP)).toBeInTheDocument();

    rerender(<RulesDialog open={false} onOpenChange={() => {}} variant="bitola" />);
    rerender(<RulesDialog open onOpenChange={() => {}} variant="bitola" />);

    expect(screen.getByTestId("rules-dialog-variant-bitola")).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText(BITOLA_STEP)).toBeInTheDocument();
    expect(screen.queryByText(CROATIA_STEP)).not.toBeInTheDocument();
  });

  it("moves between variant tabs with the arrow keys", async () => {
    const user = userEvent.setup();
    render(<RulesDialog open onOpenChange={() => {}} variant="bitola" />);
    const bitola = screen.getByTestId("rules-dialog-variant-bitola");
    const croatia = screen.getByTestId("rules-dialog-variant-croatia");
    expect(bitola).toHaveAttribute("tabindex", "0");
    expect(croatia).toHaveAttribute("tabindex", "-1");

    bitola.focus();
    await user.keyboard("{ArrowRight}");

    expect(croatia).toHaveAttribute("aria-selected", "true");
    expect(croatia).toHaveFocus();
    expect(screen.getByText(CROATIA_STEP)).toBeInTheDocument();

    await user.keyboard("{ArrowLeft}");
    expect(bitola).toHaveAttribute("aria-selected", "true");
    expect(bitola).toHaveFocus();
  });

  it("points the selected variant tab at the panel it controls", async () => {
    const user = userEvent.setup();
    render(<RulesDialog open onOpenChange={() => {}} variant="bitola" />);
    const panelOf = (testId: string) =>
      document.getElementById(screen.getByTestId(testId).getAttribute("aria-controls")!);

    expect(panelOf("rules-dialog-variant-bitola")).toHaveAttribute("role", "tabpanel");
    expect(panelOf("rules-dialog-variant-bitola")).toHaveTextContent(BITOLA_STEP);

    await user.click(screen.getByTestId("rules-dialog-variant-croatia"));

    expect(panelOf("rules-dialog-variant-croatia")).toHaveAttribute("role", "tabpanel");
    expect(panelOf("rules-dialog-variant-croatia")).toHaveTextContent(CROATIA_STEP);
  });

  it("numbers the visible steps gaplessly in the overlay, on both tabs", async () => {
    const user = userEvent.setup();
    render(<RulesDialog open onOpenChange={() => {}} variant="bitola" />);
    const numbers = () =>
      [...screen.getByTestId("rules-dialog").querySelectorAll("ol > li > span:first-child")]
        .map((el) => el.textContent ?? "")
        .filter((txt) => /^\d\d$/.test(txt))
        .slice(0, 6);
    expect(numbers()).toEqual(["01", "02", "03", "04", "05", "06"]);
    await user.click(screen.getByTestId("rules-dialog-variant-croatia"));
    expect(numbers()).toEqual(["01", "02", "03", "04", "05", "06"]);
  });

  it("marks each divergent step individually in the overlay", async () => {
    const user = userEvent.setup();
    render(<RulesDialog open onOpenChange={() => {}} variant="croatia" />);
    expect(
      within(screen.getByText("Take your seat")).queryByTestId("rules-diff-marker"),
    ).not.toBeInTheDocument();
    await user.hover(within(screen.getByText(CROATIA_STEP)).getByTestId("rules-diff-marker"));
    expect(
      await screen.findByText(/Bitola rules stop the deal at five cards each/),
    ).toBeInTheDocument();
  });

  it("carries the difference marker into the dark theme", async () => {
    const user = userEvent.setup();
    render(<RulesDialog open onOpenChange={() => {}} variant="bitola" />);
    const marker = firstDiffMarker();
    await user.hover(marker);
    expect(
      await screen.findByText(/Croatian rules deal all eight cards before anyone bids/),
    ).toBeInTheDocument();
  });

  it("renders Macedonian content when that locale is active", async () => {
    render(<RulesDialog open onOpenChange={() => {}} />);
    await act(async () => {
      await i18n.changeLanguage("mk");
    });
    expect(screen.getByText("Правила на Бељот")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Адутот игра по свои правила" }),
    ).toBeInTheDocument();
  });

  it("caveats the scoring chapter on a stop-at-target table", () => {
    render(<RulesDialog open onOpenChange={() => {}} stopAtTarget />);
    expect(screen.getByTestId("rules-dialog-stop-at-target-note")).toBeInTheDocument();
  });

  it("shows no caveat when the room finishes the hand", () => {
    render(<RulesDialog open onOpenChange={() => {}} />);
    expect(screen.queryByTestId("rules-dialog-stop-at-target-note")).not.toBeInTheDocument();
  });

  it("notes that a fallen team's declarations are voided when they do not transfer", () => {
    render(<RulesDialog open onOpenChange={() => {}} declarationsTransfer={false} />);
    expect(screen.getByTestId("rules-dialog-no-declarations-transfer-note")).toHaveTextContent(
      /voided instead of going to the other team/i,
    );
  });

  it("shows no transfer note when declarations transfer or the flag is absent", () => {
    const { unmount } = render(<RulesDialog open onOpenChange={() => {}} declarationsTransfer />);
    expect(
      screen.queryByTestId("rules-dialog-no-declarations-transfer-note"),
    ).not.toBeInTheDocument();
    unmount();

    render(<RulesDialog open onOpenChange={() => {}} />);
    expect(
      screen.queryByTestId("rules-dialog-no-declarations-transfer-note"),
    ).not.toBeInTheDocument();
  });

  it("notes that play moves to the left on a clockwise table", () => {
    render(<RulesDialog open onOpenChange={() => {}} clockwise />);
    expect(screen.getByTestId("rules-dialog-clockwise-note")).toHaveTextContent(
      /play moves to the left:/i,
    );
  });

  it("shows no direction note on a counter-clockwise table or when the flag is absent", () => {
    const { unmount } = render(<RulesDialog open onOpenChange={() => {}} clockwise={false} />);
    expect(screen.queryByTestId("rules-dialog-clockwise-note")).not.toBeInTheDocument();
    unmount();

    render(<RulesDialog open onOpenChange={() => {}} />);
    expect(screen.queryByTestId("rules-dialog-clockwise-note")).not.toBeInTheDocument();
  });

  it("shows both table notes when the table stops at the target and voids declarations", () => {
    render(<RulesDialog open onOpenChange={() => {}} stopAtTarget declarationsTransfer={false} />);
    expect(screen.getByTestId("rules-dialog-stop-at-target-note")).toBeInTheDocument();
    expect(screen.getByTestId("rules-dialog-no-declarations-transfer-note")).toBeInTheDocument();
  });
});
