import { Coins } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { Avatar } from "@/shared/components/ui/avatar";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { XpBar } from "@/shared/components/XpBar";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { formatLocalizedDate } from "@/shared/lib/formatDate";
import { xpFraction } from "@/shared/lib/xpLevel";

import { daysSince } from "../lib/format";
import { EditableUsername } from "./EditableUsername";
import { WinRateRing } from "./WinRateRing";

type IdentityHeroProps = {
  username: string;
  /** Authenticated self's id — enables username edit-in-place (undefined = read-only). */
  userId?: number;
  /** When the username was last changed; drives the edit cooldown. */
  usernameChangedAt?: string | null;
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
  /** Lifetime level (Story 9.5) — server-derived from total XP. */
  level: number;
  /** XP earned past the current level's threshold (server-provided). */
  xpIntoLevel: number;
  /** Size of the current level's XP band (server-provided). */
  xpForNextLevel: number;
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
      {/* Left region is a 2-col grid: [avatar | identity]. The stat pills and XP
          block break to a full-width row beneath the avatar on mobile
          (col-span-2, flush to the left edge) and tuck into the identity column
          to the right of the avatar on sm+ (col-start-2). */}
      <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-start gap-x-5 gap-y-4">
        <Avatar name={username} size={96} halo="profile" />
        <div className="flex min-w-0 flex-col gap-2">
          <Eyebrow>{t("profile.eyebrow")}</Eyebrow>
          <EditableUsername
            username={username}
            userId={userId}
            usernameChangedAt={usernameChangedAt}
          />
          {/* Below sm the two meta lines stack on their own rows (no middot) —
              the inline pair wraps unpredictably next to long usernames on
              phones. sm+ keeps the single "member since · played" line. */}
          <div className="text-ink-dim flex flex-col gap-0.5 text-[13px] sm:flex-row sm:flex-wrap sm:items-center sm:gap-2">
            {memberSince && <span data-testid="profile-member-since">{memberSince}</span>}
            {memberSince && lastPlayed && <span className="text-ink-off hidden sm:inline">·</span>}
            {lastPlayed && <span>{lastPlayed}</span>}
          </div>
        </div>

        <div className="col-span-2 flex flex-wrap gap-2 sm:col-span-1 sm:col-start-2">
          {/* The lifetime level (Story 9.5) is shown in the labelled XP-progress
              block just below, so no separate level pill is repeated here. */}
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

        {/* Lifetime XP progress bar (Story 9.5, AC4). Level + numeric progress
            on top, the bar below. xpIntoLevel / xpForNextLevel are server-
            provided; the bar fill is cosmetic. Leaves room for the not-yet-
            built honor / prior-season rank surfaces (render nothing for them). */}
        <div
          className="col-span-2 flex max-w-xs flex-col gap-1 sm:col-span-1 sm:col-start-2"
          data-testid="profile-xp"
        >
          <div className="flex items-baseline justify-between gap-2 text-[13px]">
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
    </header>
  );
}
