import { Clock, Flag, Info, LogOut, ShieldCheck } from "lucide-react";
import { Trans, useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { HonorBandMeter } from "@/shared/components/HonorBandMeter";
import { HonorShield } from "@/shared/components/HonorShield";
import { AppDialog } from "@/shared/components/ui/app-dialog";
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

const FACTS = [
  { icon: ShieldCheck, key: "finish" },
  { icon: LogOut, key: "drop" },
  { icon: Flag, key: "surrender" },
  { icon: Clock, key: "decay" },
] as const;

/**
 * Controlled "How honor works" dialog explaining what moves the score and showing the player's standing.
 */
export function HonorExplainerDialog({ open, onOpenChange }: HonorExplainerDialogProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);

  const isNew = honorIsNewPlayer(user?.isNewPlayer);
  const score = honorScoreOrPrior(user?.honorScore);
  const tier = normalizeHonorTier(user?.honorTier ?? "", score);
  const close = () => onOpenChange(false);

  return (
    <AppDialog
      open={open}
      onClose={close}
      icon={<ShieldCheck />}
      title={<span data-testid="honor-explainer-title">{t("profile.honor.explainer.title")}</span>}
      description={t("profile.honor.explainer.lede")}
      primary={{
        label: t("profile.honor.explainer.close"),
        onClick: close,
        testId: "honor-explainer-close",
      }}
      secondary={{
        label: t("profile.honor.explainer.fullRules"),
        onClick: () => {
          close();
          navigate("/rules#honour");
        },
        testId: "honor-explainer-rules-link",
      }}
      testId="honor-explainer"
    >
      <ul className="m-0 flex list-none flex-col gap-2.5 p-0">
        {FACTS.map(({ icon: Icon, key }) => (
          <li key={key} className="flex gap-2.5" data-testid={`honor-explainer-${key}`}>
            <Icon
              className="text-brass-deep mt-0.5 size-4 shrink-0"
              aria-hidden="true"
              strokeWidth={2.25}
            />
            <span className="text-ink-dim text-sm leading-normal">
              {t(`profile.honor.explainer.facts.${key}`)}
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
        className="bg-brass-soft/60 border-brass/35 mt-4 flex items-start gap-2.5 rounded-lg border px-3.5 py-3"
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
    </AppDialog>
  );
}
