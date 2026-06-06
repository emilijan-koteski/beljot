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
      screen.getByRole("heading", { level: 1, name: "Learn Beljot in one sitting" }),
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

  it("renders Croatian and Serbian hero titles", async () => {
    renderRules();
    await act(async () => {
      await i18n.changeLanguage("hr");
    });
    expect(
      screen.getByRole("heading", { level: 1, name: "Nauči Beljot u jednom sjedenju" }),
    ).toBeInTheDocument();
    await act(async () => {
      await i18n.changeLanguage("sr");
    });
    expect(
      screen.getByRole("heading", { level: 1, name: "Nauči Beljot u jednom sedenju" }),
    ).toBeInTheDocument();
  });
});
