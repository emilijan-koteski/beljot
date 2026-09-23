# Audio assets

The in-match sound effects and background music in `client/public/audio/`,
played by `client/src/shared/audio/audioEngine.ts`.

```text
client/public/audio/
  sfx/    12 MP3 (mono, 96 kb/s)   — card-slide-1..8 (card played),
                                     card-shove-1..4 (trick collected)
  music/   3 MP3 (stereo, 96 kb/s) — the rotating playlist
```

Sound effects total ~100 KB (102,713 bytes) and are fetched and decoded when
the match page mounts with sound on, or the moment sound is switched on. The
three music tracks total ~8.3 MB and are streamed one at a time by
an `<audio>` element, so only the playing track is downloaded.

Everything here is **CC0 / public domain**. No attribution is required and the
app deliberately shows none — the sources are recorded here only so the files
can be regenerated and their licence status never has to be re-established.
Nothing that is not CC0 or public domain may be added to this folder.

## When it plays

Two per-account switches, `soundEnabled` and `musicEnabled` (migration `000025`,
both `DEFAULT TRUE`), carried on the auth envelope. A missing value always
means ON (`resolveAudioEnabled`). They are set from the in-match Settings
dialog, the profile sidebar, or the HUD mute button, which flips both at once:
if either is on it turns both off, and if both are off it turns both on.

Each channel also has a volume, `soundVolume` and `musicVolume` (migration
`000026`, `SMALLINT DEFAULT 70`, `CHECK 0–100`), set by a slider under its
switch in the same two places. The switch decides whether a channel plays, the
volume how loud; neither touches the other, so a mute and unmute brings back
the levels the player chose. A missing value means 70 (`resolveAudioVolume`),
the loudness the game had before volumes existed. A slider is disabled — its
value kept — while its switch is off. Dragging the music slider changes the
music live; releasing either slider saves it (one PATCH, optimistic, reverted
silently if it fails), and releasing the sound-effects slider plays one card
sound at the new level. Volume 0 with the switch on is silent; for music it
stops the playlist outright, as the switch would, rather than streaming at zero
gain. A held arrow key saves its first step at once and its last one when the
key is released, not one PATCH per auto-repeat.

- **Card played** — one random `card-slide` per `event:card_played`, from the
  WS dispatcher. The local player's own manual play is the exception: it sounds
  at the click, in step with the optimistic throw, and its server echo is
  skipped. A server auto-play of the local seat sounds from the dispatcher.
- **Trick collected** — one random `card-shove` when the cards leave the table
  (after the winner glow), in both full and reduced motion. It is keyed to the
  resolved-trick snapshot, so a remount mid-collect cannot sound it twice.
- **Music** — only while `MatchPage` is mounted. A random track starts, the
  others follow in order, and it loops round. It fades in over ~1.5 s and out
  over ~0.4 s.
- A resync (`event:match_state`) never makes a sound.
- On a cold load or reload straight onto `/match/:id`, nothing sounds until the
  first pointer or key input on the page (browser autoplay rules). Arriving
  through in-app navigation, the click that got the player there already
  counts, so audio starts at once.

Both buses run through one Web Audio `AudioContext`. At the default volume of
70 the SFX gain is 0.6 and the music gain 0.18; each scales linearly with its
volume (so 100 is ~0.86 / ~0.26, and 0 is silent). The SFX level is read when
each sound starts; a music-volume change glides to the new level over ~0.1 s,
and the fade-in always lands on the current level. The music is routed through
a gain node rather than using the element's `volume` because iOS treats
`HTMLMediaElement.volume` as read-only.

## Why MP3

Safari and iOS before 18.4 cannot `decodeAudioData` Ogg Vorbis, which is what
the Kenney pack ships. MP3 decodes everywhere, so every file is transcoded to
MP3 and no other format is shipped.

## Sound effects — provenance

**Kenney "Casino Audio" 1.1** by Kenney Vleugels (<https://kenney.nl>).

- Download: <https://kenney.nl/media/pages/assets/casino-audio/2472606a04-1721639069/kenney_casino-audio.zip>
  (sha256 `f36250766ac5bc378c13708ddf12a23a8e54a3251f8d482c7536e51b5dbafa18`)
- **Licence: Creative Commons Zero (CC0).** From the zip's `License.txt`: "You
  may use these assets in personal and commercial projects. Credit (Kenney or
  www.kenney.nl) would be nice but is not mandatory."

| Original (in the zip) | Shipped |
| --- | --- |
| `Audio/card-slide-1.ogg` … `Audio/card-slide-8.ogg` | `sfx/card-slide-1.mp3` … `sfx/card-slide-8.mp3` |
| `Audio/card-shove-1.ogg` … `Audio/card-shove-4.ogg` | `sfx/card-shove-1.mp3` … `sfx/card-shove-4.mp3` |

## Music — provenance

**FreePD** (<https://freepd.com>) — every track on it was released into the
public domain under CC0. freepd.com closed in 2025; the files come from the
community mirror at <https://github.com/0lhi/FreePD>, branch `stream`, fetched
as `https://raw.githubusercontent.com/0lhi/FreePD/stream/<path>`.

| Track | Author | Mirror path | sha256 of the download | Shipped |
| --- | --- | --- | --- | --- |
| Lucky Break | Bryan Teoh | `Romance/Lucky Break.mp3` | `ff0ffc4c4c55974804b6a4ca0b3e2dfa63b7c94e61a8532162c82bef23ab1bf8` | `music/lucky-break.mp3` |
| A Good Bass for Gambling | Komiku | `Miscellaneous/A Good Bass for Gambling.mp3` | `dfaea2fa74a35bcbccd64eed8b47a5058f358a2aa1ea976eebaf654344e6ce10` | `music/a-good-bass-for-gambling.mp3` |
| Bass Meant Jazz | Kevin MacLeod | `Miscellaneous/Bass Meant Jazz.mp3` | `c876b42602e23df4e46b1c4a686e4e155d32852ea852b77a2e76fdc0c5e71745` | `music/bass-meant-jazz.mp3` |

The Lucky Break download's own ID3 comment reads "Composed, recorded, and
dedicated to the Public Domain 2020. Nicklas Waroff on sax."

## To regenerate

Neither a project dependency nor an `ffmpeg` install is needed: a static
`ffmpeg` binary is pulled ephemerally through `imageio-ffmpeg`. Download into a scratch
directory, **not** the repo, and run from the repository root:

```sh
SRC=$(mktemp -d)
OUT=client/public/audio
FF=$(uv run --no-project --with imageio-ffmpeg python -c "import imageio_ffmpeg;print(imageio_ffmpeg.get_ffmpeg_exe())")
MIRROR=https://raw.githubusercontent.com/0lhi/FreePD/stream

curl -sSLf -o "$SRC/kenney.zip" \
  "https://kenney.nl/media/pages/assets/casino-audio/2472606a04-1721639069/kenney_casino-audio.zip"
unzip -q -o "$SRC/kenney.zip" -d "$SRC/kenney"
curl -sSLf -o "$SRC/lucky-break.mp3"              "$MIRROR/Romance/Lucky%20Break.mp3"
curl -sSLf -o "$SRC/a-good-bass-for-gambling.mp3" "$MIRROR/Miscellaneous/A%20Good%20Bass%20for%20Gambling.mp3"
curl -sSLf -o "$SRC/bass-meant-jazz.mp3"          "$MIRROR/Miscellaneous/Bass%20Meant%20Jazz.mp3"

mkdir -p "$OUT/sfx" "$OUT/music"

# Sound effects: mono, 96 kb/s, no metadata.
for n in 1 2 3 4 5 6 7 8; do
  "$FF" -hide_banner -loglevel error -y -i "$SRC/kenney/Audio/card-slide-$n.ogg" \
    -map 0:a -map_metadata -1 -ac 1 -ar 44100 -c:a libmp3lame -b:a 96k "$OUT/sfx/card-slide-$n.mp3"
done
for n in 1 2 3 4; do
  "$FF" -hide_banner -loglevel error -y -i "$SRC/kenney/Audio/card-shove-$n.ogg" \
    -map 0:a -map_metadata -1 -ac 1 -ar 44100 -c:a libmp3lame -b:a 96k "$OUT/sfx/card-shove-$n.mp3"
done

# Music: stereo, 96 kb/s, loudness-normalised to -18 LUFS (true peak -1.5 dBTP)
# so the three tracks sit at one level under the same gain.
for t in lucky-break a-good-bass-for-gambling bass-meant-jazz; do
  "$FF" -hide_banner -loglevel error -y -i "$SRC/$t.mp3" \
    -map 0:a -map_metadata -1 -af loudnorm=I=-18:TP=-1.5 -ac 2 -ar 44100 \
    -c:a libmp3lame -b:a 96k "$OUT/music/$t.mp3"
done
```

`-ar 44100` matters for the music: `loudnorm` resamples to 192 kHz internally,
and without it the output would stay there. `-map 0:a` drops any embedded cover
art. The encodes were made with ffmpeg 7.0.2 (the `imageio-ffmpeg` static
build); measured afterwards, the three tracks land at -18.0, -17.9 and
-18.4 LUFS integrated.

`*.mp3` is marked `binary` in `.gitattributes`, so the repo-wide `eol=lf` rule
never touches these files. `audioEngine.test.ts` fails if any of the fifteen
files the engine references goes missing.
