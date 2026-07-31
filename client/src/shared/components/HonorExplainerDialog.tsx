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
 * "How honour works" — the explainer the shipped product never had.
 *
 * Nothing in the honour system currently states what it measures, and two of the
 * four facts below are things players actively get wrong: that SURRENDERING is
 * not abandonment (it counts as a finished match, so it lifts the score), and
 * that old abandonments decay. Both misconceptions push people toward the
 * behaviour the system exists to discourage — a player who thinks surrender will
 * cost them honour will drop the tab instead.
 *
 * Deliberately controlled rather than self-mounting: every honour surface opens
 * it (the top-bar chip, the profile band, the room badge, the ejection modal), so
 * each host owns a `useState` and renders its own instance. That is less
 * machinery than a shared store for a dialog, and it means the dialog is only
 * mounted where it can actually be opened.
 *
 * Shares the ejection modal's shell verbatim — brass hairline, 52px icon well,
 * mono eyebrow, tinted footer — so the two honour dialogs read as one family.
 * Mobile is this same Radix dialog at viewport width with margins; there is no
 * native sheet, because Beljot is one responsive web app.
 */
export function HonorExplainerDialog({ open, onOpenChange }: HonorExplainerDialogProps) {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);

  const isNew = honorIsNewPlayer(user?.isNewPlayer);
  const score = honorScoreOrPrior(user?.honorScore);
  const tier = normalizeHonorTier(user?.honorTier ?? "", score);

  // What moves the score, in the order that matters to a player deciding whether
  // to stay in a match. "Surrender" sits third on purpose: it reads as a
  // correction to the two facts above it.
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
          border: "1px solid var(--border-2)",
          borderRadius: "var(--radius-lg)",
          color: "var(--ink)",
          boxShadow: "0 40px 90px -30px rgba(14,58,36,0.55), 0 0 0 1px rgba(201,168,118,0.18)",
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
              boxShadow: "inset 0 0 0 4px rgba(201,168,118,0.10)",
            }}
          >
            <ShieldCheck size={24} color="var(--brass-deep)" />
          </div>
          <div style={{ flex: 1, minWidth: 0, paddingTop: 2 }}>
            <span
              style={{
                fontFamily: "var(--font-mono)",
                fontSize: 10.5,
                letterSpacing: 2,
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
                fontFamily: "var(--font-display)",
                fontWeight: 700,
                fontSize: 22,
                letterSpacing: -0.5,
                color: "var(--ink)",
                lineHeight: 1.15,
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
                <span className="text-ink-dim text-[13.5px] leading-normal">
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

          {/* Your standing, boxed as a callout in BOTH states (user decision
              2026-07-31): the newcomer variant is an instruction, the scored
              variant is the dialog's one personal fact, and either would
              otherwise read as one more body line. Brass tint + hairline, same
              mix vocabulary as the shell around it. */}
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
              // The tier's own glyph and colour — the same mark every other
              // honour surface draws for this player.
              <HonorShield tier={tier} size={16} className="mt-0.5 shrink-0" />
            )}
            <p className="text-ink m-0 text-[13.5px] leading-normal font-semibold">
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
          {/* Deep-links to the long-form rules section, so no other honour surface
              has to explain anything inline. */}
          <Link
            to="/rules#honour"
            onClick={() => onOpenChange(false)}
            className="text-accent hover:text-accent-deep inline-flex items-center gap-1.5 text-[13px] font-semibold"
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
              fontFamily: "var(--font-body)",
              fontSize: 14,
              fontWeight: 600,
              letterSpacing: -0.1,
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
