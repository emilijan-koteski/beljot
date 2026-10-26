import { Camera, Coins } from "lucide-react";
import type { ReactNode } from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Avatar } from "@/shared/components/ui/avatar";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { XpBar } from "@/shared/components/XpBar";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { formatLocalizedDate } from "@/shared/lib/formatDate";
import { xpFraction } from "@/shared/lib/xpLevel";

import { daysSince } from "../lib/format";
import { AvatarDialog } from "./AvatarDialog";
import { EditableUsername } from "./EditableUsername";
import { HonorHeroBand, type HonorHeroBandProps } from "./HonorHeroBand";
import { WinRateRing } from "./WinRateRing";

type IdentityHeroProps = {
  username: string;
  avatarUrl?: string | null;
  userId?: number;
  usernameChangedAt?: string | null;
  createdAt: string;
  lastPlayedAt?: string;
  games: number;
  wins: number;
  losses: number;
  capots: number;
  walletBalance: number;
  loginStreakDays: number;
  hidePrivatePills?: boolean;
  level: number;
  xpIntoLevel: number;
  xpForNextLevel: number;
  winRate: number | null;
  honor?: HonorHeroBandProps;
};

type PillTone = "neutral" | "accent" | "brass";

function HeroPill({
  value,
  label,
  icon,
  titleText,
  tone,
}: {
  value: number | string;
  label: string;
  icon?: ReactNode;
  titleText?: string;
  tone: PillTone;
}) {
  const bg =
    tone === "accent"
      ? "var(--accent-soft)"
      : tone === "brass"
        ? "var(--brass-soft)"
        : "var(--surface-2)";
  const border =
    tone === "accent"
      ? "rgba(25,101,54,0.33)"
      : tone === "brass"
        ? "var(--brass-40)"
        : "var(--border)";
  const valueColor =
    tone === "accent" ? "var(--accent)" : tone === "brass" ? "var(--brass-deep)" : "var(--ink)";
  const labelColor =
    tone === "accent" ? "var(--accent)" : tone === "brass" ? "var(--brass-deep)" : "var(--ink-dim)";
  return (
    <span
      className="inline-flex items-baseline gap-1.5 rounded-full px-2.5 py-1 text-xs"
      style={{ background: bg, border: `1px solid ${border}` }}
      title={titleText ?? (icon ? label : undefined)}
      aria-label={icon ? `${label} ${value}` : undefined}
    >
      {icon && (
        <span className="inline-flex self-center" style={{ color: valueColor }} aria-hidden="true">
          {icon}
        </span>
      )}
      <span className="text-sm font-bold tabular-nums" style={{ color: valueColor }}>
        {value}
      </span>
      {!icon && <span style={{ color: labelColor }}>{label}</span>}
    </span>
  );
}

/**
 * Profile identity header: large brass-haloed avatar, username, member-since +
 * last-played meta, a row of headline stat pills, and the featured win-rate
 * ring. Collapses the ring beneath the identity block on narrow screens.
 */
export function IdentityHero({
  username,
  avatarUrl = null,
  userId,
  usernameChangedAt,
  createdAt,
  lastPlayedAt,
  games,
  wins,
  losses,
  capots,
  walletBalance,
  loginStreakDays,
  level,
  xpIntoLevel,
  xpForNextLevel,
  winRate,
  honor,
  hidePrivatePills = false,
}: IdentityHeroProps) {
  const { t } = useTranslation();
  const [avatarDialogOpen, setAvatarDialogOpen] = useState(false);

  const memberSince = createdAt
    ? t("profile.memberSince", { date: formatLocalizedDate(createdAt, t, "long") })
    : "";

  let lastPlayed = "";
  if (lastPlayedAt) {
    const d = daysSince(lastPlayedAt);
    lastPlayed =
      d === 0
        ? t("profile.lastPlayed.today")
        : d === 1
          ? t("profile.lastPlayed.yesterday")
          : t("profile.lastPlayed.daysAgo", { count: d });
  }

  return (
    <header
      className="bg-surface border-border relative mb-5 grid grid-cols-1 items-center gap-6 overflow-hidden rounded-lg border p-6 sm:grid-cols-[minmax(0,1fr)_auto] sm:gap-8"
      style={{
        background:
          "radial-gradient(circle at 88% 50%, rgba(25,101,54,0.10), transparent 55%), var(--surface)",
      }}
      data-testid="profile-identity-hero"
    >
      <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-start gap-x-5 gap-y-4">
        <div className="relative shrink-0">
          <Avatar name={username} avatarUrl={avatarUrl} eager size={96} halo="profile" />
          {userId !== undefined && (
            <>
              <button
                type="button"
                onClick={() => setAvatarDialogOpen(true)}
                aria-label={t("profile.avatar.edit")}
                title={t("profile.avatar.edit")}
                className="bg-surface border-border text-ink hover:bg-surface-sunken focus-visible:ring-ring/70 absolute -right-1 -bottom-1 inline-flex size-8 cursor-pointer items-center justify-center rounded-full border shadow-sm transition-colors focus-visible:ring-3 focus-visible:outline-none"
                data-testid="profile-avatar-edit"
              >
                <Camera className="size-4" aria-hidden="true" />
              </button>
              <AvatarDialog
                open={avatarDialogOpen}
                onClose={() => setAvatarDialogOpen(false)}
                userId={userId}
                username={username}
                avatarUrl={avatarUrl}
              />
            </>
          )}
        </div>
        <div className="flex min-w-0 flex-col gap-2">
          <Eyebrow>{t("profile.eyebrow")}</Eyebrow>
          <EditableUsername
            username={username}
            userId={userId}
            usernameChangedAt={usernameChangedAt}
          />
          <div className="text-ink-dim flex flex-col gap-0.5 text-sm sm:flex-row sm:flex-wrap sm:items-center sm:gap-2">
            {memberSince && <span data-testid="profile-member-since">{memberSince}</span>}
            {memberSince && lastPlayed && <span className="text-ink-off hidden sm:inline">·</span>}
            {lastPlayed && <span>{lastPlayed}</span>}
          </div>
        </div>

        <div className="col-span-2 flex flex-wrap gap-2 sm:col-span-1 sm:col-start-2">
          <HeroPill value={games} label={t("profile.hero.games")} tone="neutral" />
          <HeroPill value={wins} label={t("profile.hero.wins")} tone="accent" />
          <HeroPill value={losses} label={t("profile.hero.losses")} tone="neutral" />
          <HeroPill value={capots} label={t("profile.hero.capots")} tone="brass" />
          {!hidePrivatePills && (
            <>
              <HeroPill
                value={formatCoins(walletBalance)}
                label={t("wallet.balanceLabel")}
                icon={
                  <Coins className="size-3.5" style={{ color: COIN_GOLD }} aria-hidden="true" />
                }
                tone="neutral"
              />
              {loginStreakDays > 1 && (
                <HeroPill
                  value={loginStreakDays}
                  label={t("wallet.streakLabel")}
                  titleText={t("wallet.streakTooltip", { days: loginStreakDays })}
                  icon={
                    <span className="text-sm leading-none" aria-hidden="true">
                      🔥
                    </span>
                  }
                  tone="neutral"
                />
              )}
            </>
          )}
        </div>

        <div
          className="col-span-2 flex max-w-xs flex-col gap-1 sm:col-span-1 sm:col-start-2"
          data-testid="profile-xp"
        >
          <div className="flex items-baseline justify-between gap-2 text-sm">
            <span className="text-ink font-semibold" data-testid="profile-level">
              {t("xp.levelLabel", { level })}
            </span>
            <span className="text-ink-dim tabular-nums">
              {t("xp.progress", { current: xpIntoLevel, next: xpForNextLevel })}
            </span>
          </div>
          <XpBar
            fraction={xpFraction(xpIntoLevel, xpForNextLevel)}
            label={t("xp.progressLabel", {
              level,
              current: xpIntoLevel,
              next: xpForNextLevel,
            })}
            testId="profile-xp-bar"
          />
        </div>
      </div>

      <div className="justify-self-center sm:justify-self-end">
        <WinRateRing rate={winRate} />
      </div>

      {honor && <HonorHeroBand {...honor} />}
    </header>
  );
}
