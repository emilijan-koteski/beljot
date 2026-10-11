import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useMatchStore } from "@/shared/stores/matchStore";
import { EMOTE_IDS } from "@/shared/types/wsEvents";

import { EmotePickerButton } from "./EmotePickerButton";

describe("EmotePickerButton", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.advanceTimersByTime(100_000);
    useMatchStore.getState().reset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders the toggle button hidden initially", () => {
    render(<EmotePickerButton onSend={vi.fn()} />);
    expect(screen.getByTestId("emote-toggle")).toBeInTheDocument();
    expect(screen.queryByTestId("emote-picker")).not.toBeInTheDocument();
  });

  it("opens the picker grid on toggle click", () => {
    render(<EmotePickerButton onSend={vi.fn()} />);

    fireEvent.click(screen.getByTestId("emote-toggle"));

    expect(screen.getByTestId("emote-picker")).toBeInTheDocument();
    for (const id of EMOTE_IDS) {
      expect(screen.getByTestId(`emote-tile-${id}`)).toBeInTheDocument();
    }
  });

  it("renders tiles in the same order as EMOTE_IDS", () => {
    render(<EmotePickerButton onSend={vi.fn()} />);
    fireEvent.click(screen.getByTestId("emote-toggle"));

    const tiles = screen
      .getByTestId("emote-picker")
      .querySelectorAll("[data-testid^='emote-tile-']");
    const renderedOrder = Array.from(tiles).map((el) =>
      (el.getAttribute("data-testid") ?? "").replace("emote-tile-", ""),
    );
    expect(renderedOrder).toEqual([...EMOTE_IDS]);
  });

  it("calls onSend with the clicked emote id and closes the picker", () => {
    const onSend = vi.fn();
    render(<EmotePickerButton onSend={onSend} />);

    fireEvent.click(screen.getByTestId("emote-toggle"));
    fireEvent.click(screen.getByTestId("emote-tile-clap"));

    expect(onSend).toHaveBeenCalledTimes(1);
    expect(onSend).toHaveBeenCalledWith("clap");
    expect(screen.queryByTestId("emote-picker")).not.toBeInTheDocument();
  });

  it("does not fire onSend a second time within the 3 s cooldown", () => {
    const onSend = vi.fn();
    render(<EmotePickerButton onSend={onSend} />);

    fireEvent.click(screen.getByTestId("emote-toggle"));
    fireEvent.click(screen.getByTestId("emote-tile-thumbs_up"));
    expect(onSend).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByTestId("emote-toggle"));
    const laughTile = screen.getByTestId("emote-tile-laugh");
    expect(laughTile).toBeDisabled();
    fireEvent.click(laughTile);

    expect(onSend).toHaveBeenCalledTimes(1);
  });

  it("re-enables tiles after the 3 s cooldown elapses", () => {
    const onSend = vi.fn();
    render(<EmotePickerButton onSend={onSend} />);

    fireEvent.click(screen.getByTestId("emote-toggle"));
    fireEvent.click(screen.getByTestId("emote-tile-thumbs_up"));
    expect(onSend).toHaveBeenCalledTimes(1);

    act(() => {
      vi.advanceTimersByTime(3001);
    });

    fireEvent.click(screen.getByTestId("emote-toggle"));
    const laughTile = screen.getByTestId("emote-tile-laugh");
    expect(laughTile).not.toBeDisabled();
    fireEvent.click(laughTile);

    expect(onSend).toHaveBeenCalledTimes(2);
    expect(onSend).toHaveBeenLastCalledWith("laugh");
  });

  it("closes on Escape key", () => {
    render(<EmotePickerButton onSend={vi.fn()} />);
    fireEvent.click(screen.getByTestId("emote-toggle"));
    expect(screen.getByTestId("emote-picker")).toBeInTheDocument();

    fireEvent.keyDown(document, { key: "Escape" });

    expect(screen.queryByTestId("emote-picker")).not.toBeInTheDocument();
  });

  it("closes on outside mouse click", () => {
    render(
      <div>
        <div data-testid="outside-zone" />
        <EmotePickerButton onSend={vi.fn()} />
      </div>,
    );

    fireEvent.click(screen.getByTestId("emote-toggle"));
    expect(screen.getByTestId("emote-picker")).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByTestId("outside-zone"));

    expect(screen.queryByTestId("emote-picker")).not.toBeInTheDocument();
  });

  it("respects the disabled prop on the toggle", () => {
    render(<EmotePickerButton onSend={vi.fn()} disabled />);
    expect(screen.getByTestId("emote-toggle")).toBeDisabled();
  });

  it("focuses the first tile when the picker opens for keyboard users", () => {
    render(<EmotePickerButton onSend={vi.fn()} />);
    fireEvent.click(screen.getByTestId("emote-toggle"));

    expect(document.activeElement).toBe(screen.getByTestId(`emote-tile-${EMOTE_IDS[0]}`));
  });

  it("moves focus across the tile grid with arrow keys (AC #1)", () => {
    render(<EmotePickerButton onSend={vi.fn()} />);
    fireEvent.click(screen.getByTestId("emote-toggle"));

    const tiles = EMOTE_IDS.map((id) => screen.getByTestId(`emote-tile-${id}`));

    expect(document.activeElement).toBe(tiles[0]);

    fireEvent.keyDown(tiles[0]!, { key: "ArrowRight" });
    expect(document.activeElement).toBe(tiles[1]);

    fireEvent.keyDown(tiles[1]!, { key: "ArrowDown" });
    expect(document.activeElement).toBe(tiles[4]);

    fireEvent.keyDown(tiles[4]!, { key: "ArrowLeft" });
    expect(document.activeElement).toBe(tiles[3]);

    fireEvent.keyDown(tiles[3]!, { key: "ArrowUp" });
    expect(document.activeElement).toBe(tiles[0]);
  });

  it("uses role=group on the popover, not role=dialog (non-modal)", () => {
    render(<EmotePickerButton onSend={vi.fn()} />);
    fireEvent.click(screen.getByTestId("emote-toggle"));

    const picker = screen.getByTestId("emote-picker");
    expect(picker).toHaveAttribute("role", "group");
    expect(picker).not.toHaveAttribute("role", "dialog");
  });

  it("preserves cooldown across remount (AC8)", () => {
    const onSend = vi.fn();
    const { unmount } = render(<EmotePickerButton onSend={onSend} />);

    fireEvent.click(screen.getByTestId("emote-toggle"));
    fireEvent.click(screen.getByTestId("emote-tile-thumbs_up"));
    expect(onSend).toHaveBeenCalledTimes(1);

    unmount();

    render(<EmotePickerButton onSend={onSend} />);
    fireEvent.click(screen.getByTestId("emote-toggle"));
    const laughTile = screen.getByTestId("emote-tile-laugh");
    expect(laughTile).toBeDisabled();
    fireEvent.click(laughTile);

    expect(onSend).toHaveBeenCalledTimes(1);
  });
});
