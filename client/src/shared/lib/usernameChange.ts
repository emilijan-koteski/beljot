// Client-side mirror of the server's username rules. These are UX-only helpers
// — the Go server (internal/user/validate.go) remains the sole authority for
// validation, uniqueness, and the change cooldown. Keep these values in sync
// with user.UsernameChangeCooldownDays / UsernameMinLength / UsernameMaxLength;
// the project has no shared type generation between Go and TS.
export const USERNAME_CHANGE_COOLDOWN_DAYS = 30;
export const USERNAME_MIN_LENGTH = 3;
export const USERNAME_MAX_LENGTH = 20;
export const USERNAME_REGEX = /^[a-zA-Z0-9_]+$/;

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * The instant the user may next change their username, or null when they have
 * never changed it (no cooldown in effect) or the timestamp is unparseable.
 */
export function usernameChangeAvailableAt(
  usernameChangedAt: string | null | undefined,
): Date | null {
  if (!usernameChangedAt) return null;
  const changedMs = new Date(usernameChangedAt).getTime();
  if (Number.isNaN(changedMs)) return null;
  return new Date(changedMs + USERNAME_CHANGE_COOLDOWN_DAYS * DAY_MS);
}

/** True while the last change is still inside the cooldown window. */
export function isUsernameChangeOnCooldown(
  usernameChangedAt: string | null | undefined,
  now: Date = new Date(),
): boolean {
  const availableAt = usernameChangeAvailableAt(usernameChangedAt);
  return availableAt !== null && now.getTime() < availableAt.getTime();
}

/**
 * Client mirror of server user.ValidateUsername. Returns the matching server
 * error CODE (so the UI can reuse the same i18n messages) or null when valid.
 */
export function validateUsernameClient(raw: string): string | null {
  const trimmed = raw.trim();
  if (trimmed.length < USERNAME_MIN_LENGTH) return "USERNAME_TOO_SHORT";
  if (trimmed.length > USERNAME_MAX_LENGTH) return "USERNAME_TOO_LONG";
  if (!USERNAME_REGEX.test(trimmed)) return "USERNAME_INVALID_CHARS";
  return null;
}
