import { useEffect, useState } from "react";

interface VisualViewportBox {
  height: number;
  offsetTop: number;
}

/**
 * Live size/offset of `window.visualViewport` while `enabled`, so overlays can track the area
 * left by the iOS on-screen keyboard. Returns null when disabled or unavailable.
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
    vv.addEventListener("resize", update);
    vv.addEventListener("scroll", update);
    return () => {
      vv.removeEventListener("resize", update);
      vv.removeEventListener("scroll", update);
    };
  }, [enabled]);

  return box;
}
