import { ArrowRight, Zap } from "lucide-react";
import { useTranslation } from "react-i18next";

type Props = {
  onClick: () => void;
  disabled?: boolean;
};

/**
 * Felt-green accent gradient tile — the lobby's single primary CTA.
 * Calls the existing `useQuickPlayMutation` upstream and triggers the
 * pending matchmaking overlay until success.
 */
export function QuickPlayTile({ onClick, disabled }: Props) {
  const { t } = useTranslation();
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      data-testid="quick-play-card"
      className="group/qp relative flex items-center gap-4 overflow-hidden rounded-lg border border-accent/40 bg-[linear-gradient(135deg,var(--accent-soft)_0%,rgba(25,101,54,0.04)_60%,var(--surface)_100%)] px-5.5 py-4.5 text-left text-ink shadow-glow inset-ring inset-ring-accent/10 transition-transform hover:-translate-y-0.5 disabled:opacity-60"
    >
      <span className="bg-accent text-accent-ink inline-flex size-11 items-center justify-center rounded-xl shadow-glow">
        <Zap className="size-5.5" strokeWidth={2} />
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="text-base font-semibold">{t("lobby.actions.quickPlay.title")}</span>
        <span className="text-ink-dim text-xs">{t("lobby.actions.quickPlay.subtitle")}</span>
      </span>
      <span className="text-accent ml-auto inline-flex items-center gap-1.5 text-xs font-semibold">
        {t("lobby.actions.quickPlay.cta")}
        <ArrowRight className="size-4" strokeWidth={2.2} />
      </span>
    </button>
  );
}
