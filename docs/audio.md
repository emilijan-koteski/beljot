# Audio assets

The in-match sound effects and background music in `client/public/audio/`,
played by `client/src/shared/audio/audioEngine.ts`.

```text
client/public/audio/
  sfx/    29 MP3 (mono, 96 kb/s)   — card-slide-1..8 (card played),
                                     card-shove-1..4 (trick collected),
                                     card-shuffle (a deal starts),
                                     card-place-1..4 (a deal packet lands),
                                     chips-stack-1..6 (declarations, Belote),
                                     jingle-capot, jingle-win, jingle-lose,
                                     popup-1..2 (avatar popups),
                                     clock-tick (own timer in the red)
  music/   3 MP3 (stereo, 96 kb/s) — the rotating playlist
```

Sound effects total ~210 KB (214,109 bytes) and are fetched and decoded when
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
- **Deal** — `card-shuffle` as a deal starts (the opening deal of a hand,
  including an all-pass reshuffle; not the second deal after a pick), then one
  random `card-place` as each packet lands, in both motion modes. Keyed to the
  deal, so a remount cannot repeat it; a mount into a hand in progress never
  replays a deal at all.
- **Declarations and Belote** — one random `chips-stack` when the declaration
  reveal or the Belote/Rebelote reveal opens.
- **Capot** — `jingle-capot` when the capot banner opens. A banner rebuilt on
  reconnect from the saved hand result stays silent.
- **Match won / lost** — `jingle-win` or `jingle-lose`, for the viewer's own
  team, when the match result opens; after an abandonment, a win for the team
  left at the table and a loss for the abandoner's partner (the abandoner
  hears nothing).
- **Avatar popups** — one random `popup` when an emote bubble appears (only
  when it is shown: not behind the result, a pause or a disconnect), when a
  Bitola "has a declaration" banner appears, and when a surrender proposal
  reaches the partner (prompt) or the opponents (banner). The proposer hears
  nothing.
- **Urgent clock** — `clock-tick` once per whole second, down to 1, while the
  viewer's OWN decision timer is in the countdown ring's red zone (≤ 1/8 of the
  window): their turn, their bid, a Belote or Bitola declaration prompt, and
  the Croatian declaration window while unanswered. It stops at the click.
  Other seats' timers and the auto-close, score-reveal and reconnect rings
  never tick.
- **Music** — only while `MatchPage` is mounted. A random track starts, the
  others follow in order, and it loops round. It fades in over ~1.5 s and out
  over ~0.4 s.
- A resync (`event:match_state`, or a reveal rebuilt from `lastHandResult`)
  never makes a sound. Every one-off sound is dedupe-keyed to the moment it
  marks, so a remount that re-renders the same surface cannot repeat it.
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

All three packs are by Kenney Vleugels (<https://kenney.nl>) and ship the same
licence. **Licence: Creative Commons Zero (CC0).** From each zip's
`License.txt`: "You may use these assets in personal and commercial projects.
Credit (Kenney or www.kenney.nl) would be nice but is not mandatory." (Interface
Sounds words it "This content is free to use in personal, educational and
commercial projects. Support us by crediting Kenney or www.kenney.nl (this is
not mandatory)".)

**Kenney "Casino Audio" 1.1**

- Download: <https://kenney.nl/media/pages/assets/casino-audio/2472606a04-1721639069/kenney_casino-audio.zip>
  (sha256 `f36250766ac5bc378c13708ddf12a23a8e54a3251f8d482c7536e51b5dbafa18`)

**Kenney "Music Jingles"**

- Download: <https://kenney.nl/media/pages/assets/music-jingles/f37e530b9e-1677590399/kenney_music-jingles.zip>
  (sha256 `b729ba57959bd58793d2c5cafa348aaf2655d354f3da35ec4729e03ec77197b8`)

**Kenney "Interface Sounds" 1.0**

- Download: <https://kenney.nl/media/pages/assets/interface-sounds/fa43c1dd4d-1677589452/kenney_interface-sounds.zip>
  (sha256 `f2193d072726d6758a5f7871b2dcc54dcce0d5c35c6f0a62f92549b327c81232`)

| Pack | Original (in the zip) | Shipped |
| --- | --- | --- |
| Casino | `Audio/card-slide-1.ogg` … `Audio/card-slide-8.ogg` | `sfx/card-slide-1.mp3` … `sfx/card-slide-8.mp3` |
| Casino | `Audio/card-shove-1.ogg` … `Audio/card-shove-4.ogg` | `sfx/card-shove-1.mp3` … `sfx/card-shove-4.mp3` |
| Casino | `Audio/card-shuffle.ogg` (first 0.9 s) | `sfx/card-shuffle.mp3` |
| Casino | `Audio/card-place-1.ogg` … `Audio/card-place-4.ogg` | `sfx/card-place-1.mp3` … `sfx/card-place-4.mp3` |
| Casino | `Audio/chips-stack-1.ogg` … `Audio/chips-stack-6.ogg` | `sfx/chips-stack-1.mp3` … `sfx/chips-stack-6.mp3` |
| Jingles | `Audio/Hit jingles/jingles_HIT11.ogg` | `sfx/jingle-capot.mp3` |
| Jingles | `Audio/Sax jingles/jingles_SAX02.ogg` (rising run) | `sfx/jingle-win.mp3` |
| Jingles | `Audio/Sax jingles/jingles_SAX07.ogg` (falling "wah-wah") | `sfx/jingle-lose.mp3` |
| Interface | `Audio/drop_002.ogg`, `Audio/drop_003.ogg` | `sfx/popup-1.mp3`, `sfx/popup-2.mp3` |
| Interface | `Audio/tick_004.ogg` | `sfx/clock-tick.mp3` |

The sax jingles were picked to sit with the jazz playlist. Levels were matched
by measurement against the card sounds, whose loudest 50 ms runs at about −13
to −20 dBFS RMS: the jingles land a little above that, the popup and the tick
a little below — that is all the `volume=` filters in the recipe do.

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
curl -sSLf -o "$SRC/kenney-jingles.zip" \
  "https://kenney.nl/media/pages/assets/music-jingles/f37e530b9e-1677590399/kenney_music-jingles.zip"
unzip -q -o "$SRC/kenney-jingles.zip" -d "$SRC/kenney-jingles"
curl -sSLf -o "$SRC/kenney-interface.zip" \
  "https://kenney.nl/media/pages/assets/interface-sounds/fa43c1dd4d-1677589452/kenney_interface-sounds.zip"
unzip -q -o "$SRC/kenney-interface.zip" -d "$SRC/kenney-interface"
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

# The same encode for every new effect, with a per-sound filter chain.
sfx() { # sfx <input> <filter chain> <output>
  "$FF" -hide_banner -loglevel error -y -i "$1" -af "$2" \
    -map 0:a -map_metadata -1 -ac 1 -ar 44100 -c:a libmp3lame -b:a 96k "$3"
}
# The shuffle: the pack's clip is three riffles over 3 s; the deal wants one
# short one, so keep the first 0.9 s and fade its tail.
sfx "$SRC/kenney/Audio/card-shuffle.ogg" "atrim=0:0.9,afade=t=out:st=0.7:d=0.2,volume=4dB" \
  "$OUT/sfx/card-shuffle.mp3"
# A packet lands: trim the slide-in before the thump so the sound lands with it.
for n in 1 2 3 4; do
  sfx "$SRC/kenney/Audio/card-place-$n.ogg" \
    "silenceremove=start_periods=1:start_threshold=-30dB:start_silence=0.02:detection=rms:window=0.005" \
    "$OUT/sfx/card-place-$n.mp3"
done
for n in 1 2 3 4 5 6; do
  sfx "$SRC/kenney/Audio/chips-stack-$n.ogg" "anull" "$OUT/sfx/chips-stack-$n.mp3"
done
sfx "$SRC/kenney-jingles/Audio/Hit jingles/jingles_HIT11.ogg" "volume=-2dB" "$OUT/sfx/jingle-capot.mp3"
sfx "$SRC/kenney-jingles/Audio/Sax jingles/jingles_SAX02.ogg" "volume=2dB" "$OUT/sfx/jingle-win.mp3"
sfx "$SRC/kenney-jingles/Audio/Sax jingles/jingles_SAX07.ogg" "volume=2dB" "$OUT/sfx/jingle-lose.mp3"
sfx "$SRC/kenney-interface/Audio/drop_002.ogg" "volume=-4dB" "$OUT/sfx/popup-1.mp3"
sfx "$SRC/kenney-interface/Audio/drop_003.ogg" "volume=-4dB" "$OUT/sfx/popup-2.mp3"
sfx "$SRC/kenney-interface/Audio/tick_004.ogg" "volume=-4dB" "$OUT/sfx/clock-tick.mp3"

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
never touches these files. `audioEngine.test.ts` fails if any of the 32 files
the engine references goes missing.
