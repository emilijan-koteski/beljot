import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import i18n from "i18next";
import { afterAll, describe, expect, it } from "vitest";

import { DeclarationWaiting } from "./DeclarationWaiting";

describe("DeclarationWaiting", () => {
  afterAll(async () => {
    await i18n.changeLanguage("en");
  });

  it("names the seat the table is waiting on", () => {
    render(<DeclarationWaiting activePlayerName="ana" activePlayerTeam="gold" />);
    expect(screen.getByTestId("declaration-waiting")).toHaveTextContent(
      "Waiting for ana to declare or skip",
    );
  });

  it("never swallows clicks — the table underneath stays interactive", () => {
    // It is an informational banner over a live table, not a modal.
    render(<DeclarationWaiting activePlayerName="ana" activePlayerTeam="gold" />);
    expect(screen.getByTestId("declaration-waiting")).toHaveClass("pointer-events-none");
  });

  it("renders without a known name or team", () => {
    // activePlayerSeat can be null between transitions; the banner must not
    // crash or print "undefined" at the table.
    render(<DeclarationWaiting activePlayerName={null} activePlayerTeam={null} />);
    const el = screen.getByTestId("declaration-waiting");
    expect(el).toBeInTheDocument();
    expect(el.textContent).not.toContain("undefined");
  });

  it.each(["en", "mk", "hr", "sr"] as const)("renders real copy in %s", async (loc) => {
    await i18n.changeLanguage(loc);
    render(<DeclarationWaiting activePlayerName="ana" activePlayerTeam="silver" />);
    const text = screen.getByTestId("declaration-waiting").textContent ?? "";
    expect(text).toContain("ana");
    // A missing translation makes i18next echo the key back.
    expect(text).not.toContain("match.declaration.waiting");
    if (loc === "mk") {
      // mk is all-Cyrillic; a Latin word would mean an untranslated leak.
      expect(text.replace("ana", "")).toMatch(/^[^A-Za-z]*$/);
    }
  });
});
