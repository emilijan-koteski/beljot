// Thin wrapper around the Google Identity Services (GIS) script: idempotent
// loader, typed surface for `window.google.accounts.id` (no @types package
// exists), and a single render entry point used by GoogleSignInButton.
//
// Authentication is ID-token only — no OAuth scopes, no One Tap prompt. The
// credential handed to onCredential is verified server-side; nothing here is
// trusted for auth decisions.

interface GoogleCredentialResponse {
  credential: string;
}

interface GoogleIdConfiguration {
  client_id: string;
  callback: (response: GoogleCredentialResponse) => void;
}

interface GsiButtonConfiguration {
  type: "standard" | "icon";
  theme?: "outline" | "filled_blue" | "filled_black";
  size?: "large" | "medium" | "small";
  text?: "signin_with" | "signup_with" | "continue_with" | "signin";
  shape?: "rectangular" | "pill" | "circle" | "square";
  locale?: string;
  width?: number;
}

interface GoogleAccountsId {
  initialize: (config: GoogleIdConfiguration) => void;
  renderButton: (parent: HTMLElement, options: GsiButtonConfiguration) => void;
}

declare global {
  interface Window {
    // Every level is optional: a third-party script may define window.google
    // without accounts (or accounts without id), so nothing here may be
    // dereferenced without optional chaining.
    google?: {
      accounts?: {
        id?: GoogleAccountsId;
      };
    };
  }
}

const GSI_SRC = "https://accounts.google.com/gsi/client";

let gsiLoadPromise: Promise<boolean> | null = null;
let warnedLoadFailure = false;

function warnLoadFailureOnce(): void {
  if (warnedLoadFailure) return;
  warnedLoadFailure = true;
  console.warn("Google Identity Services script failed to load — Google sign-in is unavailable");
}

/**
 * Loads the GIS script once (the promise is cached; concurrent callers share
 * it). Resolves false — never rejects — when the script cannot load, so
 * callers can simply leave their button slot empty. A load failure clears the
 * cache so a later navigation can retry.
 */
export function loadGoogleIdentity(): Promise<boolean> {
  if (gsiLoadPromise) return gsiLoadPromise;

  gsiLoadPromise = new Promise<boolean>((resolve) => {
    if (typeof document === "undefined") {
      resolve(false);
      return;
    }
    if (window.google?.accounts?.id !== undefined) {
      resolve(true);
      return;
    }
    const script = document.createElement("script");
    script.src = GSI_SRC;
    script.async = true;
    script.defer = true;
    script.onload = () => {
      if (window.google?.accounts?.id === undefined) {
        // The script loaded but GIS is not there (blocked/mangled by an
        // extension, or window.google clobbered) — warn once and clear the
        // cache so a later navigation can retry instead of failing forever.
        warnLoadFailureOnce();
        gsiLoadPromise = null;
        resolve(false);
        return;
      }
      resolve(true);
    };
    script.onerror = () => {
      warnLoadFailureOnce();
      gsiLoadPromise = null; // allow a later page to retry the load
      resolve(false);
    };
    document.head.appendChild(script);
  });
  return gsiLoadPromise;
}

/** The GIS OAuth client ID, or "" when not configured (button is hidden). */
export function getGoogleClientId(): string {
  return (import.meta.env.VITE_GOOGLE_CLIENT_ID ?? "").trim();
}

export interface RenderGoogleButtonOptions {
  clientId: string;
  parent: HTMLElement;
  /** BCP-47 language tag for the button label (from i18n.language). */
  locale?: string;
  /** Button width in px (GIS caps at 400). */
  width?: number;
  /**
   * Checked after the (async) script load, right before initialize. GIS's
   * initialize callback is a global — a caller that unmounted while the
   * script loaded must return true here so its stale initialize can't steal
   * the callback from the currently mounted button.
   */
  cancelled?: () => boolean;
  onCredential: (credential: string) => void;
}

/**
 * Loads GIS (once), initializes it with the client ID, and renders the
 * standard "Continue with Google" button into `parent`. Returns false when
 * the script could not load; the slot is left empty.
 */
export async function renderGoogleButton(options: RenderGoogleButtonOptions): Promise<boolean> {
  const loaded = await loadGoogleIdentity();
  const gsi = window.google?.accounts?.id;
  if (!loaded || !gsi) return false;
  if (options.cancelled !== undefined && options.cancelled()) return false;

  gsi.initialize({
    client_id: options.clientId,
    callback: (response) => options.onCredential(response.credential),
  });
  // Re-renders (e.g. a language switch) replace the previous button instead of
  // stacking a second one.
  options.parent.replaceChildren();
  gsi.renderButton(options.parent, {
    type: "standard",
    theme: "outline",
    size: "large",
    text: "continue_with",
    shape: "pill",
    locale: options.locale,
    width: options.width,
  });
  return true;
}

/**
 * Reads the email claim out of an ID-token credential for DISPLAY ONLY (the
 * link dialog shows which account matched). Signature is deliberately not
 * checked — the server re-verifies the full token before acting on it.
 */
export function decodeCredentialEmail(credential: string): string | null {
  try {
    const payload = credential.split(".")[1];
    if (payload === undefined || payload === "") return null;
    // atob yields a byte string — decode those bytes as UTF-8 before parsing,
    // or any non-ASCII claim in the payload mojibakes.
    const binary = atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
    const bytes = Uint8Array.from(binary, (ch) => ch.charCodeAt(0));
    const decoded: unknown = JSON.parse(new TextDecoder("utf-8").decode(bytes));
    if (decoded === null || typeof decoded !== "object") return null;
    const email = (decoded as { email?: unknown }).email;
    return typeof email === "string" && email !== "" ? email : null;
  } catch {
    return null;
  }
}
