import { Coins, DoorOpen } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Dialog, DialogClose, DialogContent } from "@/shared/components/ui/dialog";
import { formatCoins } from "@/shared/lib/formatCoins";
import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * Story 9.3: lobby arrival modal for an insolvency ejection. It is the single
 * consumer of roomStore.insolventEjection — set by the return-time 409
 * (MatchPage), the per-user system:insolvent_ejected push, and
 * system:room_closed_insolvent. Calm, non-panic copy with one clear action so
 * the player is never left at a dead end (per the UX "no dead ends" rule).
 */
export function InsolventEjectionModal() {
  const { t } = useTranslation();
  const ejection = useRoomStore((s) => s.insolventEjection);
  const setInsolventEjection = useRoomStore((s) => s.setInsolventEjection);

  const open = ejection !== null;
  const roomClosed = ejection?.reason === "roomClosed";

  const close = () => setInsolventEjection(null);

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <DialogContent
        showCloseButton={false}
        data-testid="insolvent-ejection-modal"
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
              {t("lobby.insolventEjection.eyebrow")}
            </span>
            <h2
              data-testid="insolvent-ejection-title"
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
                ? t("lobby.insolventEjection.roomClosedTitle")
                : t("lobby.insolventEjection.ejectedTitle")}
            </h2>
          </div>
        </div>

        {/* Body */}
        <p
          data-testid="insolvent-ejection-body"
          style={{
            margin: 0,
            padding: "12px 28px 0",
            fontSize: 14,
            color: "var(--ink-dim)",
            lineHeight: 1.6,
          }}
        >
          {roomClosed
            ? t("lobby.insolventEjection.roomClosedBody")
            : t("lobby.insolventEjection.ejectedBody", {
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
            data-testid="insolvent-ejection-action"
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
            {t("lobby.insolventEjection.action")}
          </DialogClose>
        </div>
      </DialogContent>
    </Dialog>
  );
}
