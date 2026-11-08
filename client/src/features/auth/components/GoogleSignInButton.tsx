import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

import { useActiveTheme } from "@/shared/hooks/useActiveTheme";
import { getGoogleClientId, renderGoogleButton } from "@/shared/lib/googleIdentity";

const GIS_MIN_BUTTON_WIDTH_PX = 200;
const DESIGN_BUTTON_WIDTH_PX = 358;

/** Props for GoogleSignInButton; onCredential receives the raw GIS ID-token, verified server-side. */
interface GoogleSignInButtonProps {
  onCredential: (credential: string) => void;
}

/** Clamps the measured slot width for the fixed-width GIS iframe, using the design width when unmeasurable. */
function gisButtonWidth(measuredSlotWidth: number): number {
  if (measuredSlotWidth === 0) return DESIGN_BUTTON_WIDTH_PX;
  return Math.max(GIS_MIN_BUTTON_WIDTH_PX, Math.min(DESIGN_BUTTON_WIDTH_PX, measuredSlotWidth));
}

/**
 * Renders the Google Identity Services button (dark in brass mode), or nothing when VITE_GOOGLE_CLIENT_ID is unset.
 * The slot keeps a light colour scheme so the browser paints no opaque box behind Google's light iframe.
 */
export function GoogleSignInButton({ onCredential }: GoogleSignInButtonProps) {
  const { i18n } = useTranslation();
  const slotRef = useRef<HTMLDivElement>(null);
  const onCredentialRef = useRef(onCredential);
  useEffect(() => {
    onCredentialRef.current = onCredential;
  });

  const clientId = getGoogleClientId();
  const locale = i18n.language;
  const gisTheme = useActiveTheme() === "brass" ? "filled_black" : "outline";

  useEffect(() => {
    const parent = slotRef.current;
    if (clientId === "" || parent === null) return;
    const width = gisButtonWidth(parent.clientWidth);
    let cancelled = false;
    void renderGoogleButton({
      clientId,
      parent,
      locale,
      theme: gisTheme,
      width,
      cancelled: () => cancelled,
      onCredential: (credential) => onCredentialRef.current(credential),
    });
    return () => {
      cancelled = true;
    };
  }, [clientId, locale, gisTheme]);

  if (clientId === "") return null;

  return (
    <div
      ref={slotRef}
      className="flex min-h-11 justify-center"
      style={{ colorScheme: "light" }}
      data-testid="google-signin-button"
    />
  );
}
