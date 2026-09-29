import { ImageUp, Trash2 } from "lucide-react";
import type { ChangeEvent } from "react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import { Avatar } from "@/shared/components/ui/avatar";
import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import {
  useRemoveAvatarMutation,
  useUploadAvatarMutation,
} from "@/shared/hooks/mutations/useProfile";

/** The server's file cap, mirrored so an oversized pick never leaves the device. */
const AVATAR_MAX_BYTES = 2_097_152;

/** The picker's filter and the pre-send type check (the server re-sniffs anyway). */
const ACCEPTED_TYPES = ["image/jpeg", "image/png", "image/webp"];

const PREVIEW_SIZE = 96;

type AvatarDialogProps = {
  open: boolean;
  onClose: () => void;
  userId: number;
  username: string;
  /** The current 256 px avatar, or null when the player has none. */
  avatarUrl: string | null;
};

type TFn = ReturnType<typeof useTranslation>["t"];

/**
 * Maps a failed upload or removal to its copy. By status first, because that
 * is the category; the two 503 causes are told apart by code, since "try
 * again in a moment" is wrong advice when storage is simply not configured.
 */
function errorMessage(t: TFn, err: unknown): string {
  if (!(err instanceof FetchError)) return t("profile.avatar.errors.generic");
  switch (err.status) {
    case 400:
      return err.code === "AVATAR_TOO_SMALL"
        ? t("profile.avatar.errors.tooSmall")
        : t("profile.avatar.errors.generic");
    case 413:
      return t("profile.avatar.errors.tooLarge");
    case 415:
      return t("profile.avatar.errors.unsupported");
    case 429:
      return t("profile.avatar.errors.rateLimited");
    case 503:
      return err.code === "AVATAR_STORAGE_UNAVAILABLE"
        ? t("profile.avatar.errors.unavailable")
        : t("profile.avatar.errors.busy");
    default:
      return t("profile.avatar.errors.generic");
  }
}

/**
 * Upload, replace or remove the signed-in player's avatar. The picked file is
 * previewed locally and sent as-is; the server decodes, crops and resizes it.
 * The two checks the server would reject for certain, size and type, run here
 * first so they never cost a request, and answer inline under the picker. A
 * failure the server reports (including the 503 of a development server
 * without storage) is a toast, and the dialog stays open for another try.
 * Like the other pending-guarded dialogs, it cannot be dismissed while a
 * request is in flight.
 */
export function AvatarDialog({ open, onClose, userId, username, avatarUrl }: AvatarDialogProps) {
  const { t } = useTranslation();
  const upload = useUploadAvatarMutation(userId);
  const remove = useRemoveAvatarMutation(userId);
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);

  const pending = upload.isPending || remove.isPending;

  // Each preview URL pins the picked file in memory until revoked: release it
  // whenever it is replaced, cleared on close, or the dialog unmounts.
  useEffect(() => {
    if (previewUrl === null) return;
    return () => URL.revokeObjectURL(previewUrl);
  }, [previewUrl]);

  function reset() {
    setFile(null);
    setPreviewUrl(null);
    setError(null);
    setProgress(0);
  }

  function close() {
    if (pending) return;
    reset();
    onClose();
  }

  function onPick(e: ChangeEvent<HTMLInputElement>) {
    const picked = e.target.files?.[0] ?? null;
    // Clear the input so picking the same file again still fires onChange.
    e.target.value = "";
    if (!picked) return;
    setError(null);
    if (picked.size > AVATAR_MAX_BYTES) {
      setFile(null);
      setPreviewUrl(null);
      setError(t("profile.avatar.errors.tooLarge"));
      return;
    }
    // An empty type means the browser could not tell; the server sniffs the
    // bytes either way, so only a known-wrong type is stopped here.
    if (picked.type !== "" && !ACCEPTED_TYPES.includes(picked.type)) {
      setFile(null);
      setPreviewUrl(null);
      setError(t("profile.avatar.errors.unsupported"));
      return;
    }
    setFile(picked);
    setPreviewUrl(URL.createObjectURL(picked));
  }

  function onUpload() {
    if (!file || pending) return;
    setError(null);
    setProgress(0);
    upload.mutate(
      { file, onProgress: setProgress },
      {
        onSuccess: () => {
          toast.success(t("profile.avatar.success"));
          reset();
          onClose();
        },
        onError: (err) => {
          setProgress(0);
          toast.error(errorMessage(t, err));
        },
      },
    );
  }

  function onRemove() {
    if (pending) return;
    setError(null);
    remove.mutate(undefined, {
      onSuccess: () => {
        toast.success(t("profile.avatar.removed"));
        reset();
        onClose();
      },
      onError: (err) => toast.error(errorMessage(t, err)),
    });
  }

  const progressPct = Math.round(Math.min(1, Math.max(0, progress)) * 100);

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) close();
      }}
    >
      <DialogContent showCloseButton={false} data-testid="avatar-dialog">
        <DialogHeader>
          <DialogTitle>{t("profile.avatar.title")}</DialogTitle>
          <DialogDescription>{t("profile.avatar.hint")}</DialogDescription>
        </DialogHeader>

        <div className="flex flex-col items-center gap-3">
          {previewUrl ? (
            <img
              src={previewUrl}
              alt={t("profile.avatar.previewAlt")}
              width={PREVIEW_SIZE}
              height={PREVIEW_SIZE}
              decoding="async"
              className="rounded-full object-cover"
              style={{ width: PREVIEW_SIZE, height: PREVIEW_SIZE }}
              data-testid="avatar-dialog-preview"
            />
          ) : (
            <Avatar name={username} avatarUrl={avatarUrl} eager size={PREVIEW_SIZE} />
          )}

          <input
            ref={inputRef}
            type="file"
            accept={ACCEPTED_TYPES.join(",")}
            className="sr-only"
            tabIndex={-1}
            onChange={onPick}
            disabled={pending}
            data-testid="avatar-dialog-input"
          />
          <Button
            type="button"
            variant="outline"
            onClick={() => inputRef.current?.click()}
            disabled={pending}
            data-testid="avatar-dialog-pick"
          >
            <ImageUp aria-hidden="true" />
            {t("profile.avatar.pick")}
          </Button>

          {upload.isPending && (
            <div
              className="bg-surface-sunken h-1.5 w-full overflow-hidden rounded-full"
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={progressPct}
              aria-label={t("profile.avatar.progressLabel")}
              data-testid="avatar-dialog-progress"
            >
              <div
                className="bg-accent h-full rounded-full transition-[width] duration-200 ease-out"
                style={{ width: `${progressPct}%` }}
              />
            </div>
          )}

          {/* The live region stays mounted and only its text changes: a region
              that appears together with its message is often not announced. */}
          <div aria-live="polite" className="text-center" data-testid="avatar-dialog-live">
            {error && (
              <p className="text-destructive text-xs font-medium" data-testid="avatar-dialog-error">
                {error}
              </p>
            )}
          </div>
        </div>

        <DialogFooter>
          {avatarUrl !== null && (
            <Button
              type="button"
              variant="destructive"
              className="sm:mr-auto"
              onClick={onRemove}
              disabled={pending}
              data-testid="avatar-dialog-remove"
            >
              <Trash2 aria-hidden="true" />
              {remove.isPending ? t("profile.avatar.removing") : t("profile.avatar.remove")}
            </Button>
          )}
          <Button
            type="button"
            variant="ghost"
            onClick={close}
            disabled={pending}
            data-testid="avatar-dialog-cancel"
          >
            {t("profile.avatar.cancel")}
          </Button>
          <Button
            type="button"
            onClick={onUpload}
            disabled={file === null || pending}
            data-testid="avatar-dialog-upload"
          >
            {upload.isPending ? t("profile.avatar.uploading") : t("profile.avatar.upload")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
