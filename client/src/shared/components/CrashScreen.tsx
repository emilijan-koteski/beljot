import { useTranslation } from "react-i18next";

import { Button, buttonVariants } from "@/shared/components/ui/button";

const LOBBY_PATH = "/lobby";

/** The full-page fallback after a render crash: reload, or leave for the lobby with a fresh page load. */
export function CrashScreen() {
  const { t } = useTranslation();
  return (
    <div
      className="bg-background flex min-h-screen items-center justify-center px-4"
      data-testid="crash-screen"
    >
      <div className="flex flex-col items-center gap-5 text-center">
        <h1 className="text-ink text-2xl font-semibold tracking-tight">
          {t("common.crash.title")}
        </h1>
        <div className="flex flex-wrap justify-center gap-3">
          <Button size="cta" onClick={() => window.location.reload()}>
            {t("common.crash.retry")}
          </Button>
          <a href={LOBBY_PATH} className={buttonVariants({ variant: "outline", size: "cta" })}>
            {t("common.crash.backToLobby")}
          </a>
        </div>
      </div>
    </div>
  );
}
