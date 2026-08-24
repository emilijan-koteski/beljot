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
    // Chapter index entries for all six chapters.
    expect(screen.getByTestId("rules-toc-goal")).toHaveTextContent("The goal");
    expect(screen.getByTestId("rules-toc-scoring")).toHaveTextContent("Scoring");
    // Content: a chapter heading, a card value, and a declaration.
    expect(
      screen.getByRole("heading", { name: "Trump plays by its own rules" }),
    ).toBeInTheDocument();
    const carreJ = screen.getByTestId("rules-meld-carreJ");
    expect(within(carreJ).getByText("Carré of Jacks")).toBeInTheDocument();
    expect(within(carreJ).getByText("+200")).toBeInTheDocument();
  });

  it("does not render a language toggle", () => {
    render(<RulesDialog open onOpenChange={() => {}} />);
    const dialog = screen.getByTestId("rules-dialog");
    // The design's per-dialog en/mk switch is intentionally gone — language is
    // set in game settings.
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

  // ── The variant split ────────────────────────────────────────────────────
  //
  // The overlay opens over a live table, so the tab it lands on has to be the
  // variant actually being played — rules that contradict the cards in front of
  // the player are worse than no rules at all.

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
    // The component stays MOUNTED while closed (`if (!open) return null` sits
    // after the hooks), so `activeVariant` survives a close. Without the reset
    // effect a player who browsed the other ruleset mid-hand would come back to
    // it, still showing rules this table is not playing.
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
    // Roving tabIndex: only the selected tab is in the tab order.
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

  it("points both variant tabs at the panel they control", () => {
    render(<RulesDialog open onOpenChange={() => {}} variant="bitola" />);
    const panelId = screen.getByTestId("rules-dialog-variant-bitola").getAttribute("aria-controls");
    expect(panelId).toBeTruthy();
    expect(screen.getByTestId("rules-dialog-variant-croatia")).toHaveAttribute(
      "aria-controls",
      panelId,
    );
    expect(document.getElementById(panelId!)).toHaveAttribute("role", "tabpanel");
  });

  it("numbers the visible steps gaplessly in the overlay, on both tabs", async () => {
    const user = userEvent.setup();
    render(<RulesDialog open onOpenChange={() => {}} variant="bitola" />);
    const numbers = () =>
      [...screen.getByTestId("rules-dialog").querySelectorAll("ol > li > span:first-child")]
        .map((el) => el.textContent ?? "")
        .filter((txt) => /^\d\d$/.test(txt))
        .slice(0, 6);
    // The dark switch filters and numbers independently of the light one, so it
    // needs its own proof: numbering the unfiltered array reads 01,02,03,05,07,09.
    expect(numbers()).toEqual(["01", "02", "03", "04", "05", "06"]);
    await user.click(screen.getByTestId("rules-dialog-variant-croatia"));
    expect(numbers()).toEqual(["01", "02", "03", "04", "05", "06"]);
  });

  it("marks each divergent step individually in the overlay", async () => {
    const user = userEvent.setup();
    render(<RulesDialog open onOpenChange={() => {}} variant="croatia" />);
    // Shared steps carry no marker; the divergent one carries its own.
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

  // The scoring chapter says the match runs until a team is on the target "at the
  // end of a hand", which is the opposite of what a "dosta" table does. The
  // content module has no room-level split (Story 12.9 owns that), so the caveat
  // rides on the dialog — and must NOT appear on an ordinary table.
  it("caveats the scoring chapter on a stop-at-target table", () => {
    render(<RulesDialog open onOpenChange={() => {}} stopAtTarget />);
    expect(screen.getByTestId("rules-dialog-stop-at-target-note")).toBeInTheDocument();
  });

  it("shows no caveat when the room finishes the hand", () => {
    render(<RulesDialog open onOpenChange={() => {}} />);
    expect(screen.queryByTestId("rules-dialog-stop-at-target-note")).not.toBeInTheDocument();
  });
});
