import { act, cleanup, render, screen } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RulesPage } from "@/features/rules/RulesPage";
import { SuitRule } from "@/shared/components/ui/suit-rule";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

describe("suit surfaces outside Scope Amendment 1", () => {
  const realScrollIntoView = Element.prototype.scrollIntoView;

  beforeEach(async () => {
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
    cleanup();
    Element.prototype.scrollIntoView = realScrollIntoView;
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("keeps the decorative suit divider French under the Croatian deck", () => {
    const { container } = render(<SuitRule />);

    const text = container.textContent ?? "";
    for (const glyph of ["♥", "♠", "♦", "♣"]) {
      expect(text).toContain(glyph);
    }
    expect(container.querySelector("[data-testid^='suit-mark-']")).toBeNull();
    expect(container.querySelector("img")).toBeNull();
  });

  it("keeps the rules-page card ladders French under the Croatian deck", () => {
    render(
      <BrowserRouter>
        <RulesPage />
      </BrowserRouter>,
    );

    expect(screen.getByTestId("ladder-plain").textContent).toContain("♥");
    expect(screen.getByTestId("ladder-trump").textContent).toContain("♣");
    for (const testId of ["ladder-plain", "ladder-trump"]) {
      const ladder = screen.getByTestId(testId);
      expect(ladder.querySelector("[data-testid^='suit-mark-']")).toBeNull();
      expect(ladder.querySelector("img")).toBeNull();
    }
  });
});
