import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

import { getGoogleClientId, renderGoogleButton } from "@/shared/lib/googleIdentity";

interface GoogleSignInButtonProps {
  /** Receives the raw GIS ID-token credential; verification is server-side. */
  onCredential: (credential: string) => void;
}

/**
 * Renders the Google Identity Services button into a div slot. Renders
 * nothing at all when VITE_GOOGLE_CLIENT_ID is unset, and the slot simply
 * stays empty when the GIS script fails to load (googleIdentity warns once).
 */
export function GoogleSignInButton({ onCredential }: GoogleSignInButtonProps) {
  const { i18n } = useTranslation();
  const slotRef = useRef<HTMLDivElement>(null);
  // The GIS callback closes over this ref so a parent re-render never forces
  // the third-party button to re-mount just to see the latest handler.
  const onCredentialRef = useRef(onCredential);
  useEffect(() => {
    onCredentialRef.current = onCredential;
  });

  const clientId = getGoogleClientId();
  const locale = i18n.language;

  useEffect(() => {
    const parent = slotRef.current;
    if (clientId === "" || parent === null) return;
    // GIS renders a fixed-width iframe, so size it to the slot at render time:
    // the auth card can be as narrow as ~230px on small phones. Clamp to GIS's
    // 200px minimum and our 358px design width; 0 (unmeasurable, e.g. jsdom)
    // falls back to the design width. No resize listener — the initial
    // measure is enough.
    const measured = parent.clientWidth;
    const width = measured === 0 ? 358 : Math.max(200, Math.min(358, measured));
    // GIS's initialize callback is global: cancel on unmount/re-run so a slow
    // script load can't let THIS (stale) page's initialize win over the page
    // the user has already navigated to.
    let cancelled = false;
    void renderGoogleButton({
      clientId,
      parent,
      locale,
      width,
      cancelled: () => cancelled,
      onCredential: (credential) => onCredentialRef.current(credential),
    });
    return () => {
      cancelled = true;
    };
  }, [clientId, locale]);

  if (clientId === "") return null;

  return (
    <div
      ref={slotRef}
      className="flex min-h-11 justify-center"
      data-testid="google-signin-button"
    />
  );
}
