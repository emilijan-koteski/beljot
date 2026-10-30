import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { resetCardDeckRequestSequence } from "@/shared/lib/cardDeckPreference";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import { CardDeckRow } from "./CardDeckRow";

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

describe("CardDeckRow", () => {
  beforeEach(() => {
    mockUpdatePreferences.mockReset();
    mockUpdatePreferences.mockResolvedValue({ cardDeckPreference: "croatian" });
    resetCardDeckRequestSequence();
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ id: 7, cardDeckPreference: "french" }),
      isLoading: false,
    });
  });

  it("marks the stored deck as the selected radio", () => {
    render(<CardDeckRow />);

    expect(screen.getByTestId("profile-deck-option-french")).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.getByTestId("profile-deck-option-croatian")).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  it("previews each deck with a suit that actually differs between them", () => {
    render(<CardDeckRow />);

    const previews = Array.from(document.querySelectorAll("img")).map((i) => i.getAttribute("src"));
    expect(previews).toContain("/cards/french/AD.svg");
    expect(previews).toContain("/cards/croatian/AD.webp");
  });

  it("writes the deck optimistically and PATCHes only the deck field", async () => {
    const user = userEvent.setup();
    render(<CardDeckRow />);

    await user.click(screen.getByTestId("profile-deck-option-croatian"));

    expect(useAuthStore.getState().user?.cardDeckPreference).toBe("croatian");
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { cardDeckPreference: "croatian" });
    expect(mockUpdatePreferences.mock.calls[0]?.[1]).not.toHaveProperty("languagePreference");
  });

  it("reverts the store deck when the PATCH fails", async () => {
    const user = userEvent.setup();
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
    render(<CardDeckRow />);

    await user.click(screen.getByTestId("profile-deck-option-croatian"));

    await waitFor(() => {
      expect(useAuthStore.getState().user?.cardDeckPreference).toBe("french");
    });
    expect(screen.getByTestId("profile-deck-option-french")).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("does not PATCH when the already-selected deck is clicked again", async () => {
    const user = userEvent.setup();
    render(<CardDeckRow />);

    await user.click(screen.getByTestId("profile-deck-option-french"));

    expect(mockUpdatePreferences).not.toHaveBeenCalled();
  });

  it("reverts to the RESOLVED deck, never a stored value it cannot render", async () => {
    const user = userEvent.setup();
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ id: 7, cardDeckPreference: "hungarian" as never }),
      isLoading: false,
    });
    render(<CardDeckRow />);

    await user.click(screen.getByTestId("profile-deck-option-croatian"));

    await waitFor(() => {
      expect(useAuthStore.getState().user?.cardDeckPreference).toBe("french");
    });
  });
});
