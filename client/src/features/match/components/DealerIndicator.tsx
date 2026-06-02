import { useTranslation } from "react-i18next";

interface DealerIndicatorProps {
  dealerName: string;
}

export function DealerIndicator({ dealerName }: DealerIndicatorProps) {
  const { t } = useTranslation();

  const trimmed = dealerName.trim();
  if (!trimmed) {
    return null;
  }

  const ariaLabel = t("match.dealerIndicator.label", { name: trimmed });

  return (
    <div
      className="border-text-secondary/30 bg-background/80 flex min-w-0 items-center gap-2 rounded-full border-2 px-3 py-1"
      aria-live="polite"
      aria-label={ariaLabel}
      data-testid="dealer-indicator"
    >
      <span className="text-text-secondary font-body text-sm">
        {t("match.dealerIndicator.dealer")}
      </span>
      <span className="text-text-secondary/40" aria-hidden>
        ·
      </span>
      <span
        className="text-text-primary font-display text-sm font-semibold max-w-32 truncate"
        data-testid="dealer-name"
      >
        {trimmed}
      </span>
    </div>
  );
}
