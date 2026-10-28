import { Lock, LockOpen } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import { PasswordToggle } from "@/shared/components/PasswordToggle";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import { Field } from "@/shared/components/ui/field";
import { Input } from "@/shared/components/ui/input";
import { Segmented } from "@/shared/components/ui/segmented";
import { useUpdateRoomPrivacyMutation } from "@/shared/hooks/mutations/useRooms";
import { useRoomStore } from "@/shared/stores/roomStore";
import type { Room } from "@/shared/types/apiTypes";

const MIN_ROOM_PASSWORD = 4;
const MAX_ROOM_PASSWORD = 72;

interface RoomPrivacyDialogProps {
  open: boolean;
  room: Room;
  onClose: () => void;
}

/**
 * Owner-only dialog to make a waiting room private (set or change its password) or public again.
 * Updates the local room store from the response so the badge flips immediately.
 */
export function RoomPrivacyDialog({ open, room, onClose }: RoomPrivacyDialogProps) {
  const { t } = useTranslation();
  const mutation = useUpdateRoomPrivacyMutation();
  const setRoom = useRoomStore((s) => s.setRoom);

  const [makePrivate, setMakePrivate] = useState(room.isPrivate);
  const [password, setPassword] = useState("");
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);

  useEffect(() => {
    if (open) {
      setMakePrivate(room.isPrivate);
      setPassword("");
      setErrorKey(null);
      setShowPassword(false);
    }
  }, [open, room.isPrivate]);

  const passwordValid =
    !makePrivate ||
    (password.length >= MIN_ROOM_PASSWORD &&
      new TextEncoder().encode(password).length <= MAX_ROOM_PASSWORD);

  async function handleSubmit() {
    if (mutation.isPending || !passwordValid) return;
    setErrorKey(null);
    try {
      const updated = await mutation.mutateAsync({
        roomId: room.id,
        isPrivate: makePrivate,
        password: makePrivate ? password : undefined,
      });
      setRoom(updated);
      toast.success(makePrivate ? t("room.privacy.toastPrivate") : t("room.privacy.toastPublic"));
      onClose();
    } catch (err) {
      const code = err instanceof FetchError ? err.code : null;
      if (code === "ROOM_PASSWORD_REQUIRED") setErrorKey("room.privacy.errors.passwordRequired");
      else if (code === "ROOM_PASSWORD_TOO_SHORT")
        setErrorKey("room.privacy.errors.passwordTooShort");
      else if (code === "ROOM_PASSWORD_TOO_LONG")
        setErrorKey("room.privacy.errors.passwordTooLong");
      else if (code === "NOT_ROOM_OWNER") setErrorKey("room.privacy.errors.notOwner");
      else if (code === "ROOM_NOT_WAITING") setErrorKey("room.privacy.errors.notWaiting");
      else setErrorKey("room.privacy.errors.unexpected");
    }
  }

  return (
    <AppDialog
      open={open}
      onClose={onClose}
      icon={makePrivate ? <Lock /> : <LockOpen />}
      title={t("room.privacy.dialogTitle")}
      description={t("room.privacy.dialogDescription")}
      error={errorKey !== null ? t(errorKey) : null}
      errorTestId="room-privacy-error"
      pending={mutation.isPending}
      onSubmit={() => void handleSubmit()}
      primary={{
        label: mutation.isPending ? t("room.privacy.saving") : t("room.privacy.save"),
        disabled: !passwordValid,
        testId: "room-privacy-submit",
      }}
      secondary={{
        label: t("room.privacy.cancel"),
        onClick: onClose,
        testId: "room-privacy-cancel",
      }}
      testId="room-privacy-dialog"
    >
      <div className="grid gap-4">
        <Field label={t("room.privacy.modeLabel")}>
          <Segmented
            value={makePrivate ? "private" : "public"}
            onValueChange={(v) => {
              setMakePrivate(v === "private");
              setErrorKey(null);
              if (v === "public") setPassword("");
            }}
            options={[
              {
                value: "public",
                label: t("room.privacy.public"),
                icon: <LockOpen className="size-3.5" />,
              },
              {
                value: "private",
                label: t("room.privacy.private"),
                icon: <Lock className="size-3.5" />,
              },
            ]}
            testId="room-privacy-mode"
            ariaLabel={t("room.privacy.modeLabel")}
          />
        </Field>

        {makePrivate && (
          <Field
            label={t("room.privacy.passwordLabel")}
            htmlFor="room-privacy-password"
            hint={t("room.privacy.passwordHint", { min: MIN_ROOM_PASSWORD })}
            required
          >
            <div className="relative">
              <Input
                id="room-privacy-password"
                type={showPassword ? "text" : "password"}
                autoComplete="new-password"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value.slice(0, MAX_ROOM_PASSWORD));
                  setErrorKey(null);
                }}
                maxLength={MAX_ROOM_PASSWORD}
                data-testid="room-privacy-password-input"
                className="h-11 pr-10"
              />
              <PasswordToggle
                shown={showPassword}
                onToggle={() => setShowPassword(!showPassword)}
                testId="room-privacy-password-toggle"
              />
            </div>
          </Field>
        )}
      </div>
    </AppDialog>
  );
}
