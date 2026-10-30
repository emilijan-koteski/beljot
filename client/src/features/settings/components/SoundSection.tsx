import { Volume2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useAudioVolume } from "@/shared/audio/useAudioVolume";
import { Slider } from "@/shared/components/ui/slider";
import { Switch } from "@/shared/components/ui/switch";
import {
  type AudioSwitchField,
  type AudioVolumeField,
  persistAudioPreferences,
  resolveAudioEnabled,
} from "@/shared/lib/audioPreference";
import { useAuthStore } from "@/shared/stores/authStore";

import { SettingsGroup, SettingsRow } from "./SettingsGroup";

const ROWS = [
  {
    field: "soundEnabled",
    labelKey: "profile.audio.sound",
    testId: "profile-sound-toggle",
    volume: {
      field: "soundVolume",
      labelKey: "profile.audio.soundVolume",
      testId: "profile-sound-volume",
    },
  },
  {
    field: "musicEnabled",
    labelKey: "profile.audio.music",
    testId: "profile-music-toggle",
    volume: {
      field: "musicVolume",
      labelKey: "profile.audio.musicVolume",
      testId: "profile-music-volume",
    },
  },
] as const satisfies ReadonlyArray<{
  field: AudioSwitchField;
  labelKey: string;
  testId: string;
  volume: { field: AudioVolumeField; labelKey: string; testId: string };
}>;

/**
 * One channel's volume slider, kept outside the switch's `<label>` so pressing the track
 * never toggles the switch. Disabled, with its value kept, while the switch is off.
 */
function VolumeRow({
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
      className="flex items-center gap-3 transition-opacity data-disabled:opacity-50"
    >
      <Slider
        className="flex-1 data-disabled:opacity-100"
        min={0}
        max={100}
        step={1}
        value={value}
        onValueChange={onValueChange}
        onValueCommitted={onValueCommitted}
        disabled={disabled}
        aria-label={label}
        getAriaValueText={(v) => `${v}%`}
      />
      <span
        aria-hidden
        className="text-ink-mute min-w-[4ch] text-right font-mono text-2xs tabular-nums"
      >
        {value}%
      </span>
    </div>
  );
}

/**
 * The Sound group of Settings: a switch and a volume slider for sound effects and for music.
 * Writes go through `persistAudioPreferences`, so they stay ordered with in-match changes.
 */
export function SoundSection() {
  const { t } = useTranslation();
  const soundEnabled = resolveAudioEnabled(useAuthStore((s) => s.user?.soundEnabled));
  const musicEnabled = resolveAudioEnabled(useAuthStore((s) => s.user?.musicEnabled));
  const values: Record<AudioSwitchField, boolean> = { soundEnabled, musicEnabled };

  return (
    <SettingsGroup
      id="sound"
      title={t("settings.sections.sound")}
      icon={<Volume2 />}
      testId="profile-audio"
    >
      {ROWS.map((row) => (
        <SettingsRow
          key={row.field}
          asLabel
          label={t(row.labelKey)}
          control={
            <Switch
              checked={values[row.field]}
              onCheckedChange={(checked) => void persistAudioPreferences({ [row.field]: checked })}
              data-testid={row.testId}
            />
          }
        >
          <VolumeRow
            field={row.volume.field}
            label={t(row.volume.labelKey)}
            disabled={!values[row.field]}
            testId={row.volume.testId}
          />
        </SettingsRow>
      ))}
    </SettingsGroup>
  );
}
