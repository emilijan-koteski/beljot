import { Eye, EyeOff, Lock, LockOpen } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/ui/dialog";
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
 * Story 9.6 (AC6): owner-only control to make a waiting room private (set/change
 * the password) or revert it to public. Changing privacy never ejects seated
 * players — the gate is join-time only. On success the server broadcasts
 * system:room_updated; we also update the local room store from the response so
 * the badge flips immediately.
 */
export function RoomPrivacyDialog({ open, room, onClose }: RoomPrivacyDialogProps) {
  const { t } = useTranslation();
  const mutation = useUpdateRoomPrivacyMutation();
  const setRoom = useRoomStore((s) => s.setRoom);

  const [makePrivate, setMakePrivate] = useState(room.isPrivate);
  const [password, setPassword] = useState("");
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);

  // Re-seed from the current room state each time the dialog opens.
  useEffect(() => {
    if (open) {
      setMakePrivate(room.isPrivate);
      setPassword("");
      setErrorKey(null);
      setShowPassword(false);
    }
  }, [open, room.isPrivate]);

  // MIN is per-character (matches the server's rune count); MAX is the bcrypt
  // byte limit, so measure UTF-8 bytes — not UTF-16 .length — to agree with the
  // server and never accept a multibyte password it would reject as too long.
  const passwordValid =
    !makePrivate ||
    (password.length >= MIN_ROOM_PASSWORD &&
      new TextEncoder().encode(password).length <= MAX_ROOM_PASSWORD);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
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
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent showCloseButton={false} data-testid="room-privacy-dialog">
        <form onSubmit={handleSubmit} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>{t("room.privacy.dialogTitle")}</DialogTitle>
            <DialogDescription>{t("room.privacy.dialogDescription")}</DialogDescription>
          </DialogHeader>

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
                  placeholder={t("room.privacy.passwordPlaceholder")}
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value.slice(0, MAX_ROOM_PASSWORD));
                    setErrorKey(null);
                  }}
                  maxLength={MAX_ROOM_PASSWORD}
                  data-testid="room-privacy-password-input"
                  className="h-11 pr-10"
                />
                <button
                  type="button"
                  tabIndex={-1}
                  className="text-ink-mute hover:text-ink absolute top-1/2 right-2.5 -translate-y-1/2 p-1.5"
                  onClick={() => setShowPassword(!showPassword)}
                  data-testid="room-privacy-password-toggle"
                  aria-label={showPassword ? "Hide password" : "Show password"}
                >
                  {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                </button>
              </div>
            </Field>
          )}

          {errorKey !== null && (
            <p role="alert" data-testid="room-privacy-error" className="text-destructive text-sm">
              {t(errorKey)}
            </p>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={onClose}
              data-testid="room-privacy-cancel"
            >
              {t("room.privacy.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={mutation.isPending || !passwordValid}
              data-testid="room-privacy-submit"
            >
              {mutation.isPending ? t("room.privacy.saving") : t("room.privacy.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
