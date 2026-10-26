import { useTranslation } from "react-i18next";

import { Switch } from "@/shared/components/ui/switch";
import {
  persistAllowNonFriendInvites,
  resolveAllowNonFriendInvites,
} from "@/shared/lib/invitePreference";
import { useAuthStore } from "@/shared/stores/authStore";

import { SidePanel } from "./SidePanel";

/**
 * Self-only profile switch for room invites from players who are not friends, saved optimistically.
 * It is a private preference, so it must never appear on the public profile page.
 */
export function InvitePrivacyPanel() {
  const { t } = useTranslation();
  const allowed = resolveAllowNonFriendInvites(useAuthStore((s) => s.user?.allowNonFriendInvites));

  return (
    <SidePanel
      eyebrow={t("profile.invites.eyebrow")}
      title={t("profile.invites.title")}
      testId="profile-invites"
    >
      <p className="text-ink-mute mb-2.5 text-[12px]">{t("profile.invites.description")}</p>
      <div className="border-border bg-surface-elevated rounded-[10px] border px-3 py-2.5">
        <label className="flex cursor-pointer items-center justify-between gap-3">
          <span className="text-ink text-[13px] font-medium">
            {t("profile.invites.allowNonFriends")}
          </span>
          <Switch
            checked={allowed}
            onCheckedChange={(checked) => void persistAllowNonFriendInvites(checked)}
            data-testid="profile-allow-non-friend-invites"
          />
        </label>
      </div>
    </SidePanel>
  );
}
