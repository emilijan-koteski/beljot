import { ChevronDown, Coins, LogOut, Menu, ShieldCheck } from "lucide-react";
import type { MouseEvent as ReactMouseEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link, NavLink, useLocation, useNavigate } from "react-router";

import { LanguageSelector } from "@/shared/components/LanguageSelector";
import { LevelRing } from "@/shared/components/LevelRing";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/components/ui/dropdown-menu";
import { XpBar } from "@/shared/components/XpBar";
import { useLobbyReturn } from "@/shared/hooks/useLobbyReturn";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import {
  honorIsNewPlayer,
  honorScoreOrPrior,
  type HonorTier,
  normalizeHonorTier,
} from "@/shared/lib/honor";
import { cn } from "@/shared/lib/utils";
import { xpBarFill } from "@/shared/lib/xpLevel";
import { useAuthStore } from "@/shared/stores/authStore";

// Honor tier tones for the header chip (Story 9.7). Same token vocabulary as
// the profile's HonorPanel — existing design tokens only, no new gold token.
const HONOR_TIER_COLOR: Record<HonorTier, string> = {
  exemplary: "var(--brass-deep)",
  trusted: "var(--accent)",
  fair: "var(--ink-off)",
  unreliable: "var(--ink-dim)",
  problematic: "var(--danger)",
};

const navItems = [
  { path: "/lobby", labelKey: "nav.play" },
  { path: "/profile", labelKey: "nav.profile" },
  { path: "/rules", labelKey: "nav.rules" },
] as const;

type TopBarProps = {
  /** Show nav links (Play / Profile / Rules). Default false. */
  showNav?: boolean;
  /** Show username pill + logout dropdown. Default false. */
  showUserMenu?: boolean;
  /**
   * When true, the LanguageSelector also pushes the picked language to the
   * server. AppLayout passes this; AuthLayout leaves it off.
   */
  persistLanguage?: boolean;
  /** Override the LanguageSelector's test-id prefix to preserve auth tests. */
  languageTestIdPrefix?: string;
  /**
   * Render the full "Beljot.online" wordmark and keep it visible at every
   * breakpoint (including phones). AuthLayout passes this so the pre-auth
   * screens show the complete brand instead of a logo-only mobile header.
   */
  showFullBrand?: boolean;
};

export function TopBar({
  showNav = false,
  showUserMenu = false,
  persistLanguage = false,
  languageTestIdPrefix,
  showFullBrand = false,
}: TopBarProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const returnToLobby = useLobbyReturn();
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);

  // History-stack shaping: navigating between the top-level pages must not
  // grow the stack — only the lobby → X hop is a push. Links whose target is
  // the lobby itself go through returnToLobby() (pop back to the live lobby
  // entry when it sits beneath, replace otherwise) while keeping their real
  // href for a11y / open-in-new-tab; modified clicks keep native behavior.
  const onLobbyPath = location.pathname === "/lobby";
  function handleLobbyLinkClick(e: ReactMouseEvent<HTMLAnchorElement>) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey)
      return;
    e.preventDefault();
    returnToLobby();
  }

  // Lifetime XP progress for the header level indicators (cosmetic fill; level
  // itself is server-authoritative). Computed once and shared by the phone ring
  // and the ≥sm "Lvl N + bar".
  const xp = user ? xpBarFill(user.totalXp, user.level) : null;

  // Honor for the header chip. The score rides the auth envelope, but the refresh
  // response is not type-guarded the way the WS payload is, so a bundle newer
  // than the server can leave it undefined — honorScoreOrPrior falls back to the
  // 80 prior instead of letting NaN render blank in danger red (code review
  // 2026-07-29). A real 0 survives: the check is Number.isFinite, not truthiness.
  //
  // The server sends the tier; normalize falls back to the score's own band if a
  // newer server ships a token this bundle has never seen, so version skew
  // degrades to a colour rather than a missing i18n key.
  // isNewPlayer defaults to SUPPRESSED when absent: unguarded, `undefined` was
  // falsy and took the numeric branch, so a server without the honor fields made
  // every account render a confident "80 / Fair" — including the newcomers the
  // flag exists to suppress (review pass 2). A real `false` survives.
  const honorScore = honorScoreOrPrior(user?.honorScore);
  const honorIsNew = honorIsNewPlayer(user?.isNewPlayer);
  const honorTier = user ? normalizeHonorTier(user.honorTier, honorScore) : "fair";

  // Clear auth state, then land on the public landing page ("/"). Without the
  // explicit navigate, ProtectedRoute would only fall back to /login.
  function handleLogout() {
    logout();
    navigate("/");
  }

  return (
    <nav
      className="sticky top-0 z-50 flex h-15 items-center border-b border-border bg-[rgba(245,242,232,0.85)] px-7 backdrop-blur-md"
      data-testid="app-nav"
    >
      <Link
        to={user ? "/lobby" : "/"}
        onClick={user ? handleLobbyLinkClick : undefined}
        data-testid="app-brand"
        aria-label={t("nav.appName")}
        className="flex items-center gap-2.5 rounded-md transition-opacity hover:opacity-80 focus-visible:ring-accent/50 focus-visible:outline-none focus-visible:ring-2"
      >
        <img
          src="/beljot-logo.svg"
          alt=""
          aria-hidden="true"
          className="size-7 shrink-0"
          data-testid="app-logo"
        />
        {/* Full "Beljot.online" wordmark — the ".online" suffix is brass, the
            same gold as the logo's border. By default it hides below md (the
            burger-header / mobile range) so the logo alone holds the left edge,
            freeing width for the coin balance + level + language + burger; the
            Link's aria-label still carries "Beljot" for assistive tech.

            showFullBrand keeps it visible at every breakpoint — used by the
            pre-auth screens, where the bar is otherwise empty. */}
        <span
          className={cn(
            "font-display text-ink text-xl font-semibold tracking-tight",
            showFullBrand ? "inline" : "hidden md:inline",
          )}
          data-testid="app-name"
        >
          {t("nav.appName")}
          <span className="text-brass font-medium">.online</span>
        </span>
      </Link>

      {showNav && (
        <div className="ml-7 hidden h-full items-center md:flex">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              replace={!onLobbyPath}
              onClick={item.path === "/lobby" ? handleLobbyLinkClick : undefined}
              className={({ isActive }) =>
                cn(
                  "flex h-full items-center px-4 text-sm font-medium transition-colors",
                  isActive
                    ? "border-accent text-ink border-b-2"
                    : "text-ink-dim hover:text-ink border-b-2 border-transparent",
                )
              }
              data-testid={`nav-${item.labelKey.split(".")[1]}`}
            >
              {t(item.labelKey)}
            </NavLink>
          ))}
        </div>
      )}

      <div className="ml-auto flex items-center gap-2.5">
        {/* Lifetime level + XP progress (Story 9.5). Level is server-authoritative
            (user.level); the fill is cosmetic display math. Live-updates via the
            event:xp_awarded handler that writes user.level / user.totalXp on the
            auth store. Two responsive treatments: a compact ring on phones, and
            the wider "Lvl N + bar" from the sm breakpoint up. */}
        {user && xp && (
          <>
            {/* Phones (<sm): compact level ring with the level centered, so the
                wider Lvl+bar doesn't crowd the coin pill on narrow screens. */}
            <div className="flex sm:hidden">
              <LevelRing
                level={user.level}
                fraction={xp.fraction}
                label={t("xp.progressLabel", {
                  level: user.level,
                  current: xp.xpIntoLevel,
                  next: xp.xpForNextLevel,
                })}
                testId="xp-ring"
              />
            </div>
            {/* ≥sm: level text + linear XP bar. */}
            <div className="hidden items-center gap-2 sm:flex" data-testid="xp-indicator">
              <span className="text-ink text-xs font-semibold tabular-nums" data-testid="xp-level">
                {t("xp.short", { level: user.level })}
              </span>
              <XpBar
                fraction={xp.fraction}
                label={t("xp.levelLabel", { level: user.level })}
                className="w-14"
                testId="xp-bar"
              />
            </div>
          </>
        )}

        {/* Coin balance pill (Story 9.1). Sits left of the language selector.
            Explicit number rendering — `0` is a real balance, never treated as
            falsy. The login streak is surfaced in the daily-reward dialog and the
            profile, not in the header. */}
        {user && (
          <div
            className="bg-surface-elevated flex items-center gap-1.5 rounded-full border border-border py-1 pr-3 pl-2.5"
            data-testid="coin-balance"
          >
            <Coins className="size-4" style={{ color: COIN_GOLD }} aria-hidden="true" />
            <span className="text-ink text-sm font-semibold tabular-nums">
              {formatCoins(user.walletBalance)}
            </span>
          </div>
        )}

        {/* Honor chip (Story 9.7). Sits right of the coin pill. The score and
            tier are server-authoritative (they ride the auth envelope, so they
            are correct on first paint) and live-update via the
            event:honor_updated handler that writes user.honorScore /
            user.honorTier on the auth store.

            Explicit number rendering — a score of 0 ("Problematic") is a real
            value, never treated as falsy. Colour is never the only signal: the
            shield icon carries the tier tone and the numeric score sits beside
            it, with the tier word in the tooltip.

            Below the match floor the server sets isNewPlayer and the chip shows
            the "New Player" label in place of the number, so a newcomer is not
            branded by a score computed from almost no evidence.

            Visible at EVERY width, like the coin pill beside it — it used to be
            sm:flex-only, which hid honor entirely on phones and left AC7 unmet on
            the primary viewport (code review 2026-07-29). Padding tightens below
            sm so three pills plus the language selector still fit; the tier word
            stays sr-only there rather than being dropped, so the accessible
            reading is identical at both widths.

            The New Player branch gets the same treatment for the same reason
            (review pass 2): "New Player" / "Нов играч" is the WIDEST content the
            chip can hold, it applies to every account's first five matches, and
            this row is a single non-wrapping flex line. Below sm the words go
            sr-only and the shield alone carries the state — the tooltip and the
            accessible reading are unchanged, and the profile's HonorPanel is where
            the full newcomer copy lives. */}
        {user && (
          <div
            className="bg-surface-elevated flex items-center gap-1.5 rounded-full border border-border py-1 pr-2.5 pl-2 sm:pr-3 sm:pl-2.5"
            data-testid="honor-chip"
            data-tier={honorTier}
            data-new-player={String(honorIsNew)}
            title={
              honorIsNew
                ? t("profile.honor.newPlayerHint")
                : t("profile.honor.meterLabel", {
                    score: honorScore,
                    tier: t(`profile.honor.tier.${honorTier}`),
                  })
            }
          >
            <ShieldCheck
              className="size-4"
              style={{ color: HONOR_TIER_COLOR[honorTier] }}
              aria-hidden="true"
            />
            {/* Names the chip for assistive tech, which would otherwise announce
                a bare "96 Exemplary" next to the coin balance with no clue that
                it is an honor score. The visible label is the icon. */}
            <span className="sr-only">{t("profile.honor.topBarLabel")}</span>
            {honorIsNew ? (
              <span
                className="text-ink-dim sr-only text-sm font-semibold sm:not-sr-only"
                data-testid="honor-new-player"
              >
                {t("profile.honor.newPlayerChip")}
              </span>
            ) : (
              <>
                <span
                  className="text-ink text-sm font-semibold tabular-nums"
                  data-testid="honor-score"
                >
                  {honorScore}
                </span>
                <span className="sr-only" data-testid="honor-tier">
                  {t(`profile.honor.tier.${honorTier}`)}
                </span>
              </>
            )}
          </div>
        )}

        <LanguageSelector persistToServer={persistLanguage} testIdPrefix={languageTestIdPrefix} />

        {/* Desktop (≥md): username pill with a logout dropdown. */}
        {showUserMenu && user && (
          <DropdownMenu>
            <DropdownMenuTrigger
              className="bg-surface-elevated hover:bg-surface-sunken aria-expanded:bg-surface-sunken hidden items-center gap-2 rounded-full border border-border py-1 pr-3 pl-1 transition-colors md:flex"
              data-testid="nav-user"
            >
              <span
                className="bg-accent text-accent-ink flex size-6.5 items-center justify-center rounded-full text-xs font-bold"
                aria-hidden="true"
              >
                {(user.username.charAt(0) || "?").toUpperCase()}
              </span>
              <span className="text-ink text-sm font-medium">{user.username}</span>
              <ChevronDown className="text-ink-dim size-3 opacity-70" />
            </DropdownMenuTrigger>
            <DropdownMenuContent
              align="end"
              className="bg-surface-elevated min-w-44 border border-border p-1 shadow-[0_14px_36px_-18px_rgba(14,58,36,0.30)]"
            >
              <div className="text-ink-mute px-2.5 pt-2 pb-1.5 text-[11px] tracking-[0.3px]">
                {t("nav.signedInAs", { defaultValue: "Signed in as" })}{" "}
                <span className="text-ink font-semibold">{user.username}</span>
              </div>
              <div className="mx-1 my-1 h-px bg-border" />
              <DropdownMenuItem
                onClick={handleLogout}
                data-testid="nav-logout"
                className="text-ink hover:bg-surface-sunken flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-medium"
              >
                <LogOut className="text-ink-dim size-3.5" />
                <span>{t("nav.logout")}</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}

        {/* Phones (<md): one hamburger folds the nav links + logout that the
            bar can't fit. The language picker stays beside it as its own icon. */}
        {(showNav || (showUserMenu && user)) && (
          <DropdownMenu>
            <DropdownMenuTrigger
              aria-label={t("nav.menu")}
              data-testid="nav-menu"
              className="text-ink-dim hover:bg-surface-sunken hover:text-ink aria-expanded:bg-surface-sunken aria-expanded:text-ink inline-flex size-8 items-center justify-center rounded-lg border border-border bg-transparent transition-colors md:hidden"
            >
              <Menu className="size-4.5" />
            </DropdownMenuTrigger>
            <DropdownMenuContent
              align="end"
              className="bg-surface-elevated min-w-48 border border-border p-1 shadow-[0_14px_36px_-18px_rgba(14,58,36,0.30)]"
            >
              {showUserMenu && user && (
                <>
                  <div className="text-ink-mute px-2.5 pt-2 pb-1.5 text-[11px] tracking-[0.3px]">
                    {t("nav.signedInAs", { defaultValue: "Signed in as" })}{" "}
                    <span className="text-ink font-semibold">{user.username}</span>
                  </div>
                  <div className="mx-1 my-1 h-px bg-border" />
                </>
              )}
              {showNav &&
                navItems.map((item) => {
                  const active = location.pathname === item.path;
                  return (
                    <DropdownMenuItem
                      key={item.path}
                      render={
                        <Link
                          to={item.path}
                          replace={!onLobbyPath}
                          onClick={item.path === "/lobby" ? handleLobbyLinkClick : undefined}
                        />
                      }
                      data-testid={`nav-menu-${item.labelKey.split(".")[1]}`}
                      className={cn(
                        "rounded-md px-2.5 py-2 text-sm",
                        active
                          ? "bg-accent-soft text-ink font-semibold"
                          : "text-ink hover:bg-surface-sunken font-medium",
                      )}
                    >
                      {t(item.labelKey)}
                    </DropdownMenuItem>
                  );
                })}
              {showNav && showUserMenu && user && <div className="mx-1 my-1 h-px bg-border" />}
              {showUserMenu && user && (
                <DropdownMenuItem
                  onClick={handleLogout}
                  data-testid="nav-menu-logout"
                  className="text-ink hover:bg-surface-sunken flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-medium"
                >
                  <LogOut className="text-ink-dim size-3.5" />
                  <span>{t("nav.logout")}</span>
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
    </nav>
  );
}
