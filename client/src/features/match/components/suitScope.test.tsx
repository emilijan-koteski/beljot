import { act, render, screen } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RulesPage } from "@/features/rules/RulesPage";
import { SuitRule } from "@/shared/components/ui/suit-rule";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

/**
 * The guard on Scope Amendment 1's **excluded** surfaces.
 *
 * The owner's authorisation was bounded — *"symbols used on the UI and dialogs
 * as trump indicators"* — so exactly four components follow the active deck.
 * These two do not, and nothing else in the suite would notice if they started:
 *
 *  • `shared/components/ui/suit-rule.tsx` is a decorative all-four-suits
 *    divider that also renders on the unauthenticated auth pages, where there is
 *    no deck preference to read at all.
 *  • `features/rules/components/CardLadder.tsx` is the rules reference, owned by
 *    Story 12.9. Re-suiting it here would pre-empt that story's own decision
 *    about how the reference presents a two-variant game.
 *
 * Both assertions are made with the **Croatian deck active**, which is the only
 * state in which a regression is observable.
 */
describe("suit surfaces outside Scope Amendment 1", () => {
  // Saved and restored: assigning onto Element.prototype without putting the
  // original back leaks the stub into every later test in the run.
  const realScrollIntoView = Element.prototype.scrollIntoView;

  beforeEach(async () => {
    // jsdom has no scrollIntoView — the rules TOC jump calls it.
    Element.prototype.scrollIntoView = vi.fn();
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ cardDeckPreference: "croatian" }),
      isLoading: false,
    });
    await act(async () => {
      await i18n.changeLanguage("en");
    });
  });

  afterEach(() => {
    Element.prototype.scrollIntoView = realScrollIntoView;
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("keeps the decorative suit divider French under the Croatian deck", () => {
    const { container } = render(<SuitRule />);

    const text = container.textContent ?? "";
    for (const glyph of ["♥", "♠", "♦", "♣"]) {
      expect(text).toContain(glyph);
    }
    // No SuitMark anywhere — the divider must not have been routed through the
    // per-deck seam.
    expect(container.querySelector("[data-testid^='suit-mark-']")).toBeNull();
    expect(container.querySelector("img")).toBeNull();
  });

  it("keeps the rules-page card ladders French under the Croatian deck", () => {
    render(
      <BrowserRouter>
        <RulesPage />
      </BrowserRouter>,
    );

    // Off-trump ladder is the hearts one, trump is clubs (see RuleBlock).
    expect(screen.getByTestId("ladder-plain").textContent).toContain("♥");
    expect(screen.getByTestId("ladder-trump").textContent).toContain("♣");
    for (const testId of ["ladder-plain", "ladder-trump"]) {
      const ladder = screen.getByTestId(testId);
      expect(ladder.querySelector("[data-testid^='suit-mark-']")).toBeNull();
      expect(ladder.querySelector("img")).toBeNull();
    }
  });
});
