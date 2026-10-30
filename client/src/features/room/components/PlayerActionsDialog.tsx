import { ArrowLeftRight, Bot, Crown, User, UserX } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HonorShield } from "@/shared/components/HonorShield";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import { normalizeHonorTier } from "@/shared/lib/honor";
import { cn } from "@/shared/lib/utils";
import type { RoomPlayer } from "@/shared/types/apiTypes";

interface PlayerActionsDialogProps {
  target: { seat: number; name: string; player: RoomPlayer } | null;
  canPromote: boolean;
  onSwap: () => void;
  onPromote: () => void;
  onKick: () => void;
  onRemoveBot: () => void;
  onClose: () => void;
}

/** The host's actions on a seated player, or on a bot, opened by tapping the seat. */
export function PlayerActionsDialog({
  target,
  canPromote,
  onSwap,
  onPromote,
  onKick,
  onRemoveBot,
  onClose,
}: PlayerActionsDialogProps) {
  const { t } = useTranslation();
  const player = target?.player;
  const bot = player?.isBot === true;
  const seat = target?.seat ?? 0;
  return (
    <AppDialog
      open={target !== null}
      onClose={onClose}
      icon={bot ? <Bot /> : <User />}
      title={target?.name ?? ""}
      description={player !== undefined && !bot ? <PlayerLine player={player} /> : undefined}
      primary={{ label: t("common.close"), onClick: onClose }}
      testId="player-actions-dialog"
    >
      <div className="flex flex-col">
        <ActionRow
          icon={<ArrowLeftRight />}
          label={t("room.playerActions.swap")}
          onClick={onSwap}
          testId={`swap-seat-${seat}`}
        />
        {!bot && canPromote && (
          <ActionRow
            icon={<Crown />}
            label={t("room.transferConfirm.confirm")}
            onClick={onPromote}
            testId={`promote-seat-${seat}`}
          />
        )}
        {bot ? (
          <ActionRow
            icon={<UserX />}
            label={t("room.removeBotConfirm.confirm")}
            onClick={onRemoveBot}
            testId={`remove-bot-${seat}`}
            danger
          />
        ) : (
          <ActionRow
            icon={<UserX />}
            label={t("room.kickConfirm.confirm")}
            onClick={onKick}
            testId={`kick-player-${seat}`}
            danger
          />
        )}
      </div>
    </AppDialog>
  );
}

function PlayerLine({ player }: { player: RoomPlayer }) {
  const { t } = useTranslation();
  return (
    <span className="inline-flex items-center gap-1.5 tabular-nums">
      {typeof player.level === "number" && t("xp.short", { level: player.level })}
      {typeof player.level === "number" && typeof player.honorScore === "number" && " · "}
      {typeof player.honorScore === "number" && (
        <>
          <HonorShield
            tier={normalizeHonorTier(player.honorTier ?? "", player.honorScore)}
            size={12}
          />
          {player.honorScore}
        </>
      )}
    </span>
  );
}

function ActionRow({
  icon,
  label,
  onClick,
  testId,
  danger = false,
}: {
  icon: ReactNode;
  label: string;
  onClick: () => void;
  testId: string;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      data-testid={testId}
      className="hover:bg-surface-sunken focus-visible:ring-ring/70 flex w-full cursor-pointer items-center gap-3 rounded-lg px-2 py-2.5 text-left outline-none focus-visible:ring-3"
    >
      <span
        className={cn(
          "flex size-9 shrink-0 items-center justify-center rounded-lg [&>svg]:size-4.5",
          danger ? "bg-danger/10 text-danger" : "bg-brass-soft text-brass-deep",
        )}
      >
        {icon}
      </span>
      <span className={cn("text-sm font-semibold", danger ? "text-danger" : "text-ink")}>
        {label}
      </span>
    </button>
  );
}
