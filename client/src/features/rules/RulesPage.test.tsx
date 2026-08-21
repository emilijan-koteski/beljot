import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { i18n } from "@/shared/i18n/i18n";

import { RulesPage } from "./RulesPage";

function renderRules() {
  return render(
    <BrowserRouter>
      <RulesPage />
    </BrowserRouter>,
  );
}

beforeEach(async () => {
  // jsdom has no scrollIntoView — the TOC jump calls it.
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

describe("RulesPage", () => {
  it("renders the hero and all six chapter headings", () => {
    renderRules();
    expect(
      screen.getByRole("heading", { level: 1, name: "Learn Belote in one sitting" }),
    ).toBeInTheDocument();
    for (const title of [
      "Race your team to 1001",
      "Shuffle, deal, take trump",
      "Trump plays by its own rules",
      "When you can play what",
      "Some hands carry points of their own",
      "Counting up — and the catch",
    ]) {
      expect(screen.getByRole("heading", { level: 2, name: title })).toBeInTheDocument();
    }
  });

  it("renders the chapter index with section labels", () => {
    renderRules();
    const toc = screen.getByTestId("rules-chapter-index");
    expect(within(toc).getByTestId("toc-goal")).toHaveTextContent("The goal");
    expect(within(toc).getByTestId("toc-scoring")).toHaveTextContent("Scoring");
  });

  it("renders both card ladders with trump Jack worth 20", () => {
    renderRules();
    const trump = screen.getByTestId("ladder-trump");
    // Jack row: rank chip + name + 20 points.
    expect(within(trump).getByText("Jack")).toBeInTheDocument();
    expect(within(trump).getByText("20")).toBeInTheDocument();
    expect(screen.getByTestId("ladder-plain")).toBeInTheDocument();
  });

  it("renders the declarations grid including Carré of Jacks at 200", () => {
    renderRules();
    const carreJ = screen.getByTestId("meld-carreJ");
    expect(within(carreJ).getByText("Carré of Jacks")).toBeInTheDocument();
    expect(within(carreJ).getByText("+200")).toBeInTheDocument();
  });

  it("scrolls to a chapter when its index entry is clicked", async () => {
    const user = userEvent.setup();
    renderRules();
    await user.click(screen.getByTestId("toc-melds"));
    expect(Element.prototype.scrollIntoView).toHaveBeenCalled();
  });

  // ── The variant split ────────────────────────────────────────────────────
  //
  // The four shared chapters must not move between tabs; `basics` and `melds`
  // must. These assertions are the only thing standing between that promise and
  // a page that quietly describes the wrong ruleset.

  const BITOLA_STEP = "Deal five each, then turn one up";
  const CROATIA_STEP = "Deal all eight up front";
  const BITOLA_RULE = "One card, one declaration";
  const CROATIA_RULE = "One card can count more than once";
  const SHARED_HEADING = "Trump plays by its own rules";
  const SHARED_RULE = "Out of the suit? You must trump — over the top if you can";

  it("opens on the Bitola tab and renders only its scoped blocks", () => {
    renderRules();
    expect(screen.getByTestId("rules-variant-bitola")).toHaveAttribute("aria-selected", "true");
    expect(screen.getByTestId("rules-variant-croatia")).toHaveAttribute("aria-selected", "false");
    expect(screen.getByText(BITOLA_STEP)).toBeInTheDocument();
    expect(screen.getByText(BITOLA_RULE)).toBeInTheDocument();
    expect(screen.queryByText(CROATIA_STEP)).not.toBeInTheDocument();
    expect(screen.queryByText(CROATIA_RULE)).not.toBeInTheDocument();
  });

  it("swaps the scoped blocks on the Croatian tab and leaves shared chapters alone", async () => {
    const user = userEvent.setup();
    renderRules();
    expect(screen.getByRole("heading", { level: 2, name: SHARED_HEADING })).toBeInTheDocument();
    expect(screen.getByText(SHARED_RULE)).toBeInTheDocument();

    await user.click(screen.getByTestId("rules-variant-croatia"));

    expect(screen.getByTestId("rules-variant-croatia")).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText(CROATIA_STEP)).toBeInTheDocument();
    expect(screen.getByText(CROATIA_RULE)).toBeInTheDocument();
    expect(screen.queryByText(BITOLA_STEP)).not.toBeInTheDocument();
    expect(screen.queryByText(BITOLA_RULE)).not.toBeInTheDocument();
    // Shared chapters are untouched — that is the whole point of block scoping.
    expect(screen.getByRole("heading", { level: 2, name: SHARED_HEADING })).toBeInTheDocument();
    expect(screen.getByText(SHARED_RULE)).toBeInTheDocument();
  });

  it("describes the Croatian deal, reveal and forced pick under that tab", async () => {
    const user = userEvent.setup();
    renderRules();
    await user.click(screen.getByTestId("rules-variant-croatia"));
    expect(screen.getByText("Round two: your last two turn up")).toBeInTheDocument();
    expect(screen.getByText("The dealer cannot pass")).toBeInTheDocument();
    expect(
      screen.getByText(/Declaring gets a phase of its own, between bidding and the first trick/),
    ).toBeInTheDocument();
  });

  it("states the shared Capot and tied-tiebreaker rules under both tabs", async () => {
    const user = userEvent.setup();
    renderRules();
    for (const tab of ["rules-variant-bitola", "rules-variant-croatia"]) {
      await user.click(screen.getByTestId(tab));
      expect(screen.getByText("Take all eight tricks and it’s a capot")).toBeInTheDocument();
      expect(
        screen.getByText(
          /if the two totals are exactly equal, it goes to the team that took trump/,
        ),
      ).toBeInTheDocument();
    }
  });

  it("names the all-eight hand after the trump suit, not any suit", () => {
    renderRules();
    const bela = screen.getByTestId("meld-bela");
    expect(within(bela).getByText("Belote")).toBeInTheDocument();
    expect(within(bela).getByText(/All eight cards of the trump suit/)).toBeInTheDocument();
  });

  /** The visible 01/02/… chips of the `basics` step list, in document order. */
  function basicsStepNumbers(): string[] {
    const basics = document.getElementById("basics");
    if (!basics) throw new Error("basics chapter is not rendered");
    return [...basics.querySelectorAll("ol > li > span.font-mono")].map(
      (el) => el.textContent ?? "",
    );
  }

  /** The step whose title is `title`, as the element that also holds its marker. */
  function step(title: string) {
    return screen.getByText(title);
  }

  it("numbers the visible steps gaplessly on both tabs", async () => {
    const user = userEvent.setup();
    renderRules();
    // Items are scoped INSIDE one shared block, so numbering the unfiltered
    // array would render 01, 02, 03, 05, 07, 09 here. Both tabs must read 01-06.
    expect(basicsStepNumbers()).toEqual(["01", "02", "03", "04", "05", "06"]);
    await user.click(screen.getByTestId("rules-variant-croatia"));
    expect(basicsStepNumbers()).toEqual(["01", "02", "03", "04", "05", "06"]);
  });

  it("authors the shared steps once, with no marker on them", async () => {
    const user = userEvent.setup();
    renderRules();
    for (const shared of ["Take your seat", "Build the deck"]) {
      // One node, not two: the shared steps are no longer duplicated per variant.
      expect(screen.getByText(shared)).toBeInTheDocument();
      expect(within(step(shared)).queryByTestId("rules-diff-marker")).not.toBeInTheDocument();
    }
    await user.click(screen.getByTestId("rules-variant-croatia"));
    for (const shared of ["Take your seat", "Build the deck"]) {
      expect(screen.getByText(shared)).toBeInTheDocument();
      expect(within(step(shared)).queryByTestId("rules-diff-marker")).not.toBeInTheDocument();
    }
  });

  it("marks each divergent step individually, explaining that step's counterpart", async () => {
    const user = userEvent.setup();
    renderRules();

    // Bitola tab: the deal step's own marker describes the Croatian deal.
    const bitolaDeal = within(step(BITOLA_STEP)).getByTestId("rules-diff-marker");
    await user.hover(bitolaDeal);
    expect(
      await screen.findByText(/Croatian rules deal all eight cards before anyone bids/),
    ).toBeInTheDocument();
    await user.unhover(bitolaDeal);

    // And the round-one step carries a DIFFERENT note — one marker per diff
    // spot, not one for the whole sequence.
    const bitolaRound1 = within(step("Round one: take that card, or pass")).getByTestId(
      "rules-diff-marker",
    );
    await user.hover(bitolaRound1);
    expect(
      await screen.findByText(/In Croatian rules there is no card to take/),
    ).toBeInTheDocument();
    await user.unhover(bitolaRound1);

    // Croatian tab: the mirror image.
    await user.click(screen.getByTestId("rules-variant-croatia"));
    const croatiaDeal = within(step(CROATIA_STEP)).getByTestId("rules-diff-marker");
    await user.hover(croatiaDeal);
    expect(
      await screen.findByText(/Bitola rules stop the deal at five cards each/),
    ).toBeInTheDocument();
  });

  it("opens a difference marker on hover", async () => {
    const user = userEvent.setup();
    renderRules();
    const marker = screen.getAllByTestId("rules-diff-marker")[0];
    await user.hover(marker);
    expect(
      await screen.findByText(/Croatian rules deal all eight cards before anyone bids/),
    ).toBeInTheDocument();
  });

  it("opens and dismisses a difference marker on tap", async () => {
    const user = userEvent.setup();
    renderRules();
    const marker = screen.getAllByTestId("rules-diff-marker")[0];
    // A real touch, not a mouse click: Base UI's hover is `mouseOnly`, so this
    // is the one path that proves the marker is reachable on a phone. Hover-only
    // would pass every other test here and still be dead on a handset.
    await user.pointer({ keys: "[TouchA]", target: marker });
    expect(
      await screen.findByText(/Croatian rules deal all eight cards before anyone bids/),
    ).toBeInTheDocument();
    await user.pointer({ keys: "[TouchA]", target: marker });
    expect(
      screen.queryByText(/Croatian rules deal all eight cards before anyone bids/),
    ).not.toBeInTheDocument();
  });

  it("renders fully translated content when the language switches to Macedonian", async () => {
    renderRules();
    await act(async () => {
      await i18n.changeLanguage("mk");
    });
    expect(
      screen.getByRole("heading", { level: 1, name: "Научи Бељот во неколку минути" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 2, name: "Трка до 1001 поен со твојот партнер" }),
    ).toBeInTheDocument();
  });

  it("keeps the active variant tab across a locale switch", async () => {
    const user = userEvent.setup();
    renderRules();
    await user.click(screen.getByTestId("rules-variant-croatia"));
    await act(async () => {
      await i18n.changeLanguage("hr");
    });
    expect(screen.getByTestId("rules-variant-croatia")).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("Podijeli svih osam odmah")).toBeInTheDocument();
  });

  it("renders Croatian and Serbian hero titles", async () => {
    renderRules();
    await act(async () => {
      await i18n.changeLanguage("hr");
    });
    expect(
      screen.getByRole("heading", { level: 1, name: "Nauči Belu u jednom sjedenju" }),
    ).toBeInTheDocument();
    await act(async () => {
      await i18n.changeLanguage("sr");
    });
    expect(
      screen.getByRole("heading", { level: 1, name: "Nauči Belu u jednom sedenju" }),
    ).toBeInTheDocument();
  });
});
