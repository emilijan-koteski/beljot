import type { ReactNode } from "react";
import { useState } from "react";

import { cn } from "@/shared/lib/utils";

type AvatarTeam = "A" | "B" | null;

type AvatarImageProps = {
  src?: string | null;
  size: number;
  eager?: boolean;
  fallback: ReactNode;
  className?: string;
};

/**
 * The picture inside an avatar disc, falling back to the disc's own content when absent or broken.
 * Reserves its box, decodes async, and loads lazily unless `eager`.
 */
export function AvatarImage({ src, size, eager = false, fallback, className }: AvatarImageProps) {
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
  avatarUrl?: string | null;
  eager?: boolean;
  size?: number;
  team?: AvatarTeam;
  owner?: boolean;
  you?: boolean;
  halo?: "profile";
  icon?: ReactNode;
  className?: string;
};

/**
 * Circular player avatar showing the uploaded picture or the initial.
 * Fill follows team colors and the ring encodes role: brass for owner, accent for "you".
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
    background = "linear-gradient(135deg, #1c7a45 0%, var(--accent-deep) 100%)";
    textColor = "var(--accent-ink)";
  }

  let ringColor: string;
  if (owner) ringColor = "var(--brass)";
  else if (you) ringColor = "var(--accent)";
  else if (team === "A") ringColor = "var(--team-a)";
  else if (team === "B") ringColor = "var(--team-b)";
  else ringColor = "var(--border-2)";

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
