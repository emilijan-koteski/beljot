import { ChevronDown, Coins, HandHeart, LogOut, Menu } from "lucide-react";
import { type MouseEvent as ReactMouseEvent, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, NavLink, useLocation, useNavigate } from "react-router";

import { LanguageSelector } from "@/shared/components/LanguageSelector";
import { LevelRing } from "@/shared/components/LevelRing";
import { HeaderRankChip } from "@/shared/components/season/HeaderRankChip";
import { SupportDialog } from "@/shared/components/support/SupportDialog";
import { Avatar } from "@/shared/components/ui/avatar";
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
import { cn } from "@/shared/lib/utils";
import { xpBarFill } from "@/shared/lib/xpLevel";
import { useAuthStore } from "@/shared/stores/authStore";

const navItems = [
  { path: "/lobby", labelKey: "nav.play" },
  { path: "/leaderboard", labelKey: "nav.leaderboard" },
  { path: "/profile", labelKey: "nav.profile" },
  { path: "/rules", labelKey: "nav.rules" },
] as const;

type TopBarProps = {
  showNav?: boolean;
  showUserMenu?: boolean;
  persistLanguage?: boolean;
  languageTestIdPrefix?: string;
  showFullBrand?: boolean;
};

/**
 * App header with the brand, nav links, progression readouts, wallet, language picker and user menus.
 * The SupportDialog is mounted outside both dropdowns because a dropdown unmounts its subtree on select.
 */
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
  const [supportOpen, setSupportOpen] = useState(false);

  const onLobbyPath = location.pathname === "/lobby";
  /** Returns to the lobby without growing the history stack; modified clicks keep native link behavior. */
  function handleLobbyLinkClick(e: ReactMouseEvent<HTMLAnchorElement>) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey)
      return;
    e.preventDefault();
    returnToLobby();
  }

  const xp = user ? xpBarFill(user.totalXp, user.level) : null;

  /** Clears auth state and lands on the public landing page rather than /login. */
  function handleLogout() {
    logout();
    navigate("/");
  }

  return (
    <nav
      className="sticky top-0 z-50 flex h-15 items-center border-b border-border bg-background/85 px-4 backdrop-blur-md lg:px-7"
      data-testid="app-nav"
    >
      <Link
        to={user ? "/lobby" : "/"}
        onClick={user ? handleLobbyLinkClick : undefined}
        data-testid="app-brand"
        aria-label={t("nav.appName")}
        className="flex shrink-0 items-center gap-2.5 rounded-md transition-opacity hover:opacity-80 focus-visible:ring-ring/70 focus-visible:outline-none focus-visible:ring-3"
      >
        <img
          src="/beljot-logo.svg"
          alt=""
          aria-hidden="true"
          className="size-7 shrink-0"
          data-testid="app-logo"
        />
        <span
          className={cn(
            "text-ink text-xl font-semibold tracking-tight whitespace-nowrap",
            showFullBrand ? "inline" : "hidden lg:inline",
          )}
          data-testid="app-name"
        >
          {t("nav.appName")}
          <span className="text-brass font-medium">.online</span>
        </span>
      </Link>

      {showNav && (
        <div className="ml-4 hidden h-full shrink-0 items-center md:flex lg:ml-7">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              replace={!onLobbyPath}
              onClick={item.path === "/lobby" ? handleLobbyLinkClick : undefined}
              className={({ isActive }) =>
                cn(
                  "flex h-full items-center whitespace-nowrap px-2.5 text-sm font-medium transition-colors lg:px-4",
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

      <div className="ml-auto flex min-w-0 items-center gap-1.5 sm:gap-2 lg:gap-2.5">
        {user && <HeaderRankChip />}

        {user && xp && (
          <>
            <div className="flex shrink-0 sm:hidden">
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
            <div className="hidden shrink-0 items-center gap-2 sm:flex" data-testid="xp-indicator">
              <span
                className="text-ink text-xs font-semibold whitespace-nowrap tabular-nums"
                data-testid="xp-level"
              >
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

        {user && (
          <div
            className="bg-surface-elevated flex shrink-0 items-center gap-1.5 rounded-full border border-border py-1 pr-3 pl-2.5"
            data-testid="coin-balance"
          >
            <Coins className="size-4 shrink-0" style={{ color: COIN_GOLD }} aria-hidden="true" />
            <span className="text-ink text-sm font-semibold whitespace-nowrap tabular-nums">
              {formatCoins(user.walletBalance)}
            </span>
          </div>
        )}

        <LanguageSelector persistToServer={persistLanguage} testIdPrefix={languageTestIdPrefix} />

        {showUserMenu && user && (
          <DropdownMenu>
            <DropdownMenuTrigger
              className="bg-surface-elevated hover:bg-surface-sunken aria-expanded:bg-surface-sunken hidden min-w-0 items-center gap-2 rounded-full border border-border py-1 pr-3 pl-1 transition-colors md:flex"
              data-testid="nav-user"
            >
              <Avatar
                name={user.username}
                avatarUrl={user.avatarUrl}
                size={26}
                testId="nav-user-avatar"
              />
              <span className="text-ink max-w-24 truncate text-sm font-medium lg:max-w-40">
                {user.username}
              </span>
              <ChevronDown className="text-ink-dim size-3 shrink-0 opacity-70" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-max min-w-44 max-w-(--available-width)">
              <div className="text-ink-mute px-2.5 pt-2 pb-1.5 text-xs">
                {t("nav.signedInAs", { defaultValue: "Signed in as" })}{" "}
                <span className="text-ink font-semibold">{user.username}</span>
              </div>
              <div className="mx-1 my-1 h-px bg-border" />
              <DropdownMenuItem
                onClick={() => setSupportOpen(true)}
                data-testid="nav-support"
                className="text-ink hover:bg-surface-sunken flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-medium"
              >
                <HandHeart className="text-ink-dim size-3.5" />
                <span className="whitespace-nowrap">{t("support.label")}</span>
              </DropdownMenuItem>
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

        {(showNav || (showUserMenu && user)) && (
          <DropdownMenu>
            <DropdownMenuTrigger
              aria-label={t("nav.menu")}
              data-testid="nav-menu"
              className="text-ink-dim hover:bg-surface-sunken hover:text-ink aria-expanded:bg-surface-sunken aria-expanded:text-ink inline-flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-transparent transition-colors md:hidden"
            >
              <Menu className="size-5" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-max min-w-48 max-w-(--available-width)">
              {showUserMenu && user && (
                <>
                  <div className="text-ink-mute px-2.5 pt-2 pb-1.5 text-xs">
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
              {showNav && <div className="mx-1 my-1 h-px bg-border" />}
              <DropdownMenuItem
                onClick={() => setSupportOpen(true)}
                data-testid="nav-menu-support"
                className="text-ink hover:bg-surface-sunken flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-medium"
              >
                <HandHeart className="text-ink-dim size-3.5" />
                <span className="whitespace-nowrap">{t("support.label")}</span>
              </DropdownMenuItem>
              {showUserMenu && user && <div className="mx-1 my-1 h-px bg-border" />}
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

      <SupportDialog open={supportOpen} onOpenChange={setSupportOpen} />
    </nav>
  );
}
