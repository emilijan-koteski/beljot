import { ChevronDown, Coins, LogOut, Menu } from "lucide-react";
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
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { cn } from "@/shared/lib/utils";
import { xpBarFill } from "@/shared/lib/xpLevel";
import { useAuthStore } from "@/shared/stores/authStore";

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
};

export function TopBar({
  showNav = false,
  showUserMenu = false,
  persistLanguage = false,
  languageTestIdPrefix,
}: TopBarProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);

  // Lifetime XP progress for the header level indicators (cosmetic fill; level
  // itself is server-authoritative). Computed once and shared by the phone ring
  // and the ≥sm "Lvl N + bar".
  const xp = user ? xpBarFill(user.totalXp, user.level) : null;

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
        {/* Wordmark hides below md (the burger-header / mobile range) so the
            logo alone holds the left edge — freeing width for a large coin
            balance + level + language + burger. The Link's aria-label still
            carries "Beljot" for assistive tech. */}
        <span
          className="font-display text-ink text-xl font-semibold tracking-tight hidden md:inline"
          data-testid="app-name"
        >
          {t("nav.appName")}
        </span>
      </Link>

      {showNav && (
        <div className="ml-7 hidden h-full items-center md:flex">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
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
                      render={<Link to={item.path} />}
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
