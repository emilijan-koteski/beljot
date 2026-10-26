---
title: 'Whistle sound when a room invite pops up'
type: 'feature'
created: '2026-10-26'
status: 'done'
route: 'dispatch'
review_loop_iteration: 1
context: ['{project-root}/docs/audio.md']
baseline_commit: 'ab6354f1a403237ef00db88e8bdbb6fb369b3640'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** A room invite appears silently. A player idle in the lobby, or looking at another tab of the app, can miss it before it expires (60 s).

**Approach:** Play a short whistle once, when the invite popup appears. It ships as one new sound effect beside the existing ones, sourced and documented the same way, and it obeys the player's existing sound-effects switch and volume.

## Boundaries & Constraints

**Always:**
- The asset is CC0 or public domain, transcoded with the `docs/audio.md` recipe (mono, 96 kb/s MP3, no metadata), level-matched to `popup-1.mp3`, and recorded in that doc's provenance table and regeneration script.
- One whistle per invite, keyed to the invite (`inviteId` + `expiresAt`, so that the server's counter restarting at 1 after a deploy cannot silence a new invite; owner amendment, 2026-10-26). A remount, a re-render or the same invite reappearing never repeats it.
- It plays only when the popup is actually shown. An invite hidden behind the ejection notice stays silent until it shows.
- `soundEnabled = false` or `soundVolume = 0` means silence; otherwise it plays at the player's sound-effects volume.
- It works outside a match, wherever the app-wide popup appears. If the browser has not yet allowed audio (no prior click on the page), it stays silent rather than queueing a sound for later.

**Never:**
- No new preference, switch or WS field. The existing sound-effects switch governs it.
- Do not open a permanent app-wide audio session, and do not create the `AudioContext` before an invite arrives.
- Do not change any existing sound or when it plays.

**Decisions (owner, 2026-10-26):**
- **The sound:** Kenney "Interface Sounds" 1.0, `Audio/confirmation_002.ogg` (an up-then-down chirp, about 0.57 s), shipped as `sfx/invite.mp3` with `volume=-8dB`. That is the gain that matched its loudest 50 ms (−7.4 dBFS RMS) to `popup-1.mp3` (−15.5 dBFS) in the preview the owner approved. No Kenney pack contains a literal whistle; this is the owner's pick from five previews. Same pack, zip and checksum as `popup-*` and `clock-tick`, so `docs/audio.md` gains a table row and a recipe line but no new source.
- **Spec size:** kept as one spec.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Invite arrives | sound on, popup shown | whistle plays once at the sound-effects volume | N/A |
| Same invite re-rendered | popup remounts with the same `inviteId` | silent | N/A |
| Second invite | a new `inviteId` replaces the first | whistles again | N/A |
| Sound off | `soundEnabled=false` or `soundVolume=0` | silent; nothing fetched | N/A |
| Behind ejection notice | `roomEjection` set when the invite arrives | silent; whistles once when the popup shows | N/A |
| Audio not yet allowed | no user activation yet, so the context will not run | silent, no deferred burst later | swallowed |
| Asset fetch fails | 404 or decode error | silent | swallowed, like every SFX |

</frozen-after-approval>

## Code Map

- `client/src/shared/audio/audioEngine.ts`:
  - `SfxName` (`:8`) and `SFX_URLS` (`:26`) gain an `invite` entry pointing at `sfx/invite.mp3`.
  - `playSfx` (`:160`) returns at once when `sessions === 0` (`:170`) or the context is not `running` (`:178`), and plays nothing while its buffer is still loading (`:183`). Called as-is from the lobby, the first invite would always be silent.
  - Reuse `loadBuffer` (`:127`), `ensureContext` (`:96`) and the gain and variant pick inside `playSfx`; do not duplicate them. Do **not** reuse `openAudioSession` (`:372`) for the alert. `sessions > 0` is the gate that lets `playSfx` sound at all, and outside a match the profile `AudioPanel` slider preview (`useAudioVolume.ts:35`, `playSfx("cardPlay")`) relies on it staying closed. See the Spec Change Log.
  - `preloadSfx` (`:149`) loads every `SFX_URLS` entry. The invite file joins the match preload, which is harmless at a few KB.
- `client/src/features/lobby/components/RoomInviteModal.tsx`: mounted app-wide by `shared/components/AppLayout.tsx:16`. It renders nothing while `invite === null` or `ejectionNoticeTakesPriority` (`:82-85`); trigger the sound only when it renders the popup.
- `client/src/shared/audio/useSfxOnce.ts`: the existing dedupe-on-mount hook, the pattern to mirror.
- `client/src/shared/audio/audioEngine.test.ts:213` hard-codes 29 effects. The asset-existence test fails if a referenced file is missing.
- `client/src/features/lobby/components/RoomInviteModal.test.tsx`: the existing suite for the popup.
- `docs/audio.md`: the file list and count (29 → 30), the total size, "When it plays", the provenance table, and the regeneration script.
- The Kenney zips were re-downloaded to the scratchpad during planning; the casino, interface and jingles checksums match `docs/audio.md`.

## Tasks & Acceptance

**Execution:**
- [x] `client/public/audio/sfx/invite.mp3` -- encode `Audio/confirmation_002.ogg` from the Interface Sounds zip with the doc's `sfx` helper and `volume=-8dB`. Download the zip to a scratch dir and check its sha256 against `docs/audio.md` first; get ffmpeg ephemerally via `imageio-ffmpeg` as the doc describes -- one asset, house format.
- [x] `client/src/shared/audio/audioEngine.ts` -- add `invite` to `SfxName`/`SFX_URLS`; add an exported alert play that runs outside a match: dedupe and preference check first, then a transient session, await the buffer and a resume attempt, play only if the context is running, then release the session after the sound ends -- lobby-safe without an app-wide session.
- [x] `client/src/features/lobby/components/RoomInviteModal.tsx` -- call the alert with `inviteId` as the dedupe key when the popup renders -- the popup is the moment.
- [x] Tests -- engine: the new effect, the count, the alert path (plays once, silent when sound is off or the context won't run, nothing fetched when off, session released). Modal: one call per `inviteId`, none behind the ejection notice.
- [x] `docs/audio.md` -- the count (29 → 30), the total size, a provenance table row, the regeneration line and a "When it plays" entry. The file list there says "in-match", so note that this one effect plays outside a match.

**Execution — review loop 1 (fix on top of the verified code, owner decision):**
- [x] `client/src/shared/audio/audioEngine.ts` -- replace the alert's `openAudioSession()` with an alert-only hold. It keeps the context from being suspended and is released after the sound, but it never opens the `playSfx` gate (`sessions`) and never installs the match gesture listeners. When the last hold and the last session are both gone, suspend the context, including one that started running after its alert gave up (a late resume). Put one deadline (the existing grace) on the whole load-and-resume wait, not only on the resume. Accept an `AbortSignal` and return silently if it is aborted before the sound starts -- keeps "do not change any existing sound or when it plays", with no late chirp.
- [x] `client/src/features/lobby/components/RoomInviteModal.tsx` -- dedupe key `${inviteId}:${expiresAt}`. Create an `AbortController` per shown invite and abort it in the effect cleanup, which covers accept, decline, expiry on arrival, replacement and hide. Build the key from `invite` directly; the separate `inviteId === null` guard is redundant -- frozen key amendment; cancellation.
- [x] Tests -- engine: `playSfx` stays silent outside a match while an alert is sounding; an aborted alert stays silent; the overall deadline holds while a fetch stalls; a late resume after the grace gets suspended; a click within the grace plays; a synchronously throwing `resume()` is silent. Modal: the key includes `expiresAt`; the same `inviteId` with a new `expiresAt` chirps again; hiding and re-showing the same invite reuses the same key; unmount and replacement abort.
- [x] `docs/audio.md` -- remove "room" from where the invite can sound (invites only reach players idle outside rooms and matches); state the 1 s grace precisely ("a click within one second of the popup still lets it sound; never later"); resolve the ambiguous "its 7.1 build" wording.

**Acceptance Criteria:**
- Given a player idle in the lobby with sound on who has clicked anywhere in the app, when a room invite arrives, then the whistle plays once as the popup appears.
- Given `make lint` and `make test`, when run, then both pass.

## Implementation Notes

- **Shipped asset:** `invite.mp3` is 7,253 bytes, 0.57 s. Its loudest 50 ms is −15.8 dBFS RMS against −15.5 for `popup-1.mp3`. The zip's sha256 matched `docs/audio.md`, and its `License.txt` says CC0. It was encoded with the cached `imageio-ffmpeg` 7.1 build, not 7.0.2; `docs/audio.md` records that.
- **Engine refactor:** `playSfx` was split into `claimDedupeKey`, `audibleSfxVolume` and `startSfx` so that `playAlertSfx` shares them; `playSfx` behaves exactly as before. The alert waits at most `ALERT_RESUME_GRACE_MS` (1 s) for the context to start and holds its session for the sound's length plus 200 ms.
- **Known risk, raised by the implementer:** the server's `inviteId` comes from an in-memory counter that restarts at 1 on every deploy. A tab left open across a deploy remembers up to 256 played keys, so a new invite that reuses an old ID would be silent. This is left for review triage.
- **Verification:** client `tsc`, `eslint` and Prettier 3.9.10 are clean. Vitest passes 2,440 of 2,441; the failure is the known mk-MK RankChangeDialog locale test. The server is unchanged since the baseline.
- **Review loop 1 (fix on top):**
  - The alert now takes an `alertHolds` hold instead of a session, and one shared `scheduleSuspendIfIdle` serves both kinds of release.
  - `ALERT_GRACE_MS` bounds the load and the start together (`settlesWithin`).
  - A start granted after the last hold is gone is suspended again.
  - `playAlertSfx` takes an `AbortSignal`, and the modal aborts it in its effect cleanup.
  - The key is now `${inviteId}:${expiresAt}`.
- **Accepted implementer decisions (loop 1):**
  - (a) While an alert waits, at most 1 s, a click or key-press listener of its own calls `resume()`. Without it Chrome never starts a context it refused, and the spec's "a click within the grace plays" could not hold. It is removed when the wait ends and is separate from the match listeners.
  - (b) The dedupe key is checked up front but spent only when the sound starts. `<StrictMode>` runs the effect, aborts it in cleanup, and runs it again; spending the key on the first call would silence every invite in dev. Consequence: an invite that never sounded (sound off, audio locked, fetch failed) may chirp if shown again later. One that did sound never repeats.
- **Verification (loop 1):** `tsc`, `eslint` and Prettier 3.9.10 are clean. Vitest passes 2,451 of 2,452 (only the known mk-MK locale failure), including the engine (102), modal (23), `useAudioVolume` (14) and `useMatchAudio` (14) suites.

## Spec Change Log

- **2026-10-26 — review loop 1 (bad_spec, fixed on top by owner decision).**
  - **Trigger:** BH3 and VG-other1, confirmed in code. The Design Notes told the implementer to hold the alert with `openAudioSession()`. That raises `sessions`, the very gate `playSfx` checks (`audioEngine.ts:205`), so for the alert's ~1–2 s any off-match `playSfx` could sound. Concretely, the profile volume-slider preview `playSfx("cardPlay")` could play or fetch the card sounds, breaking the frozen "Do not change any existing sound or when it plays".
  - **Amended:** the Design Notes and the Code Map (an alert-only hold, not a session); a review-loop-1 task block that folds in the patch-level findings (cancellation, the overall deadline, suspending after a late resume, the composite key, docs, tests); and, by owner decision, the frozen dedupe line, now `inviteId` + `expiresAt`, because the server's invite counter restarts at 1 after a deploy.
  - **Known-bad state avoided:** an invite alert silently re-enabling match-only sounds off the match page.
  - **Process:** the owner chose to fix on top of the verified code instead of reverting and re-deriving.
  - **KEEP on any re-derivation:** the shipped `invite.mp3` (sha256-checked source, −8 dB, measured −15.8 dBFS); the `playSfx` split into `claimDedupeKey`, `audibleSfxVolume` and `startSfx` (behaviour unchanged); the 1 s grace that never plays later; the modal firing only while the popup is visible; the passing engine and modal tests.

## Review Triage Log

Pass 1 (2026-10-26). Reviewers: Blind Hunter (BH), Edge Case Hunter (EC) and Verification Gap (VG; no gap findings). The bad_spec entry triggered loop 1, and the owner chose to fix on top of the verified code.

| # | Finding | Verdict | Evidence | Route |
|---|---------|---------|----------|-------|
| BH3 | The alert's `openAudioSession()` lets off-match `playSfx` sound during the alert | medium | Confirmed: `playSfx` gates only on `sessions === 0` (`:205`), and the alert increments it. `useAudioVolume.ts:35` previews `cardPlay` from the profile panel. This violates the frozen Never. The cause is the Design Notes, which prescribed `openAudioSession`. | bad_spec |
| VG-other1 | Same as BH3 | medium | Same root cause. | bad_spec |
| BH1 | The dedupe key `inviteId` collides after a server restart | medium | Confirmed: `InviteRegistry.nextID` is an in-process counter (`invite_registry.go:26,69`), and `playedKeys` lives for the tab. `useVersionCheck` reloads only on a new build, after up to 5 minutes. | loop 1 (owner-amended frozen key) |
| EC5 | Same collision | medium | Same as BH1. | loop 1 |
| VG-other2 | Same collision | medium | Same as BH1. | loop 1 |
| BH2 | The chirp can't be cancelled once the popup closes; two quick invites can sound together | low | Confirmed: the effect has no cleanup and the engine awaits the load with no liveness check. | loop 1 |
| EC2 | Same: chirp after decline, expiry, replacement or unmount | low | Same as BH2. | loop 1 |
| EC1 | An invite already expired on arrival chirps for a popup dismissed in the same commit | low | Same root cause as BH2: the abort in the effect cleanup covers it. | loop 1 |
| EC3 | No deadline on the load half of the wait | low | Confirmed: only `resumeWithin` is time-limited (`:271-274`). | loop 1 |
| EC4 | A resume honoured after the grace leaves the context running outside any session | low | Confirmed: the suspend timer skips a context that is not running (`:483`), and the gesture listeners are already gone. | loop 1 |
| BH5 | No modal test for hiding and then re-showing the same invite | low | Only the engine's dedupe guards it, and the modal suite mocks the engine. | loop 1 |
| BH6 | No test for a click within the grace, or for `resume()` throwing synchronously | low | Confirmed absent from the new engine cases. | loop 1 |
| BH7 | The docs say the invite sounds in a "room", and omit the grace | low | `availabilityReason` refuses players who are in a room or a match. The "never on a later click" wording ignores the 1 s grace. | loop 1 |
| BH4 | iOS/WebKit: a context created outside a gesture may never start, so lobby-only players hear nothing | maybe-false | Untestable here. WebKit's gesture-only unlock would make the AC fail on iOS, and the frozen "do not create the AudioContext before an invite" forbids the usual early-unlock workaround. A device test would settle it. | defer |
| EC6 | Same iOS claim | maybe-false | Same as BH4. | defer |
| BH8 | `invite.mp3` is preloaded on every match mount for nothing | low | The spec's Code Map accepted it explicitly ("harmless at a few KB"); about 7 KB. | reject |
| BH9 | The recipe can't reproduce the bytes exactly (unpinned `imageio-ffmpeg`, two builds) | low | The unpinned recipe pre-dates this change and the output is audio-equivalent. The ambiguous wording is fixed under BH7's doc task. | reject |
| BH10 | Whistle vs chirp naming; a redundant `inviteId === null` guard | low | Naming is cosmetic: the owner calls it a whistle. The guard goes away with the key change in loop 1. | reject |

Pass 2 (2026-10-26, after loop 1). Same three reviewers. None of the findings repeated a pass-1 row at the same location with the same claim. All survivors route to patch, so there is no loopback.

| # | Finding | Verdict | Evidence | Route |
|---|---------|---------|----------|-------|
| P2-BH1 | The key is spent before `startSfx`, so a failed fetch or start silences a re-show | low | Confirmed: `claimDedupeKey` runs before `startSfx`, and `loadBuffer` swallows errors, so `startSfx` returns null with the key already spent. This contradicts the function's doc and accepted decision (b). | patch |
| P2-EC1 | Same | low | Same root cause. | patch |
| P2-BH3 | While audio is locked, the player's own press on Accept or Decline unlocks it, so the chirp sounds as they respond; after Accept the key is unchanged, so it can also sound over the password prompt | low | Confirmed: the window capture listener resumes on any `pointerdown`. `shownInviteKey` doesn't change while `isJoining` or `showPasswordPrompt`, so nothing aborts until `dismiss()`. | patch (abort when the player acts) |
| P2-EC3 | Same: pointer-down on Decline unlocks before the click aborts | low | Same root cause. | patch |
| P2-EC8 | Same: the claim that cleanup "covers accept" is false | low | Same root cause. | patch |
| P2-BH4 | The signal is read only after the wait; an already-aborted signal still creates the context and fetches | low | Confirmed: the only check is after `readyWithinGrace` (`:316`). | patch |
| P2-EC7 | Same | low | Same root cause. | patch |
| P2-EC6 | A throttled hidden tab can stretch the grace timer, so a later click still sounds | low | Real: browsers clamp timers in background tabs, which is exactly the "another tab" case. Measuring elapsed time is a two-line fix. | patch |
| P2-VG1 | Removing `alertHolds > 0` from `scheduleSuspendIfIdle` leaves every engine test green | low | Pre-verified by a mutation run in the scratchpad: probes A and B fail on the mutant and pass on the current code. | patch |
| P2-BH8 | No test for the interplay between alert holds and sessions | low | Same as P2-VG1. | patch |
| P2-BH7 | The modal tests miss Accept, the password prompt, expired-on-arrival and expiry-by-timer aborts | low | Confirmed absent. They're needed alongside the P2-BH3 fix. | patch |
| P2-BH5 | The docs claim the popup sounds on the rules page | low | Confirmed: `/rules` renders under `PublicContentLayout` (`App.tsx:78-79`), which never mounts `RoomInviteModal` (`AppLayout.tsx:16` only). Doc correction. | patch (doc) + defer (pre-existing missing popup) |
| P2-EC4 | Same rules-page claim | low | Same as P2-BH5. | patch (doc) |
| P2-BH6 | Doc inaccuracies: "created by the first invite", hold length, measured level | low | Confirmed against the code and the Implementation Notes (−15.8 dBFS). Direct correction. | patch |
| P2-BH10 | Stale doc comments on `openAudioSession` and `RoomInviteModal` | low | Confirmed. Direct correction. Spec notes are append-only history, so they were not rewritten. | patch |
| P2-BH2 | WebKit may reject `resume()` instead of leaving it pending, ending the grace early so an in-grace click can't unlock | maybe-false | Plausible, but untestable here: the fake context models only pending or throwing, and WebKit's async rejection is unverified. It joins the deferred Safari check. | defer |
| P2-BH9 | The test reset doesn't tear down an in-flight alert (listeners, settle timer, a release that could take `alertHolds` to −1) | low | Real, but only in the test seam: every test awaits its alert. A fix needs generation tracking, more than a direct correction. | reject |
| P2-BH11 | `ALERT_RELEASE_MARGIN_MS` overlaps `SUSPEND_DELAY_MS` | low | Cosmetic. The margin keeps the alert independent of the music fade delay. | reject |
| P2-EC2 | An alert starting while `ctx.suspend()` is still in flight starts a source that then freezes | low | Real, but the window is the few milliseconds the suspend promise takes. Tracking it adds state. | reject |
| P2-EC5 | A hung `invite.mp3` fetch blocks every later alert, because `loadBuffer` dedupes in-flight requests | low | Real only for a fetch the browser never times out. A timeout in `loadBuffer` would change every match effect's loading. | reject |
| P2-EC9 | On a slow network the first invite after a load is silent because fetch and decode exceed the grace | low | By design: the grace bounds lateness, and the file is about 7 KB. | reject |

## Design Notes

The alert path in outline (names are the implementer's; amended in review loop 1):

```ts
let alertHolds = 0; // keeps the context awake; NOT the playSfx gate (`sessions`)

export async function playAlertSfx(name, { dedupeKey, signal }) {
  // dedupe + soundEnabled/volume gate first: nothing is created or fetched when off
  alertHolds += 1; cancelPendingSuspend();
  try {
    const context = ensureContext();
    const ok = await withDeadline(GRACE, loadAll(context, SFX_URLS[name]), resumeAttempt(context));
    if (!ok || signal?.aborted || context.state !== "running") return;
    start(name, volume); // the play core shared with playSfx
  } finally { after(soundMs + margin, () => { alertHolds -= 1; suspendIfIdle(); }); }
}
```

`suspendIfIdle` suspends the context when `sessions === 0 && alertHolds === 0`. It must also catch a context that started running after its alert gave up, so watch for a late resume through the pending promise or `onstatechange`. Releasing the hold too early cuts the sound.

## Verification

**Commands:**
- `make lint` -- expected: clean. Use `npx prettier@3.9.10` locally; the installed 3.8.1 is stale.
- `make test` -- expected: green apart from the known mk-MK RankChangeDialog locale failure.

**Manual checks:**
- Two accounts: from a room, invite the other, who is idle in the lobby and has already clicked once. The whistle plays once. Then mute sound effects on the invitee's profile and invite again: silent.
