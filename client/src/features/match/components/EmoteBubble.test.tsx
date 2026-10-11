import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { EMOTE_IDS } from "@/shared/types/wsEvents";

import { EmoteBubble } from "./EmoteBubble";

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

describe("EmoteBubble", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockMatchMedia(false);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders the bubble with the test id keyed on compass position", () => {
    render(<EmoteBubble emote="thumbs_up" compassPosition={2} onDismiss={vi.fn()} />);
    expect(screen.getByTestId("emote-bubble-2")).toBeInTheDocument();
  });

  it("renders a glyph for every emote id", () => {
    for (const id of EMOTE_IDS) {
      const { unmount } = render(
        <EmoteBubble emote={id} compassPosition={0} onDismiss={vi.fn()} />,
      );
      expect(screen.getByTestId("emote-bubble-0")).toBeInTheDocument();
      expect(screen.getByTestId("emote-bubble-0").textContent ?? "").not.toBe("");
      unmount();
    }
  });

  it("calls onDismiss after 2000 ms (default motion)", () => {
    const onDismiss = vi.fn();
    render(<EmoteBubble emote="clap" compassPosition={1} onDismiss={onDismiss} />);

    expect(onDismiss).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1999);
    });
    expect(onDismiss).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("calls onDismiss after 1000 ms when prefers-reduced-motion is set", () => {
    mockMatchMedia(true);
    const onDismiss = vi.fn();
    render(<EmoteBubble emote="laugh" compassPosition={3} onDismiss={onDismiss} />);

    act(() => {
      vi.advanceTimersByTime(999);
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
      <EmoteBubble emote="heart" compassPosition={0} onDismiss={onDismiss} />,
    );

    unmount();
    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(onDismiss).not.toHaveBeenCalled();
  });

  it("does not reset the dismiss timer when the parent re-renders with a new onDismiss identity", () => {
    const onDismiss = vi.fn();
    const { rerender } = render(
      <EmoteBubble emote="thumbs_up" compassPosition={0} onDismiss={() => onDismiss()} />,
    );

    act(() => {
      vi.advanceTimersByTime(1500);
    });
    rerender(<EmoteBubble emote="thumbs_up" compassPosition={0} onDismiss={() => onDismiss()} />);
    act(() => {
      vi.advanceTimersByTime(499);
    });
    expect(onDismiss).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("is non-interactive (pointer-events-none and aria-live=polite)", () => {
    render(<EmoteBubble emote="facepalm" compassPosition={0} onDismiss={vi.fn()} />);
    const bubble = screen.getByTestId("emote-bubble-0");
    expect(bubble).toHaveAttribute("aria-live", "polite");
    expect(bubble.className).toContain("pointer-events-none");
  });
});
