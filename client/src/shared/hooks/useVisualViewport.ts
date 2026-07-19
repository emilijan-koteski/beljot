import { useEffect, useState } from "react";

export interface VisualViewportBox {
  /** Height of the visible viewport in CSS px — shrinks when the keyboard opens. */
  height: number;
  /** How far the visual viewport is panned down from the layout viewport's top. */
  offsetTop: number;
}

/**
 * Live size/offset of `window.visualViewport` while `enabled`. The visual
 * viewport is the only viewport the mobile on-screen keyboard shrinks — the
 * layout viewport (and with it `fixed inset-0` / `100dvh`) does NOT resize on
 * iOS Safari, so the browser pans the page to reveal the focused input and
 * drags fixed headers off-screen.
 *
 * Consumers pin full-screen overlays to the visible area instead:
 * `style={{ top: box.offsetTop, height: box.height }}`.
 *
 * Returns null when disabled or when the API is unavailable (jsdom, old
 * browsers) — callers must fall back to pure-CSS sizing. Android Chrome is
 * handled without JS via `interactive-widget=resizes-content` in the viewport
 * meta tag (see index.html); this hook is the iOS path, but tracking both
 * platforms is harmless since it simply mirrors the resized viewport.
 */
export function useVisualViewport(enabled: boolean): VisualViewportBox | null {
  const [box, setBox] = useState<VisualViewportBox | null>(null);

  useEffect(() => {
    const vv = typeof window === "undefined" ? undefined : window.visualViewport;
    if (!enabled || !vv) {
      setBox(null);
      return;
    }
    const update = () => setBox({ height: vv.height, offsetTop: vv.offsetTop });
    update();
    // resize fires when the keyboard opens/closes; scroll fires as the visual
    // viewport pans within the layout viewport (iOS keyboard reveal).
    vv.addEventListener("resize", update);
    vv.addEventListener("scroll", update);
    return () => {
      vv.removeEventListener("resize", update);
      vv.removeEventListener("scroll", update);
    };
  }, [enabled]);

  return box;
}
