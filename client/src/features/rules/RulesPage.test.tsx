import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { i18n } from "@/shared/i18n/i18n";
import { mockViewportWidth } from "@/test-utils";

import { RulesPage } from "./RulesPage";

const CHAPTER_TITLES = [
  "Deal and trump",
  "Card values",
  "Playing a trick",
  "Declarations",
  "Scoring",
  "House rules",
];

let restoreViewport = () => {};

function renderRules() {
  return render(
    <BrowserRouter>
      <RulesPage />
    </BrowserRouter>,
  );
}

function stepNumbers(): string[] {
  const timeline = screen.getByTestId("rules-timeline");
  return within(timeline)
    .getAllByTestId("rules-step-number")
    .map((el) => el.textContent ?? "");
}

beforeEach(async () => {
  Element.prototype.scrollIntoView = vi.fn();
  await act(async () => {
    await i18n.changeLanguage("en");
  });
});

afterEach(async () => {
  restoreViewport();
  restoreViewport = () => {};
  await act(async () => {
    await i18n.changeLanguage("en");
  });
});

describe("RulesPage", () => {
  it("renders the hero, the four facts and the six chapters", () => {
    renderRules();
    expect(
      screen.getByRole("heading", { level: 1, name: "How to play Belote" }),
    ).toBeInTheDocument();
    const facts = screen.getByTestId("rules-facts");
    expect(
      within(facts)
        .getAllByRole("definition")
        .map((d) => d.textContent),
    ).toEqual(["4", "32", "8", "1001"]);
    expect(
      within(facts)
        .getAllByRole("term")
        .map((d) => d.textContent),
    ).toEqual(["players", "cards", "in hand", "to win"]);
    expect(screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent)).toEqual(
      CHAPTER_TITLES,
    );
  });

  it("leaves honor out, since it is about the app rather than the card game", () => {
    renderRules();
    expect(screen.queryByText(/honor/i)).not.toBeInTheDocument();
  });

  it("pins chapter chips on a phone and jumps to the chapter a chip names", async () => {
    const user = userEvent.setup();
    renderRules();
    expect(screen.queryByTestId("rules-chapter-index")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("rules-chip-melds"));

    expect(Element.prototype.scrollIntoView).toHaveBeenCalled();
    expect(screen.getByTestId("rules-chip-melds")).toHaveAttribute("aria-current", "true");
  });

  it("lists the chapters and the variant switch in a side column on a desktop", async () => {
    restoreViewport = mockViewportWidth(1280);
    const user = userEvent.setup();
    renderRules();
    expect(screen.queryByTestId("rules-chapter-chips")).not.toBeInTheDocument();
    const nav = screen.getByTestId("rules-chapter-index");
    expect(
      within(nav)
        .getAllByRole("button")
        .map((b) => b.textContent),
    ).toEqual(["Deal", "Cards", "Play", "Declarations", "Scoring", "House rules"]);

    await user.click(within(nav).getByTestId("toc-scoring"));

    expect(Element.prototype.scrollIntoView).toHaveBeenCalled();
    expect(within(nav).getByTestId("toc-scoring")).toHaveAttribute("aria-current", "true");
    expect(screen.getByTestId("rules-variant")).toBeInTheDocument();
  });

  it("lists trump and the other suits side by side, strongest first", () => {
    renderRules();
    const trump = screen.getByTestId("ladder-trump");
    const plain = screen.getByTestId("ladder-plain");
    expect(within(trump).getAllByRole("listitem")[0]).toHaveTextContent(/Jack.*20/);
    expect(within(plain).getAllByRole("listitem")[0]).toHaveTextContent(/Ace.*11/);
    expect(within(plain).getAllByRole("listitem")[4]).toHaveTextContent(/Jack.*2/);
  });

  it("lists every declaration with its points, and the all-trumps hand as a win", () => {
    renderRules();
    expect(screen.getByTestId("meld-carreJ")).toHaveTextContent(/\+200.*Four Jacks/);
    expect(screen.getByTestId("meld-kvarta")).toHaveTextContent(/\+50.*Quarte.*Four in a row/);
    expect(screen.getByTestId("meld-bela")).toHaveTextContent(/Win.*Belote.*All eight trumps/);
  });

  it("keeps the declaration tie-break behind a help marker", async () => {
    const user = userEvent.setup();
    renderRules();
    await user.hover(screen.getByTestId("rules-help"));
    expect(await screen.findByText(/four of a kind beats a run/)).toBeInTheDocument();
  });

  it("opens on Bitola and numbers its deal steps from one", () => {
    renderRules();
    expect(screen.getByText("Five cards each")).toBeInTheDocument();
    expect(screen.queryByText("Eight cards each")).not.toBeInTheDocument();
    expect(screen.getByText("One card, one declaration")).toBeInTheDocument();
    expect(stepNumbers()).toEqual(["1", "2", "3", "4"]);
  });

  it("swaps the variant-scoped steps and rules on the Croatian tab", async () => {
    const user = userEvent.setup();
    renderRules();

    await user.click(screen.getByTestId("rules-variant-croatia"));

    expect(screen.getByText("Eight cards each")).toBeInTheDocument();
    expect(screen.getByText("The dealer must choose")).toBeInTheDocument();
    expect(screen.queryByText("Five cards each")).not.toBeInTheDocument();
    expect(screen.getByText("A card can count twice")).toBeInTheDocument();
    expect(screen.getByText("Belote always counts")).toBeInTheDocument();
    expect(stepNumbers()).toEqual(["1", "2", "3", "4"]);
  });

  it("says what the other variant does behind each difference marker", async () => {
    const user = userEvent.setup();
    renderRules();
    const step = screen.getByText("Five cards each");
    expect(
      within(screen.getByText("Belote always counts")).queryByTestId("rules-diff-marker"),
    ).toBeNull();

    await user.hover(within(step).getByTestId("rules-diff-marker"));

    expect(
      await screen.findByText("Croatian: all eight cards are dealt, the last two face down."),
    ).toBeInTheDocument();
  });

  it("opens and dismisses a difference marker on tap", async () => {
    const user = userEvent.setup();
    renderRules();
    const marker = within(screen.getByText("Five cards each")).getByTestId("rules-diff-marker");
    const note = "Croatian: all eight cards are dealt, the last two face down.";

    await user.pointer({ keys: "[TouchA]", target: marker });
    expect(await screen.findByText(note)).toBeInTheDocument();

    await user.pointer({ keys: "[TouchA]", target: marker });
    expect(screen.queryByText(note)).not.toBeInTheDocument();
  });

  it("words the house rules as the create-room form does, standard value first", () => {
    renderRules();
    expect(screen.getByTestId("rules-option-declarationsTransfer")).toHaveTextContent(
      /Declarations on a fall.*To the winners · Voided/,
    );
    expect(screen.getByTestId("rules-option-matchEnd")).toHaveTextContent(
      /Match end.*Finish the hand · Stop at target/,
    );
    expect(screen.getByTestId("rules-option-direction")).toHaveTextContent(
      /To the right · To the left/,
    );
    expect(screen.getByTestId("rules-option-declarationLastTrick")).toHaveTextContent(
      /Declare until trick.*3 · 1 · 2/,
    );
  });

  it("renders translated chapters in Macedonian, Croatian and Serbian", async () => {
    renderRules();
    await act(async () => {
      await i18n.changeLanguage("mk");
    });
    expect(
      screen.getByRole("heading", { level: 1, name: "Како се игра Бељот" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Најави" })).toBeInTheDocument();
    await act(async () => {
      await i18n.changeLanguage("hr");
    });
    expect(
      screen.getByRole("heading", { level: 2, name: "Dijeljenje i adut" }),
    ).toBeInTheDocument();
    await act(async () => {
      await i18n.changeLanguage("sr");
    });
    expect(screen.getByRole("heading", { level: 2, name: "Deljenje i adut" })).toBeInTheDocument();
  });

  it("keeps the chosen variant across a language switch", async () => {
    const user = userEvent.setup();
    renderRules();
    await user.click(screen.getByTestId("rules-variant-croatia"));

    await act(async () => {
      await i18n.changeLanguage("hr");
    });

    expect(screen.getByTestId("rules-variant-croatia")).toHaveAttribute("aria-checked", "true");
    expect(screen.getByText("Po osam karata")).toBeInTheDocument();
  });
});
