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
  /** The subject's username, shown so the viewer knows exactly who is removed. */
  username: string;
  /** True while the remove request is in flight (locks dismissal + confirm). */
  pending: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

/**
 * Confirmation shown before removing a friend from the public profile. Purely a
 * confirm step — the destructive part is that the friendship row is
 * hard-deleted, though either side can send a fresh request later. Modeled on
 * UnlinkAccountDialog: not dismissable while the request is in flight, so an
 * in-flight remove can't be silently abandoned.
 */
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
