import { Coins } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { Avatar } from "@/shared/components/ui/avatar";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { formatLocalizedDate } from "@/shared/lib/formatDate";

import { daysSince } from "../lib/format";
import { WinRateRing } from "./WinRateRing";

type IdentityHeroProps = {
  username: string;
  createdAt: string;
  /** Completed-at of the most recent match, if any, for the "last played" line. */
  lastPlayedAt?: string;
  games: number;
  wins: number;
  losses: number;
  capots: number;
  /** Coin wallet balance (Story 9.1). */
  walletBalance: number;
  /** Daily-login streak (Story 9.1) — distinct from the win/loss streak. */
  loginStreakDays: number;
  /** 0–100, or null with no games. */
  winRate: number | null;
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
  /** When provided, replaces the visible text label; `label` is kept for the
      tooltip + accessible name so the meaning isn't lost (e.g. the coins pill). */
  icon?: ReactNode;
  /** Fuller hover-tooltip text that overrides the default `label` title (e.g. the
      streak pill explains "logged in N days in a row"). The accessible name stays
      the short `label` + value. */
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
        ? "rgba(201,168,118,0.40)"
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
      <span className="text-[13px] font-bold tabular-nums" style={{ color: valueColor }}>
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
  createdAt,
  lastPlayedAt,
  games,
  wins,
  losses,
  capots,
  walletBalance,
  loginStreakDays,
  winRate,
}: IdentityHeroProps) {
  const { t } = useTranslation();

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
      <div className="flex min-w-0 items-center gap-5">
        <Avatar name={username} size={96} halo="profile" />
        <div className="flex min-w-0 flex-col gap-2">
          <Eyebrow>{t("profile.eyebrow")}</Eyebrow>
          <h1
            className="text-ink font-display m-0 text-[clamp(32px,6vw,48px)] leading-[1.05] font-bold tracking-[-1.2px]"
            data-testid="profile-username"
          >
            {username}
          </h1>
          <div className="text-ink-dim flex flex-wrap items-center gap-2 text-[13px]">
            {memberSince && <span data-testid="profile-member-since">{memberSince}</span>}
            {memberSince && lastPlayed && <span className="text-ink-off">·</span>}
            {lastPlayed && <span>{lastPlayed}</span>}
          </div>
          <div className="mt-1.5 flex flex-wrap gap-2">
            <HeroPill value={games} label={t("profile.hero.games")} tone="neutral" />
            <HeroPill value={wins} label={t("profile.hero.wins")} tone="accent" />
            <HeroPill value={losses} label={t("profile.hero.losses")} tone="neutral" />
            <HeroPill value={capots} label={t("profile.hero.capots")} tone="brass" />
            {/* Matches the header coin pill: neutral surface + border, ink
                number, off-theme gold coin icon (see COIN_GOLD). */}
            <HeroPill
              value={formatCoins(walletBalance)}
              label={t("wallet.balanceLabel")}
              icon={<Coins className="size-3.5" style={{ color: COIN_GOLD }} aria-hidden="true" />}
              tone="neutral"
            />
            {loginStreakDays > 1 && (
              <HeroPill
                value={loginStreakDays}
                label={t("wallet.streakLabel")}
                titleText={t("wallet.streakTooltip", { days: loginStreakDays })}
                icon={
                  <span className="text-[13px] leading-none" aria-hidden="true">
                    🔥
                  </span>
                }
                tone="neutral"
              />
            )}
          </div>
        </div>
      </div>

      <div className="justify-self-center sm:justify-self-end">
        <WinRateRing rate={winRate} />
      </div>
    </header>
  );
}
