import { ArrowRight, Snowflake, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import type { CareerStreak } from "@/shared/api/career";

type StreakCalloutProps = {
  streak: CareerStreak;
  subjectIsSelf?: boolean;
};

const COLD_STREAK_ICE = {
  fill: "var(--team-b-tint)",
  border: "var(--team-b-edge-soft)",
  disc: "var(--team-b-fill)",
  icon: "var(--team-b)",
  title: "var(--team-b)",
  buttonBg: "var(--team-b)",
  buttonText: "var(--accent-ink)",
} as const;

/**
 * Win/loss streak banner between the hero and stats grid: felt green for wins, steel blue for losses.
 * Renders nothing when there is no active streak.
 */
export function StreakCallout({ streak, subjectIsSelf = true }: StreakCalloutProps) {
  const { t } = useTranslation();

  if (streak.length === 0 || streak.kind === "none") return null;
  const isWin = streak.kind === "win";

  return (
    <div
      className="border-border mb-5 flex items-center gap-3.5 rounded-lg border p-3.5"
      style={{
        background: isWin ? "var(--accent-soft)" : COLD_STREAK_ICE.fill,
        borderColor: isWin
          ? "color-mix(in srgb, var(--accent) 33%, transparent)"
          : COLD_STREAK_ICE.border,
      }}
      data-testid="profile-streak"
      data-streak-kind={streak.kind}
      data-streak-length={streak.length}
    >
      <span
        className="inline-flex size-9 shrink-0 items-center justify-center rounded-md"
        style={{
          background: isWin ? "var(--accent)" : COLD_STREAK_ICE.disc,
          color: isWin ? "var(--accent-ink)" : COLD_STREAK_ICE.icon,
        }}
      >
        {isWin ? <Sparkles className="size-5" /> : <Snowflake className="size-5" />}
      </span>
      <div className="flex min-w-0 flex-col gap-0.5">
        <span
          className="text-base font-semibold"
          style={{ color: isWin ? "var(--accent)" : COLD_STREAK_ICE.title }}
        >
          {isWin
            ? t("profile.streak.winTitle", { count: streak.length })
            : t("profile.streak.lossTitle", { count: streak.length })}
        </span>
        <span className="text-ink-dim text-xs">
          {subjectIsSelf
            ? isWin
              ? t("profile.streak.winSubtitle")
              : t("profile.streak.lossSubtitle")
            : isWin
              ? t("profile.streak.winSubtitlePublic")
              : t("profile.streak.lossSubtitlePublic")}
        </span>
      </div>
      {subjectIsSelf && (
        <Link
          to="/lobby"
          className="ml-auto inline-flex items-center gap-1.5 rounded-md px-3.5 py-2 text-sm font-semibold"
          style={{
            background: isWin ? "var(--accent)" : COLD_STREAK_ICE.buttonBg,
            color: isWin ? "var(--accent-ink)" : COLD_STREAK_ICE.buttonText,
          }}
          data-testid="profile-streak-play"
        >
          {t("profile.streak.play")}
          <ArrowRight className="size-3.5" />
        </Link>
      )}
    </div>
  );
}
