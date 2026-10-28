import { Bot, Crown, UserX } from "lucide-react";
import { useTranslation } from "react-i18next";

import { AppDialog } from "@/shared/components/ui/app-dialog";

interface OwnerConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  pending: boolean;
  name: string;
}

/** The host's kick confirmation: the danger tone, titled by the player's name. */
export function KickPlayerDialog({
  open,
  onOpenChange,
  onConfirm,
  pending,
  name,
}: OwnerConfirmDialogProps) {
  const { t } = useTranslation();
  return (
    <AppDialog
      open={open}
      onClose={() => onOpenChange(false)}
      tone="danger"
      icon={<UserX />}
      title={<span data-testid="kick-dialog-title">{t("room.kickConfirm.title", { name })}</span>}
      description={t("room.kickConfirm.description")}
      pending={pending}
      primary={{ label: t("room.kickConfirm.confirm"), onClick: onConfirm, testId: "kick-confirm" }}
      secondary={{
        label: t("room.kickConfirm.cancel"),
        onClick: () => onOpenChange(false),
        testId: "kick-cancel",
      }}
    />
  );
}

/** The host's confirmation before taking a bot off the table. */
export function RemoveBotDialog({
  open,
  onOpenChange,
  onConfirm,
  pending,
  name,
}: OwnerConfirmDialogProps) {
  const { t } = useTranslation();
  return (
    <AppDialog
      open={open}
      onClose={() => onOpenChange(false)}
      tone="danger"
      icon={<Bot />}
      title={
        <span data-testid="remove-bot-dialog-title">
          {t("room.removeBotConfirm.title", { name })}
        </span>
      }
      description={t("room.removeBotConfirm.description")}
      pending={pending}
      primary={{
        label: t("room.removeBotConfirm.confirm"),
        onClick: onConfirm,
        testId: "remove-bot-confirm",
      }}
      secondary={{
        label: t("room.removeBotConfirm.cancel"),
        onClick: () => onOpenChange(false),
        testId: "remove-bot-cancel",
      }}
    />
  );
}

/** The host's confirmation before handing the room to another player. */
export function TransferOwnershipDialog({
  open,
  onOpenChange,
  onConfirm,
  pending,
  name,
}: OwnerConfirmDialogProps) {
  const { t } = useTranslation();
  return (
    <AppDialog
      open={open}
      onClose={() => onOpenChange(false)}
      icon={<Crown />}
      title={
        <span data-testid="transfer-dialog-title">{t("room.transferConfirm.title", { name })}</span>
      }
      description={t("room.transferConfirm.description")}
      pending={pending}
      primary={{
        label: t("room.transferConfirm.confirm"),
        onClick: onConfirm,
        testId: "transfer-confirm",
      }}
      secondary={{
        label: t("room.transferConfirm.cancel"),
        onClick: () => onOpenChange(false),
        testId: "transfer-cancel",
      }}
    />
  );
}
