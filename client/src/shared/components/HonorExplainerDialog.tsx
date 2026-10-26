import { ArrowRight, Clock, Flag, Info, LogOut, ShieldCheck } from "lucide-react";
import { Trans, useTranslation } from "react-i18next";
import { Link } from "react-router";

import { HonorBandMeter } from "@/shared/components/HonorBandMeter";
import { HonorShield } from "@/shared/components/HonorShield";
import { Dialog, DialogClose, DialogContent } from "@/shared/components/ui/dialog";
import {
  HONOR_TIER_COLOR,
  honorIsNewPlayer,
  honorScoreOrPrior,
  normalizeHonorTier,
} from "@/shared/lib/honor";
import { useAuthStore } from "@/shared/stores/authStore";

type HonorExplainerDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/**
 * Controlled "How honour works" dialog explaining what moves the score and showing the player's standing.
 */
export function HonorExplainerDialog({ open, onOpenChange }: HonorExplainerDialogProps) {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);

  const isNew = honorIsNewPlayer(user?.isNewPlayer);
  const score = honorScoreOrPrior(user?.honorScore);
  const tier = normalizeHonorTier(user?.honorTier ?? "", score);

  const facts = [
    { icon: ShieldCheck, key: "finish" },
    { icon: LogOut, key: "drop" },
    { icon: Flag, key: "surrender" },
    { icon: Clock, key: "decay" },
  ] as const;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        data-testid="honor-explainer"
        style={{
          display: "block",
          width: 480,
          maxWidth: "calc(100% - 48px)",
          padding: 0,
          overflow: "hidden",
          background: "var(--surface)",
          border: "1px solid var(--brass)",
          borderRadius: "var(--radius-lg)",
          color: "var(--ink)",
          boxShadow: "0 40px 90px -30px rgba(14,58,36,0.55), 0 0 0 1px var(--brass-soft)",
        }}
      >
        <div
          style={{
            height: 3,
            width: "100%",
            opacity: 0.7,
            background: "linear-gradient(90deg, transparent, var(--brass), transparent)",
          }}
        />

        <div style={{ display: "flex", gap: 16, padding: "26px 28px 6px" }}>
          <div
            style={{
              flexShrink: 0,
              width: 52,
              height: 52,
              borderRadius: 14,
              background: "var(--brass-soft)",
              border: "1px solid color-mix(in srgb, var(--brass) 45%, transparent)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              boxShadow: "inset 0 0 0 4px var(--brass-10)",
            }}
          >
            <ShieldCheck size={24} color="var(--brass-deep)" />
          </div>
          <div style={{ flex: 1, minWidth: 0, paddingTop: 2 }}>
            <span
              style={{
                fontFamily: "var(--font-mono)",
                fontSize: 10,
                letterSpacing: "0.18em",
                textTransform: "uppercase",
                fontWeight: 600,
                color: "var(--brass-deep)",
              }}
            >
              {t("profile.honor.eyebrow")}
            </span>
            <h2
              data-testid="honor-explainer-title"
              style={{
                margin: "6px 0 0",
                fontWeight: 700,
                fontSize: 24,
                letterSpacing: "-0.025em",
                color: "var(--ink)",
                lineHeight: 1.2,
              }}
            >
              {t("profile.honor.explainer.title")}
            </h2>
          </div>
        </div>

        <div style={{ padding: "12px 28px 0" }}>
          <p className="text-ink-dim m-0 text-sm leading-relaxed">
            {t("profile.honor.explainer.lede")}
          </p>

          <ul className="mt-4 mb-0 flex list-none flex-col gap-2.5 p-0">
            {facts.map(({ icon: Icon, key }) => (
              <li key={key} className="flex gap-2.5" data-testid={`honor-explainer-${key}`}>
                <Icon
                  className="text-brass-deep mt-0.5 size-4 shrink-0"
                  aria-hidden="true"
                  strokeWidth={2.25}
                />
                <span className="text-ink-dim text-sm leading-normal">
                  <strong className="text-ink font-semibold">
                    {t(`profile.honor.explainer.${key}Label`)}
                  </strong>{" "}
                  {t(`profile.honor.explainer.${key}Body`)}
                </span>
              </li>
            ))}
          </ul>

          <div className="mt-5">
            <HonorBandMeter
              score={isNew ? undefined : score}
              showTierLabels
              testId="honor-explainer-meter"
            />
          </div>

          <div
            role="note"
            data-testid="honor-explainer-standing"
            data-honor={isNew ? undefined : score}
            className="mt-4 flex items-start gap-2.5 px-3.5 py-3"
            style={{
              borderRadius: 10,
              background: "color-mix(in srgb, var(--brass-soft) 60%, transparent)",
              border: "1px solid color-mix(in srgb, var(--brass) 35%, transparent)",
            }}
          >
            {isNew ? (
              <Info
                className="text-brass-deep mt-0.5 size-4 shrink-0"
                aria-hidden="true"
                strokeWidth={2.25}
              />
            ) : (
              <HonorShield tier={tier} size={16} className="mt-0.5 shrink-0" />
            )}
            <p className="text-ink m-0 text-sm leading-normal font-semibold">
              {isNew ? (
                t("profile.honor.explainer.standingNew")
              ) : (
                <Trans
                  i18nKey="profile.honor.explainer.standing"
                  values={{ score, tier: t(`profile.honor.tier.${tier}`) }}
                  components={{
                    tierWord: <strong style={{ color: HONOR_TIER_COLOR[tier] }} />,
                  }}
                />
              )}
            </p>
          </div>
        </div>

        <div
          style={{
            marginTop: 18,
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 12,
            padding: "16px 28px",
            borderTop: "1px solid var(--border)",
            background: "color-mix(in srgb, var(--surface-3) 45%, transparent)",
          }}
        >
          <Link
            to="/rules#honour"
            onClick={() => onOpenChange(false)}
            className="text-accent hover:text-accent-deep inline-flex items-center gap-1.5 text-sm font-semibold"
            data-testid="honor-explainer-rules-link"
          >
            {t("profile.honor.explainer.fullRules")}
            <ArrowRight className="size-3.5" aria-hidden="true" />
          </Link>
          <DialogClose
            data-testid="honor-explainer-close"
            style={{
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              gap: 8,
              padding: "11px 18px",
              borderRadius: 10,
              fontSize: 14,
              fontWeight: 600,
              lineHeight: 1.2,
              cursor: "pointer",
              background: "linear-gradient(180deg, var(--brass) 0%, var(--brass-deep) 100%)",
              color: "var(--brass-ink)",
              border: "1px solid var(--brass-deep)",
              boxShadow:
                "0 6px 16px -8px rgba(156,125,78,0.65), inset 0 1px 0 rgba(255,255,255,0.25)",
            }}
          >
            {t("profile.honor.explainer.close")}
          </DialogClose>
        </div>
      </DialogContent>
    </Dialog>
  );
}
