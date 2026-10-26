import { useTranslation } from "react-i18next";
import { Link } from "react-router";

export function PrivacyPage() {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-6">
      <div className="w-full max-w-150 rounded-xl bg-surface p-8">
        <h1 className="mb-3 font-display text-2xl font-bold text-ink" data-testid="privacy-title">
          {t("legal.privacy.title")}
        </h1>
        <p
          className="mb-2 text-sm uppercase tracking-wide text-accent"
          data-testid="privacy-wip-badge"
        >
          {t("legal.wipBadge")}
        </p>
        <p className="mb-3 text-ink-dim" data-testid="privacy-wip-notice">
          {t("legal.privacy.wipNotice")}
        </p>
        <p className="mb-6 text-ink-dim">{t("legal.privacy.comingSoon")}</p>

        <div className="flex justify-end">
          <Link
            to="/"
            className="text-sm text-accent underline-offset-2 hover:underline"
            data-testid="privacy-back-link"
          >
            {t("legal.back")}
          </Link>
        </div>
      </div>
    </div>
  );
}
