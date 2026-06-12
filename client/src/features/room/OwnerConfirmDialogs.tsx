import { Bot, Crown, Settings, Shuffle, UserX, X, Zap } from "lucide-react";
import type { CSSProperties, ReactNode } from "react";
import { Trans, useTranslation } from "react-i18next";

import { Avatar } from "@/shared/components/ui/avatar";
import { Badge } from "@/shared/components/ui/badge";
import { Dialog, DialogClose, DialogContent } from "@/shared/components/ui/dialog";

type Tone = "brass" | "danger";
type ButtonTone = Tone | "ghost";

/** Target player a host action acts on (team is viewer-relative: A = your team). */
interface DialogTarget {
  name: string;
  seat: number | null;
  team: "A" | "B" | null;
}

// ── Filled brass / danger / ghost footer buttons ────────────────────────────
// The shared <Button> has no solid brass or solid danger fill, so the dialog
// footer builds its own to keep the rhythm identical across both confirmations.
const BUTTON_TONES: Record<ButtonTone, CSSProperties> = {
  ghost: {
    background: "transparent",
    color: "var(--ink-dim)",
    border: "1px solid var(--border)",
  },
  brass: {
    background: "linear-gradient(180deg, var(--brass) 0%, var(--brass-deep) 100%)",
    color: "var(--brass-ink)",
    border: "1px solid var(--brass-deep)",
    boxShadow: "0 6px 16px -8px rgba(156,125,78,0.65), inset 0 1px 0 rgba(255,255,255,0.25)",
  },
  danger: {
    background: "linear-gradient(180deg, #a8362a 0%, var(--danger) 100%)",
    color: "#fbeae6",
    border: "1px solid #6f2018",
    boxShadow: "0 6px 16px -8px rgba(139,42,31,0.6), inset 0 1px 0 rgba(255,255,255,0.18)",
  },
};

function DialogButton({
  children,
  tone = "ghost",
  icon,
  onClick,
  disabled,
  testId,
}: {
  children: ReactNode;
  tone?: ButtonTone;
  icon?: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  testId?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      data-testid={testId}
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
        whiteSpace: "nowrap",
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.6 : 1,
        transition: "all .15s ease",
        ...BUTTON_TONES[tone],
      }}
    >
      {icon}
      {children}
    </button>
  );
}

// ── Shared parchment confirm-dialog shell ───────────────────────────────────
function OwnerConfirmDialog({
  open,
  onOpenChange,
  tone,
  eyebrow,
  icon,
  title,
  titleTestId,
  lede,
  children,
  confirmLabel,
  confirmIcon,
  onConfirm,
  confirmPending,
  confirmTestId,
  cancelLabel,
  cancelTestId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tone: Tone;
  eyebrow: string;
  icon: ReactNode;
  title: ReactNode;
  titleTestId: string;
  lede: ReactNode;
  children: ReactNode;
  confirmLabel: string;
  confirmIcon: ReactNode;
  onConfirm: () => void;
  confirmPending: boolean;
  confirmTestId: string;
  cancelLabel: string;
  cancelTestId: string;
}) {
  const { t } = useTranslation();
  const isDanger = tone === "danger";
  const accentColor = isDanger ? "var(--danger)" : "var(--brass-deep)";
  const badgeBg = isDanger ? "rgba(139,42,31,0.10)" : "var(--brass-soft)";
  const badgeRing = isDanger
    ? "rgba(139,42,31,0.28)"
    : "color-mix(in srgb, var(--brass) 45%, transparent)";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        style={{
          display: "block",
          width: 484,
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
        {/* Brass/danger hairline — owner-action signature */}
        <div
          style={{
            height: 3,
            width: "100%",
            opacity: 0.7,
            background: isDanger
              ? "linear-gradient(90deg, transparent, var(--danger), transparent)"
              : "linear-gradient(90deg, transparent, var(--brass), transparent)",
          }}
        />

        {/* Close */}
        <DialogClose
          aria-label={t("room.ownerConfirm.close", { defaultValue: "Close" })}
          style={{
            position: "absolute",
            top: 16,
            right: 16,
            width: 30,
            height: 30,
            borderRadius: 8,
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
            color: "var(--ink-mute)",
            background: "transparent",
            border: "1px solid transparent",
            cursor: "pointer",
          }}
        >
          <X size={16} />
        </DialogClose>

        {/* Header */}
        <div style={{ display: "flex", gap: 16, padding: "26px 28px 6px" }}>
          <div
            style={{
              flexShrink: 0,
              width: 52,
              height: 52,
              borderRadius: 14,
              background: badgeBg,
              border: `1px solid ${badgeRing}`,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              boxShadow: isDanger ? "none" : "inset 0 0 0 4px rgba(201,168,118,0.10)",
            }}
          >
            {icon}
          </div>
          <div style={{ flex: 1, minWidth: 0, paddingTop: 2 }}>
            <span
              style={{
                fontFamily: "var(--font-mono)",
                fontSize: 10.5,
                letterSpacing: 2,
                textTransform: "uppercase",
                fontWeight: 600,
                color: accentColor,
              }}
            >
              {eyebrow}
            </span>
            <h2
              data-testid={titleTestId}
              style={{
                margin: "6px 0 0",
                fontFamily: "var(--font-display)",
                fontWeight: 700,
                fontSize: 23,
                letterSpacing: -0.5,
                color: "var(--ink)",
                lineHeight: 1.15,
              }}
            >
              {title}
            </h2>
          </div>
        </div>

        {/* Lede */}
        <p
          style={{
            margin: 0,
            padding: "10px 28px 0",
            fontSize: 14,
            color: "var(--ink-dim)",
            lineHeight: 1.6,
          }}
        >
          {lede}
        </p>

        {/* Body */}
        <div style={{ padding: "18px 28px 4px" }}>{children}</div>

        {/* Footer */}
        <div
          style={{
            marginTop: 12,
            display: "flex",
            alignItems: "center",
            justifyContent: "flex-end",
            gap: 10,
            padding: "16px 28px",
            borderTop: "1px solid var(--border)",
            background: "color-mix(in srgb, var(--surface-3) 45%, transparent)",
          }}
        >
          <DialogButton tone="ghost" testId={cancelTestId} onClick={() => onOpenChange(false)}>
            {cancelLabel}
          </DialogButton>
          <DialogButton
            tone={tone}
            icon={confirmIcon}
            onClick={onConfirm}
            disabled={confirmPending}
            testId={confirmTestId}
          >
            {confirmLabel}
          </DialogButton>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// ── Compact player row — the subject of the kick dialog ─────────────────────
function PlayerRow({ name, seat, team, icon }: DialogTarget & { icon?: ReactNode }) {
  const { t } = useTranslation();
  const seated = seat !== null && seat !== undefined;
  const relation = team === "A" ? "partner" : team === "B" ? "opponent" : null;
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 12,
        padding: "12px 14px",
        background: "var(--surface-2)",
        border: "1px solid var(--border)",
        borderRadius: 12,
      }}
    >
      <Avatar name={name} size={42} team={team} icon={icon} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            fontFamily: "var(--font-display)",
            fontWeight: 600,
            fontSize: 16,
            letterSpacing: -0.2,
            color: "var(--ink)",
          }}
        >
          {name}
        </div>
        <div style={{ marginTop: 2, fontSize: 12, color: "var(--ink-mute)" }}>
          <span style={{ fontFamily: "var(--font-mono)", letterSpacing: 0.4 }}>
            {seated
              ? t("room.ownerConfirm.seatLabel", { n: (seat as number) + 1 })
              : t("room.ownerConfirm.standing")}
          </span>
        </div>
      </div>
      {seated && relation && (
        <Badge tone={relation === "partner" ? "teamA" : "teamB"}>
          {t(relation === "partner" ? "room.seatTile.partner" : "room.seatTile.opponent")}
        </Badge>
      )}
    </div>
  );
}

interface ActionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  pending: boolean;
  target: DialogTarget;
}

// ── Kick player (destructive → danger) ──────────────────────────────────────
export function KickPlayerDialog({
  open,
  onOpenChange,
  onConfirm,
  pending,
  target,
}: ActionDialogProps) {
  const { t } = useTranslation();
  return (
    <OwnerConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      tone="danger"
      eyebrow={t("room.ownerConfirm.eyebrow")}
      icon={<UserX size={24} color="var(--danger)" />}
      titleTestId="kick-dialog-title"
      title={
        <Trans
          i18nKey="room.kickConfirm.title"
          values={{ name: target.name }}
          components={{ name: <span style={{ color: "var(--danger)" }} /> }}
        />
      }
      lede={
        <Trans
          i18nKey="room.kickConfirm.lede"
          values={{ name: target.name }}
          components={{ strong: <strong style={{ color: "var(--ink)" }} /> }}
        />
      }
      confirmLabel={t("room.kickConfirm.confirm")}
      confirmIcon={<UserX size={15} color="#fbeae6" />}
      onConfirm={onConfirm}
      confirmPending={pending}
      confirmTestId="kick-confirm"
      cancelLabel={t("room.kickConfirm.cancel")}
      cancelTestId="kick-cancel"
    >
      <PlayerRow name={target.name} seat={target.seat} team={target.team} />
    </OwnerConfirmDialog>
  );
}

// ── Remove bot (destructive → danger; kick pattern, Story 10.3) ─────────────
export function RemoveBotDialog({
  open,
  onOpenChange,
  onConfirm,
  pending,
  target,
}: ActionDialogProps) {
  const { t } = useTranslation();
  return (
    <OwnerConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      tone="danger"
      eyebrow={t("room.ownerConfirm.eyebrow")}
      icon={<Bot size={24} color="var(--danger)" />}
      titleTestId="remove-bot-dialog-title"
      title={
        <Trans
          i18nKey="room.removeBotConfirm.title"
          values={{ name: target.name }}
          components={{ name: <span style={{ color: "var(--danger)" }} /> }}
        />
      }
      lede={
        <Trans
          i18nKey="room.removeBotConfirm.lede"
          values={{ name: target.name }}
          components={{ strong: <strong style={{ color: "var(--ink)" }} /> }}
        />
      }
      confirmLabel={t("room.removeBotConfirm.confirm")}
      confirmIcon={<UserX size={15} color="#fbeae6" />}
      onConfirm={onConfirm}
      confirmPending={pending}
      confirmTestId="remove-bot-confirm"
      cancelLabel={t("room.removeBotConfirm.cancel")}
      cancelTestId="remove-bot-cancel"
    >
      <PlayerRow
        name={target.name}
        seat={target.seat}
        team={target.team}
        icon={<Bot aria-hidden="true" />}
      />
    </OwnerConfirmDialog>
  );
}

// ── Transfer ownership / Make host (constructive but irreversible → brass) ──
export function TransferOwnershipDialog({
  open,
  onOpenChange,
  onConfirm,
  pending,
  fromName,
  target,
}: ActionDialogProps & { fromName: string }) {
  const { t } = useTranslation();
  const capabilities = [
    {
      icon: <Zap size={13} color="var(--brass-deep)" />,
      label: t("room.ownerConfirm.capabilities.start"),
    },
    {
      icon: <Shuffle size={13} color="var(--brass-deep)" />,
      label: t("room.ownerConfirm.capabilities.seats"),
    },
    {
      icon: <UserX size={13} color="var(--brass-deep)" />,
      label: t("room.ownerConfirm.capabilities.players"),
    },
    {
      icon: <Settings size={13} color="var(--brass-deep)" />,
      label: t("room.ownerConfirm.capabilities.settings"),
    },
  ];
  const captionStyle: CSSProperties = {
    fontFamily: "var(--font-mono)",
    fontSize: 9.5,
    letterSpacing: 1,
    textTransform: "uppercase",
    fontWeight: 600,
    textAlign: "center",
  };
  return (
    <OwnerConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      tone="brass"
      eyebrow={t("room.ownerConfirm.eyebrow")}
      icon={<Crown size={24} color="var(--brass-deep)" />}
      titleTestId="transfer-dialog-title"
      title={t("room.transferConfirm.title")}
      lede={
        <Trans
          i18nKey="room.transferConfirm.lede"
          values={{ toName: target.name }}
          components={{ strong: <strong style={{ color: "var(--ink)" }} /> }}
        />
      }
      confirmLabel={t("room.transferConfirm.confirm")}
      confirmIcon={<Crown size={15} color="var(--brass-ink)" />}
      onConfirm={onConfirm}
      confirmPending={pending}
      confirmTestId="transfer-confirm"
      cancelLabel={t("room.transferConfirm.cancel")}
      cancelTestId="transfer-cancel"
    >
      {/* Crown handoff — leaves you, lands on them */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 10,
          padding: "14px 16px",
          marginBottom: 14,
          background: "var(--surface-2)",
          border: "1px solid var(--border)",
          borderRadius: 14,
        }}
      >
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 6,
            flex: 1,
          }}
        >
          <Avatar name={fromName} size={44} you owner />
          <span style={{ ...captionStyle, color: "var(--ink-mute)" }}>
            {t("room.ownerConfirm.youHost")}
          </span>
        </div>

        <div
          style={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 5,
            padding: "0 4px",
          }}
        >
          <span
            style={{
              width: 28,
              height: 28,
              borderRadius: "50%",
              background: "var(--brass-soft)",
              border: "1px solid color-mix(in srgb, var(--brass) 45%, transparent)",
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Crown size={14} color="var(--brass-deep)" />
          </span>
          <svg
            width="46"
            height="10"
            viewBox="0 0 46 10"
            fill="none"
            stroke="var(--brass)"
            strokeWidth="1.6"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden
          >
            <path d="M2 5h40" strokeDasharray="3 3" />
            <path d="M38 1l5 4-5 4" />
          </svg>
        </div>

        <div
          style={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 6,
            flex: 1,
          }}
        >
          <div style={{ position: "relative" }}>
            <Avatar name={target.name} size={44} team={target.team} />
            <span
              aria-hidden
              style={{
                position: "absolute",
                inset: -4,
                borderRadius: "50%",
                border: "2px solid var(--brass)",
                opacity: 0.55,
              }}
            />
          </div>
          <span style={{ ...captionStyle, color: "var(--brass-deep)", fontWeight: 700 }}>
            {t("room.ownerConfirm.newHost", { name: target.name })}
          </span>
        </div>
      </div>

      {/* What you hand over */}
      <span
        style={{
          fontFamily: "var(--font-mono)",
          fontSize: 10,
          letterSpacing: 1.6,
          textTransform: "uppercase",
          fontWeight: 600,
          color: "var(--brass-deep)",
        }}
      >
        {t("room.ownerConfirm.handOver")}
      </span>
      <div style={{ marginTop: 8, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
        {capabilities.map((c) => (
          <div
            key={c.label}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              padding: "9px 11px",
              borderRadius: 10,
              background: "color-mix(in srgb, var(--brass-soft) 60%, var(--surface-2))",
              border: "1px solid var(--border)",
              fontSize: 12.5,
              color: "var(--ink-dim)",
              fontWeight: 500,
            }}
          >
            {c.icon}
            <span style={{ minWidth: 0 }}>{c.label}</span>
          </div>
        ))}
      </div>
    </OwnerConfirmDialog>
  );
}
