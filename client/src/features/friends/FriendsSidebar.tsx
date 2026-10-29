import { Users } from "lucide-react";
import { useTranslation } from "react-i18next";

import { FriendsPanel } from "@/features/friends/FriendsPanel";
import { Panel } from "@/shared/components/ui/panel";
import { useFriends } from "@/shared/hooks/queries/useFriends";

/** The desktop Friends sidebar: the same panel as the phone sheet, always open. */
export function FriendsSidebar() {
  const { t } = useTranslation();
  const { data: friends } = useFriends();
  const online = friends?.filter((f) => f.online === true).length ?? 0;
  return (
    <Panel
      icon={<Users aria-hidden="true" />}
      title={t("friends.list.heading")}
      aside={<span className="text-accent">{t("friends.onlineCount", { count: online })}</span>}
      testId="friends-sidebar"
    >
      <FriendsPanel />
    </Panel>
  );
}
