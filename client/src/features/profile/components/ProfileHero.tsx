import { Coins, Flame } from "lucide-react";
import type { CSSProperties, ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HonorShield } from "@/shared/components/HonorShield";
import { TierBadge } from "@/shared/components/season/TierBadge";
import { Avatar } from "@/shared/components/ui/avatar";
import { XpBar } from "@/shared/components/XpBar";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { formatLocalizedDate } from "@/shared/lib/formatDate";
import {
  HONOR_TIER_COLOR,
  honorIsNewPlayer,
  honorScoreOrPrior,
  normalizeHonorTier,
} from "@/shared/lib/honor";
import {
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  SEASON_TIER_LINE,
  seasonRankLabel,
  seasonSpOrZero,
} from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";
import { xpFraction } from "@/shared/lib/xpLevel";
import type { SeasonRank } from "@/shared/types/apiTypes";

import { daysSince } from "../lib/format";

type ProfileHeroProps = {
  username: string;
  avatarUrl?: string | null;
  createdAt: string;
  lastPlayedAt?: string;
  level: number;
  xpIntoLevel: number;
  xpForNextLevel: number;
  seasonRank?: SeasonRank | null;
  honor?: { score: number; tier: string; isNewPlayer: boolean };
  walletBalance?: number;
  loginStreakDays?: number;
  action?: ReactNode;
  layout: "phone" | "wide";
};

function Pill({
  children,
  label,
  style,
  testId,
}: {
  children: ReactNode;
  label?: string;
  style?: CSSProperties;
  testId?: string;
}) {
  return (
    <span
      className="border-border bg-surface inline-flex h-7 items-center gap-1.5 rounded-full border px-2.5 text-xs font-semibold"
      style={style}
      aria-label={label}
      data-testid={testId}
    >
      {children}
    </span>
  );
}

/** The "Member since … · Played today" line under the name. */
function useMetaLine(createdAt: string, lastPlayedAt: string | undefined): string[] {
  const { t } = useTranslation();
  const parts: string[] = [];
  if (createdAt)
    parts.push(t("profile.memberSince", { date: formatLocalizedDate(createdAt, t, "long") }));
  if (lastPlayedAt) {
    const d = daysSince(lastPlayedAt);
    parts.push(
      d === 0
        ? t("profile.lastPlayed.today")
        : d === 1
          ? t("profile.lastPlayed.yesterday")
          : t("profile.lastPlayed.daysAgo", { count: d }),
    );
  }
  return parts;
}

/**
 * The profile header: picture, name, when they joined and last played, season rank and honor chips, level progress,
 * and the page's one action (Settings on your own profile, the friend button on someone else's).
 */
export function ProfileHero({
  username,
  avatarUrl = null,
  createdAt,
  lastPlayedAt,
  level,
  xpIntoLevel,
  xpForNextLevel,
  seasonRank,
  honor,
  walletBalance,
  loginStreakDays,
  action,
  layout,
}: ProfileHeroProps) {
  const { t } = useTranslation();
  const meta = useMetaLine(createdAt, lastPlayedAt);
  const wide = layout === "wide";

  let rankChip: ReactNode = null;
  if (seasonRank) {
    const sp = seasonSpOrZero(seasonRank.sp);
    const tier = normalizeSeasonTier(seasonRank.tier, sp);
    rankChip = (
      <Pill style={{ borderColor: SEASON_TIER_LINE[tier] }} testId="profile-hero-rank">
        <TierBadge tier={tier} size="sm" glow={false} className="-ml-1.5 size-5 [&>svg]:size-3" />
        <span style={{ color: SEASON_TIER_COLOR[tier] }}>
          {seasonRankLabel(t, tier, normalizeSeasonDivision(tier, seasonRank.division))}
        </span>
      </Pill>
    );
  }

  let honorChip: ReactNode = null;
  if (honor) {
    const score = honorScoreOrPrior(honor.score);
    const tier = normalizeHonorTier(honor.tier, score);
    const fresh = honorIsNewPlayer(honor.isNewPlayer);
    honorChip = (
      <Pill testId="profile-hero-honor">
        <HonorShield tier={fresh ? "fair" : tier} size={14} />
        {fresh ? (
          <span>{t("profile.honor.newPlayerChip")}</span>
        ) : (
          <>
            <span className="tabular-nums">{score}</span>
            <span style={{ color: HONOR_TIER_COLOR[tier] }}>{t(`profile.honor.tier.${tier}`)}</span>
          </>
        )}
      </Pill>
    );
  }

  const chips = (
    <div className={cn("flex flex-wrap gap-1.5", !wide && "justify-center")}>
      {rankChip}
      {honorChip}
      {walletBalance !== undefined && (
        <Pill label={`${t("wallet.balanceLabel")} ${formatCoins(walletBalance)}`}>
          <Coins className="size-3.5" style={{ color: COIN_GOLD }} aria-hidden="true" />
          <span className="tabular-nums">{formatCoins(walletBalance)}</span>
        </Pill>
      )}
      {loginStreakDays !== undefined && loginStreakDays > 1 && (
        <Pill label={`${t("wallet.streakLabel")} ${loginStreakDays}`}>
          <Flame className="text-brass size-3.5" aria-hidden="true" />
          <span
            className="tabular-nums"
            title={t("wallet.streakTooltip", { days: loginStreakDays })}
          >
            {loginStreakDays}
          </span>
        </Pill>
      )}
    </div>
  );

  const xp = (
    <div className={cn("flex w-full flex-col gap-1", wide && "max-w-xs")} data-testid="profile-xp">
      <div className="flex items-baseline justify-between gap-2 text-xs">
        <span className="text-ink font-semibold" data-testid="profile-level">
          {t("xp.levelLabel", { level })}
        </span>
        <span className="text-ink-mute tabular-nums">
          {t("xp.progress", { current: xpIntoLevel, next: xpForNextLevel })}
        </span>
      </div>
      <XpBar
        fraction={xpFraction(xpIntoLevel, xpForNextLevel)}
        label={t("xp.progressLabel", { level, current: xpIntoLevel, next: xpForNextLevel })}
        testId="profile-xp-bar"
      />
    </div>
  );

  const name = (
    <h1
      className={cn(
        "text-ink m-0 font-semibold tracking-tight [overflow-wrap:anywhere]",
        wide ? "text-3xl" : "text-2xl",
      )}
      data-testid="profile-username"
    >
      {username}
    </h1>
  );
  const metaLine = (
    <span
      className={cn("text-ink-mute", wide ? "text-sm" : "text-xs")}
      data-testid="profile-member-since"
    >
      {meta.join(" · ")}
    </span>
  );
  const card =
    "bg-surface border-border relative rounded-xl border shadow-xs [background:radial-gradient(circle_at_85%_0%,color-mix(in_srgb,var(--accent)_9%,transparent),transparent_55%),var(--surface)]";

  if (!wide) {
    return (
      <header
        className={cn(card, "flex flex-col items-center gap-3 px-4 pt-6 pb-4 text-center")}
        data-testid="profile-identity-hero"
      >
        <Avatar name={username} avatarUrl={avatarUrl} eager size={84} halo="profile" />
        <div className="mt-1 flex flex-col gap-0.5">
          {name}
          {metaLine}
        </div>
        {chips}
        {xp}
        {action !== undefined && <div className="flex w-full justify-center">{action}</div>}
      </header>
    );
  }

  return (
    <header
      className={cn(card, "grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-5 p-5")}
      data-testid="profile-identity-hero"
    >
      <Avatar name={username} avatarUrl={avatarUrl} eager size={88} halo="profile" />
      <div className="flex min-w-0 flex-col gap-2">
        <div className="flex flex-col gap-0.5">
          {name}
          {metaLine}
        </div>
        {chips}
      </div>
      <div className="flex flex-col items-end gap-3 self-stretch">
        {action}
        <div className="mt-auto w-56">{xp}</div>
      </div>
    </header>
  );
}
