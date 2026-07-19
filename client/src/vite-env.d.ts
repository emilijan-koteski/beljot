/// <reference types="vite/client" />

/**
 * Build identity injected by Vite `define` (vite.config.ts): the commit SHA in
 * CI images, "dev" locally. NOT defined under vitest (separate config), so read
 * it through shared/lib/appVersion.ts, which guards with `typeof`.
 */
declare const __APP_VERSION__: string;

interface ImportMetaEnv {
  /**
   * Google OAuth client ID for the GIS "Continue with Google" button.
   * Optional — when unset/empty the button is hidden entirely.
   */
  readonly VITE_GOOGLE_CLIENT_ID?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
