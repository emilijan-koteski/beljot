import { UserMinus } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/ui/dialog";

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
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !pending) onClose();
      }}
    >
      <DialogContent showCloseButton={false} data-testid="remove-friend-dialog">
        <DialogHeader>
          <div className="flex items-center gap-3">
            <div className="bg-brass-soft border-brass/40 text-brass-deep flex size-11 shrink-0 items-center justify-center rounded-xl border">
              <UserMinus className="size-5" />
            </div>
            <DialogTitle>{t("friends.removeDialog.title")}</DialogTitle>
          </div>
          <DialogDescription data-testid="remove-friend-description">
            {t("friends.removeDialog.description", { username })}
          </DialogDescription>
        </DialogHeader>

        <DialogFooter>
          <Button
            type="button"
            variant="destructive"
            disabled={pending}
            onClick={onConfirm}
            data-testid="remove-friend-confirm"
          >
            {pending ? t("friends.removeDialog.submitting") : t("friends.removeDialog.confirm")}
          </Button>
          <Button
            type="button"
            variant="ghost"
            disabled={pending}
            onClick={onClose}
            data-testid="remove-friend-cancel"
          >
            {t("friends.removeDialog.cancel")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
