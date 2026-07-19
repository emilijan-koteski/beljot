import "@/shared/i18n/i18n";
import "@/index.css";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/App";
import { ErrorBoundary } from "@/shared/components/ErrorBoundary";

window.addEventListener("vite:preloadError", (event) => {
  const lastReloadMs = Number(sessionStorage.getItem("beljot:preload-error-reload") ?? "0");
  if (Date.now() - lastReloadMs < 60_000) return;
  event.preventDefault();
  sessionStorage.setItem("beljot:preload-error-reload", String(Date.now()));
  window.location.reload();
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>,
);
