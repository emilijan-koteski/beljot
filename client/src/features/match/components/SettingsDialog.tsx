import { Globe, Spade, Volume1, Volume2 } from "lucide-react";
import { type ReactNode, useId } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";

import { updatePreferences } from "@/shared/api/profile";
import { useAudioVolume } from "@/shared/audio/useAudioVolume";
import { Segmented } from "@/shared/components/ui/segmented";
import { Slider } from "@/shared/components/ui/slider";
import { Switch } from "@/shared/components/ui/switch";
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
import { ClassicActions } from "./overlay/ClassicActions";
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
 * One audio channel: its switch, named by the label it sits in, over its volume slider.
 * The slider stays outside the label so pressing the track never toggles the switch.
 */
function AudioChannel({
  checked,
  onCheckedChange,
  label,
  switchTestId,
  volumeField,
  volumeLabel,
  volumeTestId,
}: {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  label: string;
  switchTestId: string;
  volumeField: AudioVolumeField;
  volumeLabel: string;
  volumeTestId: string;
}) {
  const { value, onValueChange, onValueCommitted } = useAudioVolume(volumeField);
  return (
    <div className="flex flex-col gap-2">
      <label className="flex cursor-pointer items-center justify-between gap-3">
        <span className="text-ink text-sm font-medium">{label}</span>
        <Switch checked={checked} onCheckedChange={onCheckedChange} data-testid={switchTestId} />
      </label>
      <div
        data-testid={volumeTestId}
        data-disabled={checked ? undefined : ""}
        className="flex items-center gap-3 transition-opacity data-disabled:opacity-50"
      >
        <Volume1 size={14} aria-hidden="true" className="text-brass shrink-0" />
        <Slider
          min={0}
          max={100}
          step={1}
          value={value}
          onValueChange={onValueChange}
          onValueCommitted={onValueCommitted}
          disabled={!checked}
          aria-label={volumeLabel}
          getAriaValueText={(v) => `${v}%`}
          className="data-disabled:opacity-100"
        />
        <span aria-hidden className="text-brass min-w-[4ch] text-right text-xs tabular-nums">
          {value}%
        </span>
      </div>
    </div>
  );
}

/**
 * Brass small-caps heading with an icon over one group of settings.
 */
function SettingSection({
  icon,
  heading,
  headingId,
  children,
}: {
  icon: ReactNode;
  heading: string;
  headingId?: string;
  children: ReactNode;
}) {
  return (
    <section className="flex flex-col gap-3">
      <div
        className="flex items-center gap-2 text-xs uppercase"
        style={{ color: BRASS, letterSpacing: "0.18em" }}
      >
        {icon}
        <span id={headingId}>{heading}</span>
      </div>
      {children}
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
  const soundHeadingId = useId();

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
      <OverlayBackdrop dim="prompt">
        <div
          ref={dialogRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="settings-dialog-title"
          className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            size="dialog"
            title={
              <span id="settings-dialog-title" className="inline-flex items-center gap-2.5">
                {t("match.settings.title")}
              </span>
            }
          >
            <div className="felt-controls flex flex-col gap-5">
              <SettingSection
                icon={<Globe size={14} aria-hidden="true" />}
                heading={t("match.settings.languageHeading")}
              >
                <Segmented
                  value={i18n.language}
                  onValueChange={(lang) => void handleLanguageChange(lang)}
                  options={LANGUAGES.map((lang) => ({ value: lang.code, label: t(lang.labelKey) }))}
                  ariaLabel={t("match.settings.languageHeading")}
                  testId="settings-language-option"
                />
              </SettingSection>

              <SettingSection
                icon={<Spade size={14} aria-hidden="true" />}
                heading={t("match.settings.deckHeading")}
              >
                <Segmented
                  value={deck}
                  onValueChange={(next) => void handleDeckChange(next)}
                  options={DECKS.map((d) => ({ value: d.code, label: t(d.labelKey) }))}
                  ariaLabel={t("match.settings.deckHeading")}
                  testId="settings-deck-option"
                />
              </SettingSection>

              <SettingSection
                icon={<Volume2 size={14} aria-hidden="true" />}
                heading={t("match.settings.soundHeading")}
                headingId={soundHeadingId}
              >
                <div role="group" aria-labelledby={soundHeadingId} className="flex flex-col gap-4">
                  <AudioChannel
                    checked={soundEnabled}
                    onCheckedChange={(checked) =>
                      void persistAudioPreferences({ soundEnabled: checked })
                    }
                    label={t("match.settings.soundEffects")}
                    switchTestId="settings-sound-toggle"
                    volumeField="soundVolume"
                    volumeLabel={t("match.settings.soundVolume")}
                    volumeTestId="settings-sound-volume"
                  />
                  <AudioChannel
                    checked={musicEnabled}
                    onCheckedChange={(checked) =>
                      void persistAudioPreferences({ musicEnabled: checked })
                    }
                    label={t("match.settings.music")}
                    switchTestId="settings-music-toggle"
                    volumeField="musicVolume"
                    volumeLabel={t("match.settings.musicVolume")}
                    volumeTestId="settings-music-volume"
                  />
                </div>
              </SettingSection>
            </div>

            <ClassicActions
              className="mt-5"
              primary={{
                label: t("match.settings.close"),
                onClick: () => onOpenChange(false),
                testId: "settings-dialog-close",
              }}
            />
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );

  return createPortal(dialog, document.body);
}
