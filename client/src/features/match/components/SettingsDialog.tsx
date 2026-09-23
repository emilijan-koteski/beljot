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

// Order: French first — it is the default, and the deck every account that did
// not pick otherwise is already looking at.
const DECKS = [
  { code: "french", labelKey: "cardDeck.french" },
  { code: "croatian", labelKey: "cardDeck.croatian" },
] as const satisfies ReadonlyArray<{ code: CardDeck; labelKey: string }>;

const BRASS = "#c9a876";

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
        border: selected ? `1px solid ${BRASS}` : "1px solid rgba(201,168,118,0.32)",
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
 * One brass on/off row — the switch counterpart of `SettingRow`, same frame, so
 * a section of switches sits flush with the radio sections above it. The row's
 * text is its accessible name; the track/thumb is decoration for
 * `aria-checked`.
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
        border: checked ? `1px solid ${BRASS}` : "1px solid rgba(201,168,118,0.32)",
        background: checked
          ? "linear-gradient(90deg, rgba(80,60,30,0.55), rgba(50,38,20,0.35))"
          : "linear-gradient(90deg, rgba(20,46,28,0.55), rgba(14,40,24,0.35))",
        color: "var(--ink-light, #f5f2e8)",
        boxShadow: checked
          ? "inset 0 1px 0 rgba(201,168,118,0.22), 0 0 0 1px rgba(201,168,118,0.25)"
          : "inset 0 1px 0 rgba(201,168,118,0.10)",
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
          background: checked ? "rgba(201,168,118,0.85)" : "rgba(0,0,0,0.3)",
          border: checked ? `1px solid ${BRASS}` : "1px solid rgba(201,168,118,0.45)",
          boxShadow: checked ? "0 0 6px rgba(201,168,118,0.45)" : "none",
        }}
      >
        <span
          className="absolute rounded-full motion-safe:transition-transform"
          style={{
            left: 2,
            width: 12,
            height: 12,
            background: checked ? "#1d3a26" : "rgba(201,168,118,0.6)",
            transform: checked ? "translateX(16px)" : "none",
          }}
        />
      </span>
    </button>
  );
}

/**
 * One brass volume row, sitting under its channel's switch. The slider moves
 * the level live and persists it on release (see `useAudioVolume`); it is
 * disabled — value kept — while the switch above it is off. The accessible
 * name comes from `label`; the `N%` readout is its visible counterpart.
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
        trackClassName="bg-[rgba(0,0,0,0.3)] shadow-[inset_0_0_0_1px_rgba(201,168,118,0.45)]"
        indicatorClassName="bg-[rgba(201,168,118,0.85)]"
        thumbClassName="border-[#c9a876] bg-[#1d3a26] shadow-[0_0_6px_rgba(201,168,118,0.45)] has-[:focus-visible]:ring-[rgba(201,168,118,0.5)]"
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
 * Brass eyebrow + a group of rows — one per settings group. A radiogroup by
 * default (language, deck); a section of independent switches passes
 * `role="group"`, since its rows are not mutually exclusive. Either way the
 * heading names the group, via `aria-labelledby`.
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
 * In-game settings dialog. Exposes the UI language, the card deck, and the two
 * audio switches each with its volume slider; the layout is sectioned so
 * future settings (table theme, timer preference, etc.) can drop in without
 * rework.
 *
 * Every change persists to the user's profile via `updatePreferences`, mirroring
 * the lobby's [LanguageSelector] behavior — optimistic store write, silent
 * revert on failure, no reload and no interruption to play. The deck is read
 * straight back off the auth store, so every mounted `PlayingCard` re-skins the
 * moment the optimistic write lands, mid-hand included; the audio engine reads
 * the same store, so a switch takes effect on the very next sound.
 *
 * Renders inside the same classic-felt overlay shell (ClassicPanel +
 * OverlayBackdrop) used by the bidding / belot / surrender / rules prompts so
 * the in-game chrome stays visually consistent. Portaled to document.body so
 * the z-50 backdrop floats above the bidder banner and seat panels.
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
      // Revert the optimistic auth-store update so it stays in sync with the
      // server (mirrors LanguageSelector). UI locale stays put.
      const current = useAuthStore.getState().user;
      if (current?.id === user.id) {
        useAuthStore.getState().setUser({ ...current, languagePreference: previousPreference });
      }
    }
  }

  /**
   * Persist the deck. Shares one latest-wins helper with the profile panel, so
   * two fast toggles cannot leave store and server disagreeing — see
   * `persistCardDeck`. Deliberately NOT merged with the language handler above:
   * they write different fields, only language has an i18n switch to sequence
   * first, and the language path's identical race is pre-existing and deferred.
   */
  async function handleDeckChange(next: CardDeck) {
    if (next === deck) return;
    // `deck` is the RESOLVED value, so a rollback cannot reinstate an
    // unrecognised string that would render as a missing asset.
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
            {/* One sectioned group per setting, so the next one (table theme,
                timer preference) drops in below without shifting these. */}
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

              {/* Card deck — purely visual, and applied without leaving the
                  match: the store write re-renders every card in place. */}
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

              {/* Sound — two independent switches, each writing only its own
                  field through the shared latest-wins helper (the HUD mute and
                  the profile panel write the same fields), each with its
                  channel's volume slider underneath. */}
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
