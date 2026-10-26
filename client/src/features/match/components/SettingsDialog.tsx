import { Globe, Spade, Volume1, Volume2 } from "lucide-react";
import { type ReactNode, useId } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";

import { updatePreferences } from "@/shared/api/profile";
import { useAudioVolume } from "@/shared/audio/useAudioVolume";
import { Slider } from "@/shared/components/ui/slider";
import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import {
  type AudioVolumeField,
  persistAudioPreferences,
  resolveAudioEnabled,
} from "@/shared/lib/audioPreference";
import { persistCardDeck } from "@/shared/lib/cardDeckPreference";
import { Z } from "@/shared/lib/zLayers";
import { useAuthStore } from "@/shared/stores/authStore";

import type { CardDeck } from "../lib/cardFace";
import { resolveCardDeck } from "../lib/cardFace";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface SettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const LANGUAGES = [
  { code: "en", labelKey: "language.en" },
  { code: "hr", labelKey: "language.hr" },
  { code: "sr", labelKey: "language.sr" },
  { code: "mk", labelKey: "language.mk" },
] as const;

const DECKS = [
  { code: "french", labelKey: "cardDeck.french" },
  { code: "croatian", labelKey: "cardDeck.croatian" },
] as const satisfies ReadonlyArray<{ code: CardDeck; labelKey: string }>;

const BRASS = "var(--brass)";

/**
 * One brass radio row. Shared by the Language and Card Deck sections rather than
 * copied into each, so the two pickers cannot drift apart visually and the next
 * section added below inherits the same row for free.
 */
function SettingRow({
  selected,
  onSelect,
  testId,
  children,
}: {
  selected: boolean;
  onSelect: () => void;
  testId: string;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={onSelect}
      data-testid={testId}
      className="flex items-center justify-between rounded-lg px-3 py-2 text-left transition-[background,border-color,box-shadow] cursor-pointer"
      style={{
        border: selected ? `1px solid ${BRASS}` : "1px solid var(--brass-40)",
        background: selected
          ? "linear-gradient(90deg, rgba(80,60,30,0.55), rgba(50,38,20,0.35))"
          : "linear-gradient(90deg, rgba(20,46,28,0.55), rgba(14,40,24,0.35))",
        color: "var(--ink-light)",
        boxShadow: selected
          ? "inset 0 1px 0 var(--brass-25), 0 0 0 1px var(--brass-25)"
          : "inset 0 1px 0 var(--brass-10)",
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
            border: selected ? "none" : "1px solid var(--brass-40)",
            boxShadow: selected ? "0 0 6px var(--brass-55)" : "none",
            flexShrink: 0,
          }}
        />
        {children}
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
}

/**
 * Brass on/off switch row, the switch counterpart of `SettingRow`.
 */
function SettingSwitchRow({
  checked,
  onToggle,
  testId,
  children,
}: {
  checked: boolean;
  onToggle: () => void;
  testId: string;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={onToggle}
      data-testid={testId}
      className="flex items-center justify-between rounded-lg px-3 py-2 text-left transition-[background,border-color,box-shadow] cursor-pointer"
      style={{
        border: checked ? `1px solid ${BRASS}` : "1px solid var(--brass-40)",
        background: checked
          ? "linear-gradient(90deg, rgba(80,60,30,0.55), rgba(50,38,20,0.35))"
          : "linear-gradient(90deg, rgba(20,46,28,0.55), rgba(14,40,24,0.35))",
        color: "var(--ink-light)",
        boxShadow: checked
          ? "inset 0 1px 0 var(--brass-25), 0 0 0 1px var(--brass-25)"
          : "inset 0 1px 0 var(--brass-10)",
      }}
    >
      <span
        className="font-body text-sm font-medium"
        style={{
          fontFamily: "var(--font-body)",
          letterSpacing: 0.2,
        }}
      >
        {children}
      </span>
      <span
        aria-hidden
        className="relative inline-flex shrink-0 items-center rounded-full"
        style={{
          width: 34,
          height: 18,
          background: checked ? "var(--brass-85)" : "rgba(0,0,0,0.3)",
          border: checked ? `1px solid ${BRASS}` : "1px solid var(--brass-40)",
          boxShadow: checked ? "0 0 6px var(--brass-40)" : "none",
        }}
      >
        <span
          className="absolute rounded-full motion-safe:transition-transform"
          style={{
            left: 2,
            width: 12,
            height: 12,
            background: checked ? "#1d3a26" : "var(--brass-55)",
            transform: checked ? "translateX(16px)" : "none",
          }}
        />
      </span>
    </button>
  );
}

/**
 * Brass volume slider row under its channel switch, disabled while that switch is off.
 */
function SettingVolumeRow({
  field,
  label,
  disabled,
  testId,
}: {
  field: AudioVolumeField;
  label: string;
  disabled: boolean;
  testId: string;
}) {
  const { value, onValueChange, onValueCommitted } = useAudioVolume(field);
  return (
    <div
      data-testid={testId}
      data-disabled={disabled ? "" : undefined}
      className="flex items-center gap-3 px-3 py-1 transition-opacity data-disabled:opacity-45"
    >
      <Volume1 size={14} aria-hidden="true" style={{ color: BRASS, flexShrink: 0 }} />
      <Slider
        min={0}
        max={100}
        step={1}
        value={value}
        onValueChange={onValueChange}
        onValueCommitted={onValueCommitted}
        disabled={disabled}
        aria-label={label}
        getAriaValueText={(v) => `${v}%`}
        className="data-disabled:opacity-100"
        trackClassName="bg-[rgba(0,0,0,0.3)] shadow-[inset_0_0_0_1px_var(--brass-40)]"
        indicatorClassName="bg-brass/85"
        thumbClassName="border-[#c9a876] bg-[#1d3a26] shadow-[0_0_6px_var(--brass-40)] has-[:focus-visible]:ring-brass/55"
      />
      <span
        aria-hidden
        className="text-xs tabular-nums"
        style={{
          color: BRASS,
          fontFamily: "var(--font-body)",
          minWidth: "4ch",
          textAlign: "right",
        }}
      >
        {value}%
      </span>
    </div>
  );
}

/**
 * Brass eyebrow heading over a group of setting rows, a radiogroup by default.
 * Sections of independent switches pass `role="group"`.
 */
function SettingSection({
  icon,
  heading,
  role = "radiogroup",
  children,
}: {
  icon: ReactNode;
  heading: string;
  role?: "radiogroup" | "group";
  children: ReactNode;
}) {
  const headingId = useId();
  return (
    <section className="flex flex-col gap-3">
      <div
        className="flex items-center gap-2 text-xs uppercase"
        style={{
          color: BRASS,
          fontFamily: "var(--font-body)",
          letterSpacing: "0.18em",
        }}
      >
        {icon}
        <span id={headingId}>{heading}</span>
      </div>

      <div className="flex flex-col gap-2" role={role} aria-labelledby={headingId}>
        {children}
      </div>
    </section>
  );
}

/**
 * In-game settings dialog for language, card deck and audio, persisted via `updatePreferences`.
 * Writes are optimistic with a silent revert on failure, so the table re-skins instantly without interrupting play.
 */
export function SettingsDialog({ open, onOpenChange }: SettingsDialogProps) {
  const { t, i18n } = useTranslation();
  const dialogRef = useFocusTrap<HTMLDivElement>({ onEscape: () => onOpenChange(false) });
  const deck = resolveCardDeck(useAuthStore((s) => s.user?.cardDeckPreference));
  const soundEnabled = resolveAudioEnabled(useAuthStore((s) => s.user?.soundEnabled));
  const musicEnabled = resolveAudioEnabled(useAuthStore((s) => s.user?.musicEnabled));

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
      const current = useAuthStore.getState().user;
      if (current?.id === user.id) {
        useAuthStore.getState().setUser({ ...current, languagePreference: previousPreference });
      }
    }
  }

  async function handleDeckChange(next: CardDeck) {
    if (next === deck) return;
    await persistCardDeck(next, deck);
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
            <div className="flex flex-col gap-5">
              <SettingSection
                icon={<Globe size={14} aria-hidden="true" />}
                heading={t("match.settings.languageHeading")}
              >
                {LANGUAGES.map((lang) => (
                  <SettingRow
                    key={lang.code}
                    selected={i18n.language === lang.code}
                    onSelect={() => handleLanguageChange(lang.code)}
                    testId={`settings-language-option-${lang.code}`}
                  >
                    {t(lang.labelKey)}
                  </SettingRow>
                ))}
              </SettingSection>

              <SettingSection
                icon={<Spade size={14} aria-hidden="true" />}
                heading={t("match.settings.deckHeading")}
              >
                {DECKS.map((d) => (
                  <SettingRow
                    key={d.code}
                    selected={deck === d.code}
                    onSelect={() => handleDeckChange(d.code)}
                    testId={`settings-deck-option-${d.code}`}
                  >
                    {t(d.labelKey)}
                  </SettingRow>
                ))}
              </SettingSection>

              <SettingSection
                icon={<Volume2 size={14} aria-hidden="true" />}
                heading={t("match.settings.soundHeading")}
                role="group"
              >
                <SettingSwitchRow
                  checked={soundEnabled}
                  onToggle={() => void persistAudioPreferences({ soundEnabled: !soundEnabled })}
                  testId="settings-sound-toggle"
                >
                  {t("match.settings.soundEffects")}
                </SettingSwitchRow>
                <SettingVolumeRow
                  field="soundVolume"
                  label={t("match.settings.soundVolume")}
                  disabled={!soundEnabled}
                  testId="settings-sound-volume"
                />
                <SettingSwitchRow
                  checked={musicEnabled}
                  onToggle={() => void persistAudioPreferences({ musicEnabled: !musicEnabled })}
                  testId="settings-music-toggle"
                >
                  {t("match.settings.music")}
                </SettingSwitchRow>
                <SettingVolumeRow
                  field="musicVolume"
                  label={t("match.settings.musicVolume")}
                  disabled={!musicEnabled}
                  testId="settings-music-volume"
                />
              </SettingSection>
            </div>

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
