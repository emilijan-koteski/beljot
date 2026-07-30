import { Coins, DoorOpen, ShieldAlert } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Dialog, DialogClose, DialogContent } from "@/shared/components/ui/dialog";
import { formatCoins } from "@/shared/lib/formatCoins";
import { useRoomStore } from "@/shared/stores/roomStore";

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
              <Coins size={24} color="var(--brass-deep)" />
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

        {/* Footer */}
        <div
          style={{
            marginTop: 18,
            display: "flex",
            justifyContent: "flex-end",
            padding: "16px 28px",
            borderTop: "1px solid var(--border)",
            background: "color-mix(in srgb, var(--surface-3) 45%, transparent)",
          }}
        >
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
