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
          letterSpacing: 1.2,
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
          fontFamily: "var(--font-display)",
          fontSize: 15,
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
 * Story 9.3: lobby arrival modal for a room ejection. It is the single consumer
 * of roomStore.roomEjection — set by the return-time 409 (MatchPage), the
 * per-user system:insolvent_ejected and system:honor_ejected pushes, and
 * system:room_closed_insolvent. Calm, non-panic copy with one clear action so
 * the player is never left at a dead end (per the UX "no dead ends" rule).
 *
 * Three copy branches, selected by `reason`: insolvency (balance vs buy-in),
 * honor (Story 9.8 — the player's score vs the room's minimum), and the
 * reason-agnostic room-closed notice.
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
          border: "1px solid var(--border-2)",
          borderRadius: "var(--radius-lg)",
          color: "var(--ink)",
          boxShadow: "0 40px 90px -30px rgba(14,58,36,0.55), 0 0 0 1px rgba(201,168,118,0.18)",
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

        {/* Header */}
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
              boxShadow: "inset 0 0 0 4px rgba(201,168,118,0.10)",
            }}
          >
            {roomClosed ? (
              <DoorOpen size={24} color="var(--brass-deep)" />
            ) : honor ? (
              <ShieldAlert size={24} color="var(--brass-deep)" />
            ) : (
              // Coins are always the off-theme gold (COIN_GOLD), even in a brass
              // icon well where its neighbours take --brass-deep: the door and the
              // shield are theme glyphs, a coin is a coin.
              <Coins size={24} color={COIN_GOLD} />
            )}
          </div>
          <div style={{ flex: 1, minWidth: 0, paddingTop: 2 }}>
            <span
              style={{
                fontFamily: "var(--font-mono)",
                fontSize: 10.5,
                letterSpacing: 2,
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
                fontFamily: "var(--font-display)",
                fontWeight: 700,
                fontSize: 22,
                letterSpacing: -0.5,
                color: "var(--ink)",
                lineHeight: 1.15,
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

        {/* Body */}
        <p
          data-testid="room-ejection-body"
          // Expose the honor numbers as data-* so tests assert the computed values
          // without depending on i18n wording. Absent on the other two branches.
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
              ? // ?? 0 is a fallback for a malformed notice only. The dispatch
                // handler rejects any payload whose numbers are not typeof
                // "number", so a real 0 arrives as 0 and renders as 0.
                t("lobby.roomEjection.honorBody", {
                  minHonor: ejection?.minHonor ?? 0,
                  honor: ejection?.honor ?? 0,
                })
              : t("lobby.roomEjection.insolventBody", {
                  balance: formatCoins(ejection?.balance ?? 0),
                  buyIn: formatCoins(ejection?.buyIn ?? 0),
                })}
        </p>

        {/* The comparison, DRAWN TO SCALE. A sentence with two numbers in it makes
            the player do the arithmetic; two bars on a shared 0-100 axis show at a
            glance how far off they are — which is the difference between "I'm
            locked out" and "I'm four matches away". Numbers stay alongside, so this
            never depends on the bars alone. */}
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

        {/* Footer. Stacks below sm — in DOM order, so the phone layout keeps
            the same action-then-dismiss reading as the desktop row. */}
        <div
          className="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-end"
          style={{
            marginTop: 18,
            padding: "16px 28px",
            borderTop: "1px solid var(--border)",
            background: "color-mix(in srgb, var(--surface-3) 45%, transparent)",
          }}
        >
          {/* A DOOR OUT, not just an acknowledgement. The honour branch is the one
              that removed something from the player, so it is the one that has to
              offer somewhere to go — the lobby, pre-filtered to rooms they can
              actually join. Without this the modal is a dead end, which is exactly
              what turns a nudge into a churn moment. */}
          {honor && (
            <button
              type="button"
              onClick={() => {
                close();
                navigate("/lobby", { state: { lobbyFilter: "qualify" } });
              }}
              data-testid="room-ejection-browse"
              className="text-ink-dim hover:text-ink border-border hover:border-border-2 inline-flex cursor-pointer items-center justify-center gap-2 rounded-[10px] border px-4 py-2.5 text-sm font-semibold transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
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
              fontFamily: "var(--font-body)",
              fontSize: 14,
              fontWeight: 600,
              letterSpacing: -0.1,
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
