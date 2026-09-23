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

import { SidePanel } from "./SidePanel";

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
 * One channel's volume slider, under its switch. Outside the switch's
 * `<label>` on purpose: inside it, every press on the track would also toggle
 * the switch. Disabled — value kept — while the switch is off.
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
      />
      <span
        aria-hidden
        className="text-ink-mute min-w-[4ch] text-right font-mono text-[11px] tabular-nums"
      >
        {value}%
      </span>
    </div>
  );
}

/**
 * Profile-sidebar audio switches and volumes, so sound effects and music are
 * settable outside a match as well as from the in-game Settings dialog and HUD
 * mute.
 *
 * Same optimistic write with silent, latest-wins revert as those two — all
 * three share `persistAudioPreferences`, so a change here and one in a match
 * are ordered against each other — and each switch or slider PATCHes only its
 * own field (a slider once, on release).
 *
 * SELF-ONLY. Every one of these is a private account preference, absent from
 * `PublicProfileResponse`, so the panel mounts on `ProfilePage` and must never
 * appear on `PublicPlayerProfilePage`.
 */
export function AudioPanel() {
  const { t } = useTranslation();
  const soundEnabled = resolveAudioEnabled(useAuthStore((s) => s.user?.soundEnabled));
  const musicEnabled = resolveAudioEnabled(useAuthStore((s) => s.user?.musicEnabled));
  const values: Record<AudioSwitchField, boolean> = { soundEnabled, musicEnabled };

  return (
    <SidePanel
      eyebrow={t("profile.audio.eyebrow")}
      title={t("profile.audio.title")}
      testId="profile-audio"
    >
      <p className="text-ink-mute mb-2.5 text-[12px]">{t("profile.audio.description")}</p>
      <div className="flex flex-col gap-2">
        {ROWS.map((row) => (
          <div
            key={row.field}
            className="border-border bg-surface-elevated flex flex-col gap-2 rounded-[10px] border px-3 py-2.5"
          >
            <label className="flex cursor-pointer items-center justify-between gap-3">
              <span className="text-ink text-[13px] font-medium">{t(row.labelKey)}</span>
              <Switch
                checked={values[row.field]}
                onCheckedChange={(checked) =>
                  void persistAudioPreferences({ [row.field]: checked })
                }
                data-testid={row.testId}
              />
            </label>
            <VolumeRow
              field={row.volume.field}
              label={t(row.volume.labelKey)}
              disabled={!values[row.field]}
              testId={row.volume.testId}
            />
          </div>
        ))}
      </div>
    </SidePanel>
  );
}
