import { Coins, DoorOpen, ShieldAlert } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { Dialog, DialogClose, DialogContent } from "@/shared/components/ui/dialog";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { HONOR_TIER_COLOR, honorTierForScore } from "@/shared/lib/honor";
import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * One bar of the honour comparison, on a shared 0-100 axis so the two rows are
 * directly comparable. The number is always alongside the bar — the bar is the
 * quick read, not the only one.
 */
function ComparisonRow({
  label,
  value,
  color,
  testId,
}: {
  label: string;
  value: number;
  color: string;
  testId: string;
}) {
  return (
    <div
      style={{ display: "flex", alignItems: "center", gap: 10 }}
      data-testid={testId}
      data-value={value}
    >
      <span
        style={{
          fontFamily: "var(--font-mono)",
          fontSize: 10,
          letterSpacing: "0.1em",
          textTransform: "uppercase",
          color: "var(--ink-mute)",
          width: 52,
          flexShrink: 0,
        }}
      >
        {label}
      </span>
      <span
        style={{
          position: "relative",
          flex: 1,
          height: 8,
          borderRadius: 4,
          background: "var(--surface-3)",
          overflow: "hidden",
        }}
      >
        <span
          style={{
            position: "absolute",
            inset: "0 auto 0 0",
            width: `${Math.min(100, Math.max(0, value))}%`,
            borderRadius: 4,
            background: color,
          }}
        />
      </span>
      <span
        style={{
          fontSize: 16,
          fontWeight: 700,
          color: "var(--ink)",
          fontVariantNumeric: "tabular-nums",
          width: 28,
          textAlign: "right",
        }}
      >
        {value}
      </span>
    </div>
  );
}

/**
 * Lobby modal explaining why the player was removed from a room, read from roomStore.roomEjection.
 * Copy branches on `reason`: insolvency, honor below the room's minimum, or room closed.
 */
export function RoomEjectionModal() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const ejection = useRoomStore((s) => s.roomEjection);
  const setRoomEjection = useRoomStore((s) => s.setRoomEjection);

  const open = ejection !== null;
  const roomClosed = ejection?.reason === "roomClosed";
  const honor = ejection?.reason === "honor";

  const close = () => setRoomEjection(null);

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <DialogContent
        showCloseButton={false}
        data-testid="room-ejection-modal"
        style={{
          display: "block",
          width: 460,
          maxWidth: "calc(100% - 48px)",
          padding: 0,
          overflow: "hidden",
          background: "var(--surface)",
          border: "1px solid var(--brass)",
          borderRadius: "var(--radius-lg)",
          color: "var(--ink)",
          boxShadow: "0 40px 90px -30px rgba(14,58,36,0.55), 0 0 0 1px var(--brass-soft)",
        }}
      >
        <div
          style={{
            height: 3,
            width: "100%",
            opacity: 0.7,
            background: "linear-gradient(90deg, transparent, var(--brass), transparent)",
          }}
        />

        <div style={{ display: "flex", gap: 16, padding: "26px 28px 6px" }}>
          <div
            style={{
              flexShrink: 0,
              width: 52,
              height: 52,
              borderRadius: 14,
              background: "var(--brass-soft)",
              border: "1px solid color-mix(in srgb, var(--brass) 45%, transparent)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              boxShadow: "inset 0 0 0 4px var(--brass-10)",
            }}
          >
            {roomClosed ? (
              <DoorOpen size={24} color="var(--brass-deep)" />
            ) : honor ? (
              <ShieldAlert size={24} color="var(--brass-deep)" />
            ) : (
              <Coins size={24} color={COIN_GOLD} />
            )}
          </div>
          <div style={{ flex: 1, minWidth: 0, paddingTop: 2 }}>
            <span
              style={{
                fontFamily: "var(--font-mono)",
                fontSize: 10,
                letterSpacing: "0.18em",
                textTransform: "uppercase",
                fontWeight: 600,
                color: "var(--brass-deep)",
              }}
            >
              {t("lobby.roomEjection.eyebrow")}
            </span>
            <h2
              data-testid="room-ejection-title"
              style={{
                margin: "6px 0 0",
                fontWeight: 700,
                fontSize: 24,
                letterSpacing: "-0.025em",
                color: "var(--ink)",
                lineHeight: 1.2,
              }}
            >
              {roomClosed
                ? t("lobby.roomEjection.roomClosedTitle")
                : honor
                  ? t("lobby.roomEjection.honorTitle")
                  : t("lobby.roomEjection.insolventTitle")}
            </h2>
          </div>
        </div>

        <p
          data-testid="room-ejection-body"
          data-min-honor={honor ? ejection?.minHonor : undefined}
          data-honor={honor ? ejection?.honor : undefined}
          style={{
            margin: 0,
            padding: "12px 28px 0",
            fontSize: 14,
            color: "var(--ink-dim)",
            lineHeight: 1.6,
          }}
        >
          {roomClosed
            ? t("lobby.roomEjection.roomClosedBody")
            : honor
              ? t("lobby.roomEjection.honorBody", {
                  minHonor: ejection?.minHonor ?? 0,
                  honor: ejection?.honor ?? 0,
                })
              : t("lobby.roomEjection.insolventBody", {
                  balance: formatCoins(ejection?.balance ?? 0),
                  buyIn: formatCoins(ejection?.buyIn ?? 0),
                })}
        </p>

        {honor && typeof ejection?.honor === "number" && typeof ejection?.minHonor === "number" && (
          <div style={{ padding: "16px 28px 0", display: "flex", flexDirection: "column", gap: 8 }}>
            <ComparisonRow
              label={t("lobby.roomEjection.youLabel")}
              value={ejection.honor}
              color={HONOR_TIER_COLOR[honorTierForScore(ejection.honor)]}
              testId="room-ejection-you"
            />
            <ComparisonRow
              label={t("lobby.roomEjection.tableLabel")}
              value={ejection.minHonor}
              color="var(--ink-off)"
              testId="room-ejection-table"
            />
          </div>
        )}

        <div
          className="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-end"
          style={{
            marginTop: 18,
            padding: "16px 28px",
            borderTop: "1px solid var(--border)",
            background: "color-mix(in srgb, var(--surface-3) 45%, transparent)",
          }}
        >
          {honor && (
            <button
              type="button"
              onClick={() => {
                close();
                navigate("/lobby", { state: { lobbyFilter: "qualify" } });
              }}
              data-testid="room-ejection-browse"
              className="text-ink-dim hover:text-ink border-border hover:border-brass inline-flex cursor-pointer items-center justify-center gap-2 rounded-md border px-4 py-2.5 text-sm font-semibold transition-colors focus-visible:ring-3 focus-visible:ring-ring/70 focus-visible:outline-none"
            >
              {t("lobby.roomEjection.browseQualifying")}
            </button>
          )}
          <DialogClose
            data-testid="room-ejection-action"
            style={{
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              gap: 8,
              padding: "11px 18px",
              borderRadius: 10,
              fontSize: 14,
              fontWeight: 600,
              lineHeight: 1.2,
              cursor: "pointer",
              background: "linear-gradient(180deg, var(--brass) 0%, var(--brass-deep) 100%)",
              color: "var(--brass-ink)",
              border: "1px solid var(--brass-deep)",
              boxShadow:
                "0 6px 16px -8px rgba(156,125,78,0.65), inset 0 1px 0 rgba(255,255,255,0.25)",
            }}
          >
            {t("lobby.roomEjection.action")}
          </DialogClose>
        </div>
      </DialogContent>
    </Dialog>
  );
}
