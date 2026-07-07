/// <reference types="vite/client" />

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
