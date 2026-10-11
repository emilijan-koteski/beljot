---
title: 'In-match sound effects and background music'
type: 'feature'
created: '2026-09-23'
status: 'done'
baseline_commit: '87c9d9e612dcb3e10a648d4e25b0afeeec316808'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/docs/card-deck.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Matches are completely silent — no feedback when a card is played or a trick is collected, and no table ambience.

**Approach:** Ship CC0 card sound effects (Kenney "Casino Audio") and a CC0 casino-jazz playlist (FreePD), played through one Web Audio engine and gated by two new account preferences — `soundEnabled` and `musicEnabled`, both default ON — toggleable separately from the in-game Settings dialog and the profile sidebar, plus a HUD mute button.

## Boundaries & Constraints

**Always:** Missing preference fields resolve to ON. Toggles apply instantly mid-hand with no reload. Persistence is optimistic with silent latest-wins revert, like the card deck. Sound effects play for reduced-motion users too. A reconnect/resync (`event:match_state`) never triggers sounds. Music plays only while `MatchPage` is mounted. Assets are MP3 under `client/public/audio`. All four locales get every new key; mk is all-Cyrillic.

**Decisions (user, 2026-09-23):** Preferences persist to the account (DB columns, synced across devices). Music is a 3-track rotating playlist: "Lucky Break" (Bryan Teoh), "A Good Bass for Gambling" (Komiku), "Bass Meant Jazz" (Kevin MacLeod). A HUD mute button toggles BOTH preferences: if either is on it turns both off, if both are off it turns both on.

**Never:** No volume sliders, no sounds beyond card-play and trick-collect, no audio outside the match page, no third-party audio library, no asset that is not CC0/public domain, no attribution UI/credits page, no commit until the user says so.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Opponent/bot plays | `event:card_played`, other seat | one card-slide sound | N/A |
| Own manual play | click playable card | sound at click; server echo (non-auto, own seat) silent | N/A |
| Own auto-play | `event:card_played` own seat, `autoPlayed` | one card-slide sound | N/A |
| Trick resolves | `pendingResolvedTrick` set | one collect sound when cards leave (full + reduced motion); no repeat on remount | N/A |
| Resync | `event:match_state` | no sound | N/A |
| Sound off | `soundEnabled=false` | no SFX; music unaffected | N/A |
| Music toggled mid-match | Settings/HUD/profile | fades out / starts immediately | N/A |
| No prior gesture | reload on `/match/:id` | silent until first pointer/key input, then music starts | play() rejection swallowed |
| PATCH fails | toggle with API error | store reverts that field only, unless superseded | silent |
| Invalid body | `soundEnabled: "yes"` | 400 | nothing written |

</frozen-after-approval>

## Code Map

- `server/migrations/000025_*` -- new; `sound_enabled`/`music_enabled BOOLEAN NOT NULL DEFAULT TRUE` (default is the backfill; comment style of 000020)
- `server/internal/user/model.go:23` -- add two bools `gorm:"default:true;not null"`; GORM omits zero-value `false` on Create, so DB default applies (registration never sets false)
- `server/internal/user/repository.go:41`, `gorm_repo.go:145` -- replace `UpdatePreferences(id, lang, deck *string)` with a `PreferencesUpdate` struct (two `*string`, two `*bool`); stays ONE `Updates(map)`. Seven stubs implement the interface: `user/handler_test.go:420`, `auth/handler_test.go:100`, `lobby/lobby_test.go:110`, `chat/handler_test.go:57`, `friend/handler_test.go:210`, `user/xp_service_test.go:43`, `user/honor_service_test.go:67`
- `server/internal/user/handler.go:150,634-688` -- request `*bool` fields; empty check covers all four; echo only written fields (`map[string]any`); `ProfileResponse` :49-53 filled :443
- `server/internal/auth/handler.go:37,174` -- `RegisterResponseData`/`authResponseData` echo both; `sso_handler.go:171` relies on DB defaults
- Client threading: every `cardDeckPreference` site — `shared/types/apiTypes.ts:27`, `shared/api/profile.ts:15,91,101`, `shared/api/auth.ts`, builders `shared/api/axiosClient.ts:147-160`, `shared/hooks/mutations/useAuth.ts:20-49`, `test-utils.tsx makeUser`
- `shared/lib/cardDeckPreference.ts` -- pattern to mirror for the new persist helper (optimistic + latest-wins)
- `shared/hooks/useWsDispatch.ts:183-228` -- `EVENT_CARD_PLAYED` branch; one call per WS message (4th card batched with `trick_resolved` still sounds)
- `features/match/MatchPage.tsx` -- `executePlayCard` :1013-1061 (optimistic self flight); collect effect :474-547 (`glowTimer` :511, reduced-motion `reducedTimer` :482); HUD desktop cluster :1801-1829, mobile menu :1936-1945
- `features/match/components/SettingsDialog.tsx` -- `SettingRow`/`SettingSection` (hardcoded `role="radiogroup"` :136)
- `features/profile/ProfilePage.tsx:193` -- mount point after `CardDeckPanel`; `components/SidePanel.tsx`
- `features/match/lib/cardFace.test.ts:88` -- `existsSync` asset-check pattern
- `shared/i18n/i18n.parity.test.ts` -- identical key sets + non-empty values across locales
- `@base-ui/react/switch` -- installed, unused; no Switch in `shared/components/ui/`
- `.gitattributes` -- no rule for mp3 (currently text/eol=lf)

## Tasks & Acceptance

**Execution:**
- [x] `client/public/audio/**`, `docs/audio.md`, `.gitattributes` -- fetch Kenney zip (card-slide-1..8, card-shove-1..4) and 3 FreePD tracks (`github.com/0lhi/FreePD` branch `stream`); transcode with imageio-ffmpeg to MP3 (SFX mono 96k; music stereo 96k, `loudnorm=I=-18:TP=-1.5`); record provenance + commands; add `*.mp3 binary`
- [x] `server/migrations/000025_add_audio_preferences_to_users.{up,down}.sql`, `server/internal/user/*`, `server/internal/auth/*`, 7 stubs -- persist + validate + echo the two prefs
- [x] Client types/builders/`makeUser` -- thread `soundEnabled`/`musicEnabled`
- [x] `client/src/shared/lib/audioPreference.ts` -- `resolveAudioEnabled(v) => v ?? true`; `persistAudioPreferences(patch)` with per-field latest-wins counters, PATCH body = changed fields only
- [x] `client/src/shared/audio/audioEngine.ts` -- lazy `AudioContext`, sfx/music gains, `preloadSfx`, `playSfx(name, {dedupeKey?})` (reads pref from auth store, random variant), music element via `createMediaElementSource` (iOS ignores `volume`), random start track, advance on `ended`, fade in/out, start cancels pending stop, gesture unlock listeners, no-op without `AudioContext`
- [x] `client/src/shared/audio/useMatchAudio.ts` + `MatchPage.tsx` -- hook lifecycle; SFX triggers at the Code Map points; HUD mute button (desktop + mobile); replace the "intentionally omitted" comment
- [x] `useWsDispatch.ts` -- card-play SFX when `playerSeat !== myPlayerSeat || autoPlayed`
- [x] `SettingsDialog.tsx` -- `SettingSection` `role` prop; brass `SettingSwitchRow` (`role="switch"`); "Sound" section with two rows
- [x] `shared/components/ui/switch.tsx`, `features/profile/components/AudioPanel.tsx`, `ProfilePage.tsx` -- profile sidebar panel (self-only)
- [x] `shared/i18n/{en,mk,hr,sr}.json` -- keys `match.settings.{soundHeading,soundEffects,music}`, `match.hud.{mute,unmute}`, `profile.audio.{eyebrow,title,description,sound,music}`
- [x] Tests for all of the above (Go handler/auth; engine, preference, dispatcher, SettingsDialog, AudioPanel, Profile/PublicProfile, MatchPage mute)

**Acceptance Criteria:**
- Given a fresh or existing account, when entering a match, then card sounds and music play by default.
- Given sound effects toggled off in Settings, when a card is played, then no sound plays and music continues; the profile panel shows the same state after reload.
- Given both prefs on, when the HUD mute is pressed, then both turn off, music fades out, and the icon shows muted; pressing again turns both on.
- Given the player leaves the match (lobby/room/back), then music stops.
- Given someone else's public profile, then no audio panel renders.

## Implementation Notes

- Implemented by a fresh-context subagent from this spec; verified by the orchestrator against the diff since `baseline_commit`.
- Added beyond the task list: an audio "session" counter (`openAudioSession`, opened by `useMatchAudio`) so SFX are dropped outside a mounted `MatchPage` — the trigger list alone did not guarantee "no audio outside the match page". SFX are also dropped while the `AudioContext` is still locked, so pre-gesture card plays do not burst out on unlock.
- After a completed `stopMusic`, the next `startMusic` picks a fresh random track rather than resuming.
- HUD mute calls `persistAudioPreferences({soundEnabled: next, musicEnabled: next})`; the helper drops fields that would not change, so the one PATCH carries only the switches that move.
- Gesture unlock listens to `pointerdown`, `pointerup`, `touchend`, `keydown` (touch grants activation on up, not down).
- Assets: 12 SFX ~100 KB, 3 music tracks ~8.3 MB; provenance, sha256 of sources and the regenerate recipe in `docs/audio.md`.
- Verification: client vitest 146 files / 2055 tests + tsc + eslint + prettier green; Go `go test ./...` green against a throwaway DB migrated to 000025 (DB-backed audio tests ran, not skipped), up/down migration verified, golangci-lint 2.12.2 0 issues. The local dev DB was NOT migrated — run `make migrate` before `go test ./...` or the dev server.
- Orchestrator added `useMatchAudio.test.tsx` "keeps the music playing when only sound effects are switched off" to cover the matrix's "Sound off → music unaffected" row.

## Spec Change Log

- 2026-09-23 — **Superseded in part.** The user renegotiated the "Never: No volume sliders" boundary after this spec shipped: per-channel 0–100 volume sliders are specified in `spec-in-match-audio-volume-sliders.md`, which governs volumes from then on. Everything else here stands; the frozen block above is left as the human approved it.

## Review Triage Log

| # | Layer | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|
| 1 | blind + edge | Music playlist stalls forever on a track `error` (only `ended` is handled) | medium | `audioEngine.ts` registers only `ended`; a failed load/stream fires `error`, so no advance and every later gesture's `play()` rejects silently | patch |
| 2 | blind | Failed SFX loads re-fetched on every `playSfx` with no backoff | low | Real only when assets fail (offline / missing file); 12 tiny fetches fail fast, no user-visible harm; a fix trades away mid-match recovery | reject |
| 3 | blind | `AudioContext` never suspended after the last session; context + SFX preload created even with sound off | low | Everyday: every player leaving a match keeps a running context for the SPA lifetime (battery on phones); hook preloads unconditionally, contrary to the plan's "preload when sound is on" | patch |
| 4 | blind + edge | `webkitAudioContext` fallback cannot decode (callback-only `decodeAudioData`) | low | Prefixed-only engines (Safari < 14.1) never store a buffer; the fix is a direct deletion of the fallback | patch |
| 5 | blind | No way to mute during score-reveal / capot / match-result overlays while music plays | low | HUD cluster and mobile menu render only at `overlayPhase === "normal"` (pre-existing gate, Settings hidden too); overlays are transient or end with leaving the page, which stops music; fix needs new overlay UI placement | reject |
| 6 | blind | Settings "Sound" group (and existing radiogroups) have no accessible name | low | `SettingSection` heading is a bare span; `role="group"` has no `aria-labelledby`; direct fix via `useId` | patch |
| 7 | blind + edge | Overlapping PATCHes of one field are unordered server-side | low | Inversion needs the later request to reach Postgres first; rare, same pattern as `persistCardDeck`; fix adds request serialization | reject |
| 8 | blind | Register/SSO rely on GORM `default:true` zero-value behaviour; mock coerces false→true | false | Correct by design and pinned against a real DB (`TestGormUserRepository_Create_AudioPreferencesDefaultOn`); `Create` never needs to write false | reject |
| 9 | blind + gap | `PublicProfileResponse` doc comment omits the two new private fields | low | `handler.go:113-114` list ends at `CardDeckPreference, UsernameChangedAt`; comment-only direct fix | patch |
| 10 | blind | `/audio/*` served `no-cache` | low | Same deliberate policy as every unhashed `public/` asset (cards); conditional requests answer 304 cheaply | reject |
| 11 | blind | Profile fetch does not copy the switches into the store | low | Same as the deck (not copied either); store refreshes on login/refresh; cross-device mid-session change is rare | reject |
| 12 | blind | `docs/audio.md` inaccuracies (~120 KB vs ~100 KB; autoplay sentence overstated; "no system ffmpeg") | low | SFX total 102,713 bytes; in-app navigation already has activation; ffmpeg line describes a machine | patch |
| 13 | blind | `gbcrSpy.mockRestore()` skipped when the remount test fails | low | Leaks a prototype mock into later tests only on failure; direct `try/finally` fix | patch |
| 14 | gap | Migration `DEFAULT TRUE` backfill never exercised | medium | GORM sends the tag default explicitly on Create, so no test hits the column default; a `DEFAULT FALSE` typo would ship green | patch |
| 15 | gap | Belot-deferred own play sound untested | medium | Only the direct-click path asserts `playSfx`; moving the call out of `executePlayCard` would silence Belot throws with no failing test | patch |
| 16 | gap | Language/deck `radiogroup` role no longer pinned | low | Role became a default param; no test asserts `radiogroup` | patch |
| 17 | gap | Auth-envelope → store mapping of the switches untested on the client | low | A same-typed swap type-checks; whole envelope mapping (wallet/XP/honor) is equally untested | defer |
| 18 | edge | Token refresh during an in-flight audio PATCH overwrites the optimistic value | low | `doRefresh` rebuilds the user from the pre-PATCH row; identical exposure already exists for deck/language | defer |
| 19 | edge | Two overlapping writes of one field that BOTH fail leave the store on an unsaved value | low | Offline double-toggle only; same shape as `persistCardDeck`; next refresh corrects; fix adds a confirmed-value store | reject |
| 20 | edge | The unlocking first click's own card sound is dropped (resume is async) | low | Once per cold load at most; `resume()` usually resolves between pointerdown and click; fix adds deferred playback | reject |
| 21 | edge | Click at the deadline + server auto-play → two card sounds | low | Needs a click inside the auto-play race window; harmless double slide; fix needs a timing heuristic | reject |

## Design Notes

Trigger placement: the card-play sound is keyed to WS messages (dispatcher), not to flight pushes, because reduced-motion users get no flights and the 4th card can be batched away from its flight. The own manual play is the one exception — it sounds at click, in step with the optimistic flight, and its non-auto server echo is skipped.

Music routes through Web Audio so a single gain node controls level on iOS, where `HTMLMediaElement.volume` is read-only. One `AudioContext` serves both buses, so one gesture unlock covers both. Starting gains: SFX ~0.6, music ~0.18; fade-in ~1.5 s, fade-out ~0.4 s. MP3 only — Safari/iOS < 18.4 cannot `decodeAudioData` Ogg.

Assets (licences verified at planning):
- Kenney "Casino Audio" 1.1, CC0 (zip's License.txt): `https://kenney.nl/media/pages/assets/casino-audio/2472606a04-1721639069/kenney_casino-audio.zip` → `Audio/card-slide-{1..8}.ogg` → `client/public/audio/sfx/card-slide-{1..8}.mp3`; `Audio/card-shove-{1..4}.ogg` → `.../sfx/card-shove-{1..4}.mp3`.
- FreePD (public domain, CC0; freepd.com closed 2025, mirror `https://raw.githubusercontent.com/0lhi/FreePD/stream/<path>`): `Romance/Lucky Break.mp3` (Bryan Teoh) → `client/public/audio/music/lucky-break.mp3`; `Miscellaneous/A Good Bass for Gambling.mp3` (Komiku) → `a-good-bass-for-gambling.mp3`; `Miscellaneous/Bass Meant Jazz.mp3` (Kevin MacLeod) → `bass-meant-jazz.mp3`.
- No system ffmpeg: `uv run --no-project --with imageio-ffmpeg python -c "import imageio_ffmpeg;print(imageio_ffmpeg.get_ffmpeg_exe())"` prints a static binary. Download to the session scratchpad, not the repo.
- `docs/audio.md` mirrors `docs/card-deck.md`: sources, licence, authors, original → shipped filenames, exact commands; not under `public/`.

Mute semantics: icon/label reflect "any on" (`Volume2` + "Mute") vs "both off" (`VolumeX` + "Unmute"); one PATCH carries both fields.

i18n (approved wording; mk Cyrillic, hr "glazba" vs sr "muzika"):

| key | en | mk | hr | sr |
|---|---|---|---|---|
| match.settings.soundHeading | Sound | Звук | Zvuk | Zvuk |
| match.settings.soundEffects | Sound effects | Звучни ефекти | Zvučni efekti | Zvučni efekti |
| match.settings.music | Music | Музика | Glazba | Muzika |
| match.hud.mute | Mute | Исклучи звук | Isključi zvuk | Isključi zvuk |
| match.hud.unmute | Unmute | Вклучи звук | Uključi zvuk | Uključi zvuk |
| profile.audio.eyebrow | Sound | Звук | Zvuk | Zvuk |
| profile.audio.title | Sound & music | Звук и музика | Zvuk i glazba | Zvuk i muzika |
| profile.audio.description | Card sounds and background music during matches. | Звуци од картите и музика во позадина за време на мечот. | Zvukovi karata i glazba u pozadini tijekom meča. | Zvukovi karata i muzika u pozadini tokom meča. |
| profile.audio.sound | Sound effects | Звучни ефекти | Zvučni efekti | Zvučni efekti |
| profile.audio.music | Music | Музика | Glazba | Muzika |

Test ids: `settings-sound-toggle`, `settings-music-toggle`, `mute-button`, `hud-menu-mute`, `profile-audio`, `profile-sound-toggle`, `profile-music-toggle`.

## Verification

**Commands:**
- `cd client && npx vitest run && npx tsc --noEmit && npx eslint . && npx prettier --check .` -- expected: all pass
- `cd server && go test ./... && golangci-lint run ./...` -- expected: all pass
- `grep -rn "cardDeckPreference" client/src server/internal` -- expected: every threading site also carries the audio prefs

**Manual checks (if no CLI):**
- Live match with bots: sounds on each card and trick collect; music fades in on entry, stops on leaving; toggles and mute apply instantly; reload on `/match/:id` stays silent until first click.
