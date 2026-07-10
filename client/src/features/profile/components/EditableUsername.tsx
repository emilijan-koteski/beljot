import { Check, Pencil, X } from "lucide-react";
import type { KeyboardEvent } from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import { Button } from "@/shared/components/ui/button";
import { Input } from "@/shared/components/ui/input";
import { useUpdateUsernameMutation } from "@/shared/hooks/mutations/useProfile";
import { formatLocalizedDate } from "@/shared/lib/formatDate";
import {
  isUsernameChangeOnCooldown,
  USERNAME_MAX_LENGTH,
  usernameChangeAvailableAt,
  validateUsernameClient,
} from "@/shared/lib/usernameChange";

type EditableUsernameProps = {
  username: string;
  /** Undefined when there is no authenticated self context — editing is hidden. */
  userId: number | undefined;
  /** When the username was last changed; drives the cooldown gate. */
  usernameChangedAt?: string | null;
};

// Matches the static hero title previously rendered inline by IdentityHero, so
// swapping in the editable version leaves the visual identity unchanged.
const TITLE_CLASS =
  "text-ink font-display m-0 text-[clamp(32px,6vw,48px)] leading-[1.05] font-bold tracking-[-1.2px]";

/**
 * The profile identity title with edit-in-place. Display mode shows the
 * username with a pencil; clicking swaps to an input with confirm/cancel
 * (Enter saves, Escape cancels). Client-side validation mirrors the server for
 * instant feedback, but the server stays authoritative — a rejected change
 * surfaces its error inline. On success the mutation updates both the profile
 * cache and the auth store, so the header refreshes without a reload.
 */
export function EditableUsername({ username, userId, usernameChangedAt }: EditableUsernameProps) {
  const { t } = useTranslation();
  const mutation = useUpdateUsernameMutation(userId ?? 0);
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState(username);
  const [error, setError] = useState<string | null>(null);

  const onCooldown = isUsernameChangeOnCooldown(usernameChangedAt);
  const availableAt = usernameChangeAvailableAt(usernameChangedAt);
  const cooldownHint = availableAt
    ? t("profile.editUsername.cooldownHint", {
        date: formatLocalizedDate(availableAt.toISOString(), t, "long"),
      })
    : "";

  function messageForCode(code: string): string {
    switch (code) {
      case "USERNAME_TOO_SHORT":
        return t("profile.editUsername.errors.tooShort");
      case "USERNAME_TOO_LONG":
        return t("profile.editUsername.errors.tooLong");
      case "USERNAME_INVALID_CHARS":
        return t("profile.editUsername.errors.invalidChars");
      case "USERNAME_TAKEN":
        return t("profile.editUsername.errors.taken");
      case "USERNAME_UNCHANGED":
        return t("profile.editUsername.errors.unchanged");
      case "USERNAME_CHANGE_TOO_SOON":
        // Only show a concrete date when we actually know the last-change time;
        // otherwise a generic message beats fabricating "try again on <today>".
        return availableAt
          ? t("profile.editUsername.errors.tooSoon", {
              date: formatLocalizedDate(availableAt.toISOString(), t, "long"),
            })
          : t("profile.editUsername.errors.tooSoonGeneric");
      default:
        return t("profile.editUsername.errors.generic");
    }
  }

  function startEdit() {
    setDraft(username);
    setError(null);
    setIsEditing(true);
  }

  function cancelEdit() {
    setIsEditing(false);
    setError(null);
  }

  function submit() {
    const trimmed = draft.trim();
    // Submitting the unchanged name is a no-op — just close, never call the API
    // (which would reject it and, worse, could be read as consuming a change).
    if (trimmed === username) {
      cancelEdit();
      return;
    }
    const code = validateUsernameClient(trimmed);
    if (code) {
      setError(messageForCode(code));
      return;
    }
    mutation.mutate(
      { username: trimmed },
      {
        onSuccess: () => {
          setIsEditing(false);
          setError(null);
          toast.success(t("profile.editUsername.success"));
        },
        onError: (err) => {
          setError(messageForCode(err instanceof FetchError ? err.code : ""));
        },
      },
    );
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter") {
      e.preventDefault();
      submit();
    } else if (e.key === "Escape") {
      e.preventDefault();
      cancelEdit();
    }
  }

  if (isEditing) {
    const trimmed = draft.trim();
    const saveDisabled = mutation.isPending || trimmed.length === 0 || trimmed === username;
    return (
      <div className="flex flex-col gap-1.5">
        <div className="flex items-center gap-2">
          <Input
            autoFocus
            value={draft}
            maxLength={USERNAME_MAX_LENGTH}
            onChange={(e) => {
              setDraft(e.target.value);
              if (error) setError(null);
            }}
            onFocus={(e) => e.currentTarget.select()}
            onKeyDown={onKeyDown}
            disabled={mutation.isPending}
            aria-label={t("profile.editUsername.inputLabel")}
            aria-invalid={error ? true : undefined}
            className="font-display h-auto max-w-60 py-1 text-2xl font-bold"
            data-testid="profile-username-input"
          />
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={submit}
            disabled={saveDisabled}
            aria-label={t("profile.editUsername.save")}
            data-testid="profile-username-save"
          >
            <Check className="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={cancelEdit}
            disabled={mutation.isPending}
            aria-label={t("profile.editUsername.cancel")}
            data-testid="profile-username-cancel"
          >
            <X className="size-4" />
          </Button>
        </div>
        {error && (
          <p
            className="text-destructive text-xs font-medium"
            aria-live="polite"
            data-testid="profile-username-error"
          >
            {error}
          </p>
        )}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-2">
        <h1 className={TITLE_CLASS} data-testid="profile-username">
          {username}
        </h1>
        {userId !== undefined && (
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={startEdit}
            disabled={onCooldown}
            title={onCooldown ? cooldownHint : t("profile.editUsername.edit")}
            aria-label={t("profile.editUsername.edit")}
            data-testid="profile-edit-username-button"
          >
            <Pencil className="size-4" />
          </Button>
        )}
      </div>
      {userId !== undefined && onCooldown && cooldownHint && (
        <p className="text-ink-mute text-[12px]" data-testid="profile-username-cooldown">
          {cooldownHint}
        </p>
      )}
    </div>
  );
}
