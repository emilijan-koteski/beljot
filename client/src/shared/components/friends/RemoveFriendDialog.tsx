import { UserMinus } from "lucide-react";
import { useTranslation } from "react-i18next";

import { AppDialog } from "@/shared/components/ui/app-dialog";

interface RemoveFriendDialogProps {
  open: boolean;
  username: string;
  pending: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

/** Confirmation shown before removing a friend; it cannot be dismissed while the request is in flight. */
export function RemoveFriendDialog({
  open,
  username,
  pending,
  onConfirm,
  onClose,
}: RemoveFriendDialogProps) {
  const { t } = useTranslation();

  return (
    <AppDialog
      open={open}
      onClose={onClose}
      tone="danger"
      icon={<UserMinus />}
      title={t("friends.removeDialog.title", { username })}
      description={
        <span data-testid="remove-friend-description">{t("friends.removeDialog.description")}</span>
      }
      pending={pending}
      primary={{
        label: pending ? t("friends.removeDialog.submitting") : t("friends.removeDialog.confirm"),
        onClick: onConfirm,
        testId: "remove-friend-confirm",
      }}
      secondary={{
        label: t("friends.removeDialog.cancel"),
        onClick: onClose,
        testId: "remove-friend-cancel",
      }}
      testId="remove-friend-dialog"
    />
  );
}
