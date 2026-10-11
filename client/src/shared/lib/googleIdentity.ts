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
 * Loads the GIS script once, sharing the cached promise. Resolves false (never
 * rejects) on failure and clears the cache so a later navigation can retry.
 */
function loadGoogleIdentity(): Promise<boolean> {
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
        warnLoadFailureOnce();
        gsiLoadPromise = null;
        resolve(false);
        return;
      }
      resolve(true);
    };
    script.onerror = () => {
      warnLoadFailureOnce();
      gsiLoadPromise = null;
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

interface RenderGoogleButtonOptions {
  clientId: string;
  parent: HTMLElement;
  locale?: string;
  width?: number;
  cancelled?: () => boolean;
  onCredential: (credential: string) => void;
}

/**
 * Loads GIS and renders the "Continue with Google" button into `parent`. Returns
 * false if the script failed to load or `cancelled()` (checked before the global initialize) is true.
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
 * Reads the email claim from an ID-token credential for display only; the
 * signature is not checked because the server re-verifies the token.
 */
export function decodeCredentialEmail(credential: string): string | null {
  try {
    const payload = credential.split(".")[1];
    if (payload === undefined || payload === "") return null;
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
