import type { ReactNode } from "react";
import { useState } from "react";

import { cn } from "@/shared/lib/utils";

type AvatarTeam = "A" | "B" | null;

type AvatarImageProps = {
  /** The picture; null, undefined or "" renders the fallback straight away. */
  src?: string | null;
  /** Rendered box in CSS px, reserved up front so nothing shifts on load. */
  size: number;
  /** Load now instead of near the viewport (above-the-fold discs only). */
  eager?: boolean;
  /** What shows when there is no picture or it fails to load. */
  fallback: ReactNode;
  className?: string;
};

/**
 * The picture inside an avatar disc, with the disc's own content as the
 * fallback. Shared by `Avatar` and the hand-built discs (match seat, nav pill,
 * lobby chips, leaderboard) so every disc loads and fails the same way: a
 * reserved width/height box, async decoding, lazy unless `eager`, and on error
 * the fallback instead of a broken-image glyph.
 */
export function AvatarImage({ src, size, eager = false, fallback, className }: AvatarImageProps) {
  // The URL that failed to load, not a boolean: a new upload is a new URL, so
  // it gets its own attempt without an effect to reset the flag.
  const [failedUrl, setFailedUrl] = useState<string | null>(null);
  if (typeof src !== "string" || src === "" || src === failedUrl) {
    return <>{fallback}</>;
  }
  return (
    <img
      src={src}
      alt=""
      width={size}
      height={size}
      decoding="async"
      loading={eager ? "eager" : "lazy"}
      draggable={false}
      className={cn("size-full rounded-full object-cover", className)}
      onError={() => setFailedUrl(src)}
    />
  );
}

type AvatarProps = {
  name: string;
  /**
   * The player's uploaded picture. When set it fills the disc (inside the
   * ring); when absent, or if it fails to load, the initial renders instead.
   */
  avatarUrl?: string | null;
  /**
   * Load the picture immediately instead of when it scrolls near the
   * viewport. Only for the one above-the-fold disc (the profile hero); every
   * list and seat disc stays lazy so off-screen discs cost nothing.
   */
  eager?: boolean;
  size?: number;
  team?: AvatarTeam;
  owner?: boolean;
  you?: boolean;
  /**
   * "profile" gives the large profile-hero treatment: a surface gap + brass
   * outer ring + soft felt drop shadow (instead of the thin role ring). Used by
   * the profile Identity Hero.
   */
  halo?: "profile";
  /**
   * Replaces the initial with a glyph (e.g. the bot marker for bot seats).
   * Pass a bare lucide icon — it is sized proportionally to the avatar the
   * same way the initial is.
   */
  icon?: ReactNode;
  className?: string;
};

/**
 * Circular player avatar used by seat tiles, rosters, lists and the profile
 * hero: the uploaded picture when there is one, else the initial. Fill follows
 * team colors when set; the ring color encodes role — brass for owner, accent
 * for "you", team color otherwise. Owner gets a soft brass halo via box-shadow
 * so they read at a glance across the diamond. The ring and halo frame the
 * picture exactly as they frame the initial.
 */
export function Avatar({
  name,
  avatarUrl,
  eager = false,
  size = 36,
  team = null,
  owner,
  you,
  halo,
  icon,
  className,
}: AvatarProps) {
  const initial = (name || "?").charAt(0).toUpperCase();

  let background: string;
  let textColor: string;
  if (team === "A") {
    background = "var(--team-a-fill)";
    textColor = "#3b2c08";
  } else if (team === "B") {
    background = "var(--team-b-fill)";
    textColor = "#2c2f35";
  } else {
    // Neutral / undetermined — felt-green. Used for standing members and for
    // everyone while the viewer is still unseated (no Us/Them perspective yet).
    background = "linear-gradient(135deg, #1c7a45 0%, var(--accent-deep) 100%)";
    textColor = "var(--accent-ink)";
  }

  let ringColor: string;
  if (owner) ringColor = "var(--brass)";
  else if (you) ringColor = "var(--accent)";
  else if (team === "A") ringColor = "var(--team-a)";
  else if (team === "B") ringColor = "var(--team-b)";
  else ringColor = "var(--border-2)";

  // Profile hero: surface gap + brass outer ring + soft felt drop shadow,
  // rendered entirely via box-shadow so the gradient fill stays edge-to-edge.
  const isProfile = halo === "profile";
  const border = isProfile ? "none" : `2px solid ${ringColor}`;
  let boxShadow = "none";
  if (isProfile) {
    boxShadow =
      "0 0 0 4px var(--surface), 0 0 0 5px var(--brass), 0 16px 36px -16px rgba(25,101,54,0.55)";
  } else if (owner) {
    boxShadow = "0 0 0 3px rgba(201,168,118,0.20)";
  }

  return (
    <div
      className={cn(
        "font-display inline-flex shrink-0 items-center justify-center rounded-full font-bold",
        className,
      )}
      style={{
        width: size,
        height: size,
        background,
        color: textColor,
        border,
        fontSize: Math.max(11, size * 0.42),
        letterSpacing: -0.3,
        boxShadow,
      }}
      aria-hidden="true"
    >
      {icon ? (
        <span
          data-testid="avatar-icon"
          className="inline-flex items-center justify-center [&_svg]:h-full [&_svg]:w-full"
          // The 5% upward shift is optical: the lucide Bot glyph's visual
          // mass (the robot head) sits low in its viewBox, so geometric
          // centering reads as off-center — especially at chip sizes.
          style={{ width: size * 0.52, height: size * 0.52, transform: "translateY(-5%)" }}
        >
          {icon}
        </span>
      ) : (
        <AvatarImage src={avatarUrl} size={size} eager={eager} fallback={initial} />
      )}
    </div>
  );
}
