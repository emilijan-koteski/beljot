import "@/shared/i18n/i18n";

import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DeclareBanner } from "./DeclareBanner";

function mockMatchMedia(reducedMotion: boolean) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: reducedMotion && query.includes("prefers-reduced-motion"),
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
}

describe("DeclareBanner", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockMatchMedia(false);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders with the declarer username and aria-live status", () => {
    render(<DeclareBanner declarerUsername="alice" compassPosition={1} onDismiss={vi.fn()} />);
    const banner = screen.getByTestId("declare-banner");
    expect(banner).toBeInTheDocument();
    expect(banner).toHaveAttribute("role", "status");
    expect(banner).toHaveAttribute("aria-live", "polite");
    expect(banner).toHaveTextContent(/alice/);
  });

  it("does NOT leak meld details — only the fact a declaration exists", () => {
    render(<DeclareBanner declarerUsername="bob" compassPosition={2} onDismiss={vi.fn()} />);
    const text = screen.getByTestId("declare-banner").textContent ?? "";
    // No point values or meld names in the announcement.
    expect(text).not.toMatch(/\d/);
    expect(text.toLowerCase()).not.toMatch(/tierce|quarte|quint|carr/);
  });

  it("is NOT a dialog (the table keeps playing)", () => {
    render(<DeclareBanner declarerUsername="alice" compassPosition={1} onDismiss={vi.fn()} />);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("anchors to the declarer's compass seat — east (1) / west (3)", () => {
    const { unmount } = render(
      <DeclareBanner declarerUsername="alice" compassPosition={1} onDismiss={vi.fn()} />,
    );
    expect(screen.getByTestId("declare-banner").className).toContain("right-[22rem]");
    unmount();

    render(<DeclareBanner declarerUsername="alice" compassPosition={3} onDismiss={vi.fn()} />);
    expect(screen.getByTestId("declare-banner").className).toContain("left-[22rem]");
  });

  it("calls onDismiss after 4000 ms (default motion)", () => {
    const onDismiss = vi.fn();
    render(<DeclareBanner declarerUsername="alice" compassPosition={0} onDismiss={onDismiss} />);

    act(() => {
      vi.advanceTimersByTime(3999);
    });
    expect(onDismiss).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("calls onDismiss after 1500 ms when prefers-reduced-motion is set", () => {
    mockMatchMedia(true);
    const onDismiss = vi.fn();
    render(<DeclareBanner declarerUsername="alice" compassPosition={2} onDismiss={onDismiss} />);

    act(() => {
      vi.advanceTimersByTime(1499);
    });
    expect(onDismiss).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("clears its timer on unmount before firing onDismiss", () => {
    const onDismiss = vi.fn();
    const { unmount } = render(
      <DeclareBanner declarerUsername="alice" compassPosition={0} onDismiss={onDismiss} />,
    );

    unmount();
    act(() => {
      vi.advanceTimersByTime(10000);
    });

    expect(onDismiss).not.toHaveBeenCalled();
  });

  it("does not reset the dismiss timer when the parent re-renders with a new onDismiss identity", () => {
    // Same regression guard as EmoteBubble: MatchPage passes an inline arrow;
    // a timer keyed on callback identity would restart every parent re-render.
    const onDismiss = vi.fn();
    const { rerender } = render(
      <DeclareBanner declarerUsername="alice" compassPosition={0} onDismiss={() => onDismiss()} />,
    );

    act(() => {
      vi.advanceTimersByTime(3000);
    });
    rerender(
      <DeclareBanner declarerUsername="alice" compassPosition={0} onDismiss={() => onDismiss()} />,
    );
    act(() => {
      vi.advanceTimersByTime(999);
    });
    expect(onDismiss).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("is non-interactive (pointer-events-none)", () => {
    render(<DeclareBanner declarerUsername="alice" compassPosition={0} onDismiss={vi.fn()} />);
    expect(screen.getByTestId("declare-banner").className).toContain("pointer-events-none");
  });
});
