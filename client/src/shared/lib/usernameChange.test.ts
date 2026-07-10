import { describe, expect, it } from "vitest";

import {
  isUsernameChangeOnCooldown,
  usernameChangeAvailableAt,
  validateUsernameClient,
} from "./usernameChange";

describe("usernameChangeAvailableAt", () => {
  it("returns null when never changed", () => {
    expect(usernameChangeAvailableAt(null)).toBeNull();
    expect(usernameChangeAvailableAt(undefined)).toBeNull();
  });

  it("returns null for an unparseable timestamp", () => {
    expect(usernameChangeAvailableAt("not-a-date")).toBeNull();
  });

  it("returns 30 days after the change", () => {
    const changed = "2026-01-01T00:00:00Z";
    const available = usernameChangeAvailableAt(changed);
    expect(available?.toISOString()).toBe("2026-01-31T00:00:00.000Z");
  });
});

describe("isUsernameChangeOnCooldown", () => {
  const changed = "2026-01-01T00:00:00Z";

  it("is true within the 30-day window", () => {
    expect(isUsernameChangeOnCooldown(changed, new Date("2026-01-15T00:00:00Z"))).toBe(true);
  });

  it("is false once the window has elapsed", () => {
    expect(isUsernameChangeOnCooldown(changed, new Date("2026-02-05T00:00:00Z"))).toBe(false);
  });

  it("is false when never changed", () => {
    expect(isUsernameChangeOnCooldown(null, new Date("2026-01-15T00:00:00Z"))).toBe(false);
  });
});

describe("validateUsernameClient", () => {
  it("accepts a valid username", () => {
    expect(validateUsernameClient("valid_name1")).toBeNull();
    expect(validateUsernameClient("  trimmed  ")).toBeNull();
  });

  it("rejects too short / too long / invalid chars with the server codes", () => {
    expect(validateUsernameClient("ab")).toBe("USERNAME_TOO_SHORT");
    expect(validateUsernameClient("a".repeat(21))).toBe("USERNAME_TOO_LONG");
    expect(validateUsernameClient("bad name!")).toBe("USERNAME_INVALID_CHARS");
  });
});
