---
title: 'Volume sliders for in-match sound effects and music'
type: 'feature'
created: '2026-09-23'
status: 'done'
baseline_commit: 'b6e7dc5b107b4c8299a80d0cf0bd8fc4fd8bf49f'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/spec-in-match-sound-and-music.md'
  - '{project-root}/docs/audio.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sound effects and music can only be switched on or off; players cannot set how loud either one is.

**Approach:** Add one volume slider per channel (sound effects, music), 0–100 %, persisted to the account alongside the existing switches, and shown under each switch in the in-game Settings dialog and the profile Audio panel. This supersedes the "no volume sliders" boundary of `spec-in-match-sound-and-music.md`.

## Boundaries & Constraints

**Always:** The on/off switches and HUD mute stay exactly as they are; a slider only sets its channel's level and is disabled while that channel's switch is off. Mute then unmute restores the chosen volumes. Missing volume fields resolve to the default (70), which reproduces today's loudness exactly. Dragging applies the level live; the value persists on release (one PATCH per commit, optimistic, per-field latest-wins silent revert, like the switches). Volume 0 with the switch on is silent. All four locales get every new key; mk all-Cyrillic.

**Decisions (user, 2026-09-23):** Keep the switches and add sliders under them (not sliders-replace-switches). Build on top of the uncommitted audio feature without committing it.

**Never:** No master volume, no per-sound volumes, no new sounds, no audio outside the match page, no third-party slider library, no commit until the user says so.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Existing/new account | no volume set | both sliders show 70; loudness unchanged from today | N/A |
| Drag music slider mid-match | music playing | level follows the thumb live; one PATCH `{musicVolume}` on release | N/A |
| Commit SFX slider in-match | release at N | one card-play sound at the new level; PATCH `{soundVolume: N}` | N/A |
| Slider at 0 | switch on | that channel silent; switch unchanged | N/A |
| Switch off | any volume | slider disabled; value kept | N/A |
| Mute then unmute | volumes 30/80 | both come back at 30/80 | N/A |
| PATCH fails | commit | slider and live level revert to the previous value | silent |
| Invalid value | `-1`, `101`, `50.5`, `"x"` | 400 | nothing written |

</frozen-after-approval>

## Code Map

- `server/migrations/000026_add_audio_volumes_to_users.{up,down}.sql` -- new; `sound_volume`/`music_volume SMALLINT NOT NULL DEFAULT 70 CHECK (… BETWEEN 0 AND 100)`; comment style of 000025
- `server/internal/user/model.go` -- `SoundVolume`/`MusicVolume int` `gorm:"default:70;not null"` next to the audio switches
- `server/internal/user/repository.go` `PreferencesUpdate` (+ `IsEmpty`), `gorm_repo.go` `UpdatePreferences` -- add `SoundVolume`/`MusicVolume *int`, same single `Updates(map)`
- `server/internal/user/handler.go` -- `UpdatePreferencesRequest` gets `*int` fields (the `PreferencesUpdate(req)` conversion must keep compiling); range-check 0..100 → `apperr.ErrBadRequest` before any write; echo written fields; `ProfileResponse` + the Public comment's private-field list
- `server/internal/auth/handler.go` `RegisterResponseData`/`authResponseData` -- echo both volumes
- Client threading: every site carrying `soundEnabled` (`shared/types/apiTypes.ts`, `shared/api/{auth,profile,axiosClient}.ts`, `shared/hooks/mutations/useAuth.ts`, `test-utils.tsx makeUser`, profile fixtures) -- add `soundVolume`/`musicVolume`
- `shared/lib/audioPreference.ts` -- `resolveAudioVolume(v)` (integer clamp 0..100, missing → 70); extend `AudioPreferenceField`/patch and the per-field latest-wins counters to the two volume fields
- `shared/audio/audioEngine.ts` -- replace `SFX_LEVEL`/`MUSIC_LEVEL` with gain-at-default constants (0.6 / 0.18 at volume 70) scaled linearly by volume; `playSfx` reads `soundVolume` at play time; new `setMusicVolume(v)` ramps the music gain (~0.1 s) when playing and not fading out, and is the level `startMusic`/fades target
- `shared/audio/useMatchAudio.ts` -- effect on resolved `musicVolume` → `setMusicVolume`
- `@base-ui/react/slider` (installed, v1.3.0: `onValueChange` live, `onValueCommitted` on release) -- new `shared/components/ui/slider.tsx` wrapper, styled via className so the brass dialog and the profile panel can both use it
- `features/match/components/SettingsDialog.tsx` Sound section, `features/profile/components/AudioPanel.tsx` -- a slider row under each switch
- `docs/audio.md` "When it plays" -- document volumes

## Tasks & Acceptance

**Execution:**
- [x] Server: migration 000026, model, repo, handler validation/echo, auth echo, update every `UserRepository` stub only if its signature is affected; Go tests (valid 0/100, invalid -1/101/50.5/"x", volume-only PATCH leaves others alone, raw-insert default 70, echo on register/login/refresh/profile)
- [x] Client types + threading sites + `makeUser`
- [x] `audioPreference.ts` -- `resolveAudioVolume` + volume fields; tests
- [x] `audioEngine.ts` + `useMatchAudio.ts` -- volume-scaled gains, `setMusicVolume`; tests (default 70 = today's gains, 0 silent, live music change, fade targets honour volume)
- [x] `shared/audio/useAudioVolume.ts` (new) -- per-channel hook: local draft while dragging (music drafts drive `setMusicVolume` live), commit → `persistAudioPreferences`, SFX commit → `playSfx("cardPlay")`; shared by both UIs
- [x] `slider.tsx`, `SettingsDialog.tsx`, `AudioPanel.tsx` -- slider rows (accessible name from the volume key, visible `N%`), disabled while the switch is off; test ids `settings-sound-volume`, `settings-music-volume`, `profile-sound-volume`, `profile-music-volume`; tests
- [x] i18n `{en,mk,hr,sr}.json` -- keys below; `docs/audio.md`

**Acceptance Criteria:**
- Given music volume set to 30 in Settings, when the page reloads or the profile opens, then the slider shows 30 and music plays at that level.
- Given sound effects switched off, when the Settings dialog or profile panel is open, then the sound slider is disabled and its value is kept for when the switch returns on.

## Design Notes

Why a local draft: the store must keep the pre-drag value until release, so `persistAudioPreferences` computes the right `previous` for a revert and sends exactly one PATCH. Music drafts go straight to the engine for live feedback; the store-driven `setMusicVolume` effect then settles on the committed (or reverted) value.

i18n (mk Cyrillic; hr "glazba"/"glasnoća", sr "muzika"/"jačina"):

| key | en | mk | hr | sr |
|---|---|---|---|---|
| match.settings.soundVolume | Sound effects volume | Јачина на звучните ефекти | Glasnoća zvučnih efekata | Jačina zvučnih efekata |
| match.settings.musicVolume | Music volume | Јачина на музиката | Glasnoća glazbe | Jačina muzike |
| profile.audio.soundVolume | Sound effects volume | Јачина на звучните ефекти | Glasnoća zvučnih efekata | Jačina zvučnih efekata |
| profile.audio.musicVolume | Music volume | Јачина на музиката | Glasnoća glazbe | Jačina muzike |

The local dev DB is at migration 24 (000025 unapplied). DB-backed Go tests must run against a throwaway database on the local postgres with every up migration applied, then drop it; never migrate the dev DB.

## Implementation Notes

- Baseline is a snapshot commit of the uncommitted audio feature (`b6e7dc5…`, held by `refs/bmad/baseline-volume-sliders`, not on any branch), so diffs against it show only the slider work.
- Implemented by a fresh-context subagent; verified by the orchestrator against the diff: client vitest 147 files / 2129 tests + tsc + eslint + prettier green; Go `go test ./...` green against a throwaway DB migrated through 000026 (DB-backed volume tests ran), 000026 down verified, golangci-lint 2.12.2 0 issues.
- Validation also rejects `"50"`, `true` and `1e2` (JSON decode into `*int`). A DB-level CHECK test confirms out-of-range writes are refused by Postgres too.
- `setMusicVolume` glides live only once the music is faded in; mid fade-in it keeps the fade's landing time; during a fade-out it only records the level for the next start.
- `useAudioVolume` restores the stored music level if the dialog unmounts mid-drag (nothing committed).
- Base UI treats each keyboard step as a commit, so arrow keys PATCH once per step.
- Dev DB was found at 25 (000025 applied outside this run); 000026 still needs `make migrate`.

- Review patches: commits are coalesced (isolated commit saves at once; commits within 250 ms are held and saved once when quiet; held value saved on unmount), so a held arrow key sends a leading + one trailing PATCH/preview. Commits with no value change (Base UI's stale `lastChangedValueRef`) are ignored. Music volume 0 is treated like the switch off, so the playlist stops (consequence: dragging up from a saved 0 is silent until release). Slider hit area 24 px; explicit min/max/step; DB CHECK test asserts SQLSTATE 23514 per column; real pointer-drag component tests on both surfaces.
- Final verification: client 2139 tests + tsc + eslint + prettier green; Go `go test ./...` green on a throwaway DB migrated through 000026; golangci-lint 0 issues.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|
| 1 | blind | Two overlapping writes of one volume that BOTH fail leave the store on an unsaved value | low | Offline/server-down only; same shape as the switches (audio spec row 19); next refresh corrects; fix adds a confirmed-value store | reject |
| 2 | blind + edge | Every keyboard step commits: a held arrow key sends a PATCH and plays a card sound per auto-repeat (also at the limit) | medium | Base UI `handleInputChange` calls `onValueCommitted` on every keydown (SliderRoot.js:203-213); `useAudioVolume` persists + previews on each | patch |
| 3 | blind | HUD mute shows "Mute" while both volumes are 0 | low | Volume 0 is the player's explicit choice; the spec defines mute on the switches only | reject |
| 4 | blind | Music at volume 0 keeps streaming/decoding the playlist at gain 0 | low | `useMatchAudio` starts music whenever the switch is on; wastes data/battery and on iOS a playing element holds the audio session over the player's own music; one-condition fix | patch |
| 5 | blind | Linear curve: 70→100 is only +3.1 dB, most audible change sits low on the slider | low | Real, but linear is the spec's explicit choice with 70 = today's loudness; perceived feel is subjective and needs a listen — surfaced to the user as a tunable | reject |
| 6 | blind | Profile sliders give no audible feedback; profile drag calls `setMusicVolume` needlessly | low | No audio off the match page is a Never; the panel copy already says "during matches"; the engine call is a harmless recorded level that the next match mount overwrites from the store | reject |
| 7 | blind | Slider hit area below 24×24 (control h-5, thumb 16px) | low | `slider.tsx` sizes; phone players drag it; direct CSS fix | patch |
| 8 | blind | Percent readout hardcoded `${v}%` (mk/hr use "30 %") | low | Matches the app's existing percent convention (e.g. `WinRateRing`); cosmetic | reject |
| 9 | blind | Two near-identical volume row components | low | Mirrors the deliberate split between the brass dialog rows and the token-styled profile panel (as with the deck pickers) | reject |
| 10 | blind + edge | Drag that ends without a commit (switch disabled mid-drag, pointercancel) leaves a stale draft | low | Needs a PATCH failure or a cancelled pointer mid-drag; fix adds guard state across disable/cancel | reject |
| 11 | blind | Slider range (0/100/1) is implicit | low | Relies on Base UI defaults; server rejects anything else with 400; direct fix | patch |
| 12 | blind | DB CHECK test asserts only "some error", covers only `music_volume` > 100 | low | `user_test.go` uses `assert.Error`; direct test tightening (SQLSTATE 23514, both columns, −1) | patch |
| 13 | blind | Out-of-range volume returns `BAD_REQUEST` not an `INVALID_*` code; default 70 literal | false | `ErrBadRequest` is the spec's explicit choice and the client reverts silently whatever the code; the gorm tag must be a literal | reject |
| 14 | blind + gap | Pointer drag wiring untested: change/keyboard fire change+commit together, so dropping or miswiring `onValueChange` passes | medium | Pre-verified by the gap layer against SliderRoot.js; only hook-level drag tests exist | patch |
| 15 | blind | Audio spec still says "Never: No volume sliders" with no pointer to the superseding spec | low | Stale guidance for readers; append-only change-log note on the done spec, outside its frozen block | patch |
| 16 | edge | Base UI commits a stale `lastChangedValueRef` when nothing changed (thumb click after a revert, End at max), re-saving an old level | low | Verified: `setValue` early-returns on an equal value (SliderRoot.js:172) yet commit uses `lastChangedValueRef.current ?? value` (:211, SliderControl.js:273); one-line guard in `useAudioVolume` | patch |
| 17 | gap | Go DB tests fail on the dev DB (at 25) | false | Environment, not code: every DB-backed test passes on a throwaway DB migrated through 000026; the user must `make migrate` | reject |

## Verification

**Commands:**
- `cd client && npx vitest run && npx tsc --noEmit && npx eslint . && npx prettier --check .` -- expected: all pass
- `cd server && go test ./... && golangci-lint run ./...` (golangci-lint v2; throwaway DB for DB tests) -- expected: all pass

**Manual checks (if no CLI):**
- Live match: dragging music changes loudness live; SFX release plays one card sound at the new level; sliders disable with their switch; values survive reload.
