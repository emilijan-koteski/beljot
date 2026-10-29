import { Users } from "lucide-react";
import { useTranslation } from "react-i18next";

import { FriendsPanel } from "@/features/friends/FriendsPanel";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import { useFriends } from "@/shared/hooks/queries/useFriends";

interface FriendsDialogProps {
  open: boolean;
  onClose: () => void;
}

/** The Friends sheet on phones and tablets; whispering a friend closes it so the chat can open. */
export function FriendsDialog({ open, onClose }: FriendsDialogProps) {
  const { t } = useTranslation();
  const { data: friends } = useFriends();
  const online = friends?.filter((f) => f.online === true).length ?? 0;
  return (
    <AppDialog
      open={open}
      onClose={onClose}
      icon={<Users />}
      title={t("friends.list.heading")}
      description={t("friends.dialog.description", { online, total: friends?.length ?? 0 })}
      primary={{ label: t("friends.dialog.done"), onClick: onClose }}
      testId="friends-dialog"
    >
      <FriendsPanel onWhisper={onClose} />
    </AppDialog>
  );
}
