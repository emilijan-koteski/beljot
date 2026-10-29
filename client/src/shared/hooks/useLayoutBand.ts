import { useMediaQuery } from "@/shared/hooks/useMediaQuery";

export type LayoutBand = "phone" | "tablet" | "desktop";

export const TABLET_MIN_WIDTH = 768;
export const DESKTOP_MIN_WIDTH = 1024;

/**
 * The layout band the window is in: a phone below 768px, a tablet up to 1023px, a desktop from 1024px.
 * The bounds are Tailwind's `md` and `lg`, so a band and the matching CSS variants switch together.
 */
export function useLayoutBand(): LayoutBand {
  const tablet = useMediaQuery(`(min-width: ${TABLET_MIN_WIDTH}px)`);
  const desktop = useMediaQuery(`(min-width: ${DESKTOP_MIN_WIDTH}px)`);
  if (desktop) return "desktop";
  return tablet ? "tablet" : "phone";
}
