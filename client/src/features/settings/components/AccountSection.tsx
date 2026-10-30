import { Camera, UserRound } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Avatar } from "@/shared/components/ui/avatar";
import { useProfileQuery } from "@/shared/hooks/queries/useProfile";
import { useAuthStore } from "@/shared/stores/authStore";

import { AvatarDialog } from "./AvatarDialog";
import { EditableUsername } from "./EditableUsername";
import { SettingsGroup } from "./SettingsGroup";

/**
 * The Account group of Settings: picture, name and email in one block, as on the profile header.
 * The camera button opens the picture dialog and the pencil edits the name in place.
 */
export function AccountSection() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const { data: profile } = useProfileQuery(user?.id);
  const [pictureOpen, setPictureOpen] = useState(false);

  if (!user) return null;
  const username = profile?.username ?? user.username;
  const avatarUrl = profile ? (profile.avatarLargeUrl ?? null) : (user.avatarUrl ?? null);

  return (
    <SettingsGroup
      id="account"
      title={t("settings.sections.account")}
      icon={<UserRound />}
      testId="settings-account"
    >
      <div className="flex items-center gap-4 px-4 py-4">
        <div className="relative flex shrink-0">
          <Avatar name={username} avatarUrl={avatarUrl} eager size={72} halo="profile" />
          <button
            type="button"
            onClick={() => setPictureOpen(true)}
            aria-label={t("profile.avatar.edit")}
            title={t("profile.avatar.edit")}
            className="bg-surface border-border text-ink hover:bg-surface-sunken focus-visible:ring-ring/70 absolute -right-1 -bottom-1 inline-flex size-7 cursor-pointer items-center justify-center rounded-full border shadow-sm transition-colors focus-visible:ring-3 focus-visible:outline-none"
            data-testid="profile-avatar-edit"
          >
            <Camera className="size-3.5" aria-hidden="true" />
          </button>
          <AvatarDialog
            open={pictureOpen}
            onClose={() => setPictureOpen(false)}
            userId={user.id}
            username={username}
            avatarUrl={avatarUrl}
          />
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <EditableUsername
            username={username}
            userId={user.id}
            usernameChangedAt={profile?.usernameChangedAt}
          />
          <span className="text-ink-mute truncate text-sm" data-testid="settings-email">
            {user.email}
          </span>
        </div>
      </div>
    </SettingsGroup>
  );
}
