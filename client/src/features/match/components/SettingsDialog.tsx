import { Globe } from "lucide-react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";

import { updatePreferences } from "@/shared/api/profile";
import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { Z } from "@/shared/lib/zLayers";
import { useAuthStore } from "@/shared/stores/authStore";

import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface SettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// Order: Latin-script entries sorted ASC by native name (English, Hrvatski,
// Srpski) then Cyrillic-script entries sorted ASC (Македонски). Mirrors the
// order used in the lobby nav LanguageSelector so users see the same list
// regardless of where they switch language.
const LANGUAGES = [
  { code: "en", labelKey: "language.en" },
  { code: "hr", labelKey: "language.hr" },
  { code: "sr", labelKey: "language.sr" },
  { code: "mk", labelKey: "language.mk" },
] as const;

const BRASS = "#c9a876";

/**
 * In-game settings dialog. Currently exposes only the UI language; the layout
 * is sectioned ("Language" heading) so future settings (sound, table theme,
 * timer preference, etc.) can drop in without rework.
 *
 * The language change persists to the user's profile via `updatePreferences`,
 * mirroring the lobby's [LanguageSelector] behavior — fire-and-forget so the
 * UI doesn't block on the network round-trip.
 *
 * Renders inside the same classic-felt overlay shell (ClassicPanel +
 * OverlayBackdrop) used by the bidding / belot / surrender / rules prompts so
 * the in-game chrome stays visually consistent. Portaled to document.body so
 * the z-50 backdrop floats above the bidder banner and seat panels.
 */
export function SettingsDialog({ open, onOpenChange }: SettingsDialogProps) {
  const { t, i18n } = useTranslation();
  const dialogRef = useFocusTrap<HTMLDivElement>({ onEscape: () => onOpenChange(false) });

  async function handleLanguageChange(lang: string) {
    if (lang === i18n.language) return;

    await i18n.changeLanguage(lang);

    const user = useAuthStore.getState().user;
    if (!user) return;

    const previousPreference = user.languagePreference;
    useAuthStore.getState().setUser({ ...user, languagePreference: lang });

    try {
      await updatePreferences(user.id, { languagePreference: lang });
    } catch {
      // Revert the optimistic auth-store update so it stays in sync with the
      // server (mirrors LanguageSelector). UI locale stays put.
      const current = useAuthStore.getState().user;
      if (current?.id === user.id) {
        useAuthStore.getState().setUser({ ...current, languagePreference: previousPreference });
      }
    }
  }

  if (!open) return null;

  const dialog = (
    <div className="fixed inset-0" style={{ zIndex: Z.UTIL }} data-testid="settings-dialog">
      <OverlayBackdrop dim={0.5}>
        <div
          ref={dialogRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="settings-dialog-title"
          className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            width={460}
            title={
              <span id="settings-dialog-title" className="inline-flex items-center gap-2.5">
                {t("match.settings.title")}
              </span>
            }
          >
            {/* Language section — sectioned heading so future settings (sound,
                table theme, timer preference) can drop in below without
                shifting this block. */}
            <section className="flex flex-col gap-3">
              <div
                className="flex items-center gap-2 text-xs uppercase"
                style={{
                  color: BRASS,
                  fontFamily: "var(--font-body)",
                  letterSpacing: "0.18em",
                }}
              >
                <Globe size={14} aria-hidden="true" />
                <span>{t("match.settings.languageHeading")}</span>
              </div>

              <div className="flex flex-col gap-2" role="radiogroup">
                {LANGUAGES.map((lang) => {
                  const selected = i18n.language === lang.code;
                  return (
                    <button
                      key={lang.code}
                      type="button"
                      role="radio"
                      aria-checked={selected}
                      onClick={() => handleLanguageChange(lang.code)}
                      data-testid={`settings-language-option-${lang.code}`}
                      className="flex items-center justify-between rounded-lg px-3 py-2 text-left transition-[background,border-color,box-shadow] cursor-pointer"
                      style={{
                        border: selected
                          ? `1px solid ${BRASS}`
                          : "1px solid rgba(201,168,118,0.32)",
                        background: selected
                          ? "linear-gradient(90deg, rgba(80,60,30,0.55), rgba(50,38,20,0.35))"
                          : "linear-gradient(90deg, rgba(20,46,28,0.55), rgba(14,40,24,0.35))",
                        color: "var(--ink-light, #f5f2e8)",
                        boxShadow: selected
                          ? "inset 0 1px 0 rgba(201,168,118,0.22), 0 0 0 1px rgba(201,168,118,0.25)"
                          : "inset 0 1px 0 rgba(201,168,118,0.10)",
                      }}
                    >
                      <span
                        className="font-body text-sm font-medium inline-flex items-center gap-2"
                        style={{
                          fontFamily: "var(--font-body)",
                          letterSpacing: 0.2,
                        }}
                      >
                        <span
                          aria-hidden
                          className="rounded-full"
                          style={{
                            width: 7,
                            height: 7,
                            background: selected ? BRASS : "transparent",
                            border: selected ? "none" : "1px solid rgba(201,168,118,0.45)",
                            boxShadow: selected ? "0 0 6px rgba(201,168,118,0.55)" : "none",
                            flexShrink: 0,
                          }}
                        />
                        {t(lang.labelKey)}
                      </span>
                      {selected && (
                        <span
                          className="text-xs uppercase tracking-wider"
                          style={{ color: BRASS, fontFamily: "var(--font-body)" }}
                          aria-hidden
                        >
                          ✓
                        </span>
                      )}
                    </button>
                  );
                })}
              </div>
            </section>

            <div className="flex justify-end items-center mt-5">
              <ClassicButton
                variant="primary"
                onClick={() => onOpenChange(false)}
                data-testid="settings-dialog-close"
              >
                {t("match.settings.close")}
              </ClassicButton>
            </div>
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );

  return createPortal(dialog, document.body);
}
