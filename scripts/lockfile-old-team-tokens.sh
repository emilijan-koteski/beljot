#!/usr/bin/env bash
# scripts/lockfile-old-team-tokens.sh
#
# CI lockfile — fails the build if any of the old red/blue team identifiers
# are reintroduced after the teamA/teamB rename. Wired into
# `.github/workflows/ci.yml` as a step before `Lint` so the gate runs on
# every PR and push.
#
# Forbidden token regex copied verbatim from AC-001 of the spec
# `_bmad-output/archive/implementation-artifacts/spec-team-us-them-and-cross-seat-layout.md`.
# Do NOT edit the regex without renegotiating the frozen-after-approval
# spec section.
#
# Scope: client/src, server/internal, server/migrations. Excludes .git,
# node_modules, _bmad-output, dist, __snapshots__ (snapshots regenerate
# automatically; not the regression target).
#
# Local verification: deliberately add `// team_red` to any TS file under
# client/src and rerun — exit code must be 1, with the offending line
# printed.

set -euo pipefail

# Pattern verbatim from AC-001 of the spec. Captures: teamRed/teamBlue
# (kebab + snake variants), TeamRed/TeamBlue, red*Points / blue*Points
# combinations, redScore/blueScore, redHandTotal/blueHandTotal,
# team_red_score / team_blue_score, --color-team-red/blue,
# text-team-red/blue, border-team-red/blue, bg-team-red/blue.
#
# Cleanup-pass extension (2026-05-01): also catch the prose-context
# "Red"/"Blue" team-naming patterns reintroduced in comments and assertion
# messages (the originals say `Red team`, `seat N (Red)`, `team red`, etc.).
# Tailwind suit colors and the Serbian word "red" (turn/order) are
# unaffected because they don't match these phrases.
#
# Compound-name tightening (2026-05-01): the prior pass kept bare `a`/`b`
# in compound identifiers (`a_card_points`, `aHandTotal`, `ACardPoints`,
# `aScore`, etc.). Winston's grep-ability rule requires the `team_`/`team`/
# `Team` prefix on compound names too — append those bare forms to the
# forbidden set so they can't be reintroduced. Word boundaries (\b) keep
# unrelated names that happen to start with `a` / `b` clear.
PATTERN='team[_-]?(red|blue)|[Tt]eam(Red|Blue)|red(Card|Decl|Hand|Match|Final)Points?|red(Score)|blue(Card|Decl|Hand|Match|Final)Points?|blue(Score)|red(Hand|Match)Total|blue(Hand|Match)Total|red_(card|decl|hand)_(points|total)|blue_(card|decl|hand)_(points|total)|team_(red|blue)_score|--color-team-(red|blue)|text-team-(red|blue)|border-team-(red|blue)|bg-team-(red|blue)|(Red|Blue) (team|seat)|seat [0-9]+ \((Red|Blue)\)|team [Rr]ed\b|team [Bb]lue\b|\b(a|b)_(card|decl|hand)_(points|total)\b|\b(a|b)(Card|Decl|Hand|Match|Final)Points?\b|\b(a|b)(Hand|Match|Final)(Total|Score)\b|\b(A|B)(Card|Decl|Hand|Match|Final)Points?\b|\b(A|B)(Hand|Match|Final)(Total|Score)\b'

# Resolve repo root from this script's location so the script works from
# any CWD (CI runs it from repo root; a contributor might run from
# scripts/).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Search scope — directories that should never contain the forbidden
# identifiers. Each path is verified to exist; missing paths are skipped
# (no false positive on a fresh checkout).
SCOPE_DIRS=(
  "$REPO_ROOT/client/src"
  "$REPO_ROOT/server/internal"
  "$REPO_ROOT/server/migrations"
)
EXISTING_SCOPE=()
for d in "${SCOPE_DIRS[@]}"; do
  if [[ -d "$d" ]]; then
    EXISTING_SCOPE+=("$d")
  fi
done

if [[ ${#EXISTING_SCOPE[@]} -eq 0 ]]; then
  echo "lockfile: no scope directories found — nothing to scan." >&2
  exit 0
fi

# -r recursive, -I ignore binaries, -E extended regex, -n line numbers,
# --color=never so CI logs don't get ANSI-escape pollution.
# Excludes: .git, node_modules, _bmad-output, dist, __snapshots__.
# This is a self-contained lockfile script — script intentionally references
# the lookup pattern itself; it would otherwise self-trigger. Skip this
# file via --exclude.
set +e
MATCHES=$(grep -rIEn --color=never \
  --exclude-dir=.git \
  --exclude-dir=node_modules \
  --exclude-dir=_bmad-output \
  --exclude-dir=dist \
  --exclude-dir=__snapshots__ \
  --exclude="lockfile-old-team-tokens.sh" \
  "$PATTERN" \
  "${EXISTING_SCOPE[@]}")
GREP_EXIT=$?
set -e

# grep exits 0 = matched, 1 = no match, >1 = error.
if [[ $GREP_EXIT -eq 1 ]]; then
  echo "lockfile: clean — no forbidden old-team tokens found."
  exit 0
fi
if [[ $GREP_EXIT -gt 1 ]]; then
  echo "lockfile: grep failed with exit $GREP_EXIT" >&2
  exit "$GREP_EXIT"
fi

# Matches found — print and fail.
echo "lockfile: forbidden old-team-name tokens detected — refusing to merge." >&2
echo "" >&2
echo "$MATCHES" >&2
echo "" >&2
echo "These identifiers were renamed during the teamA/teamB rename. Either" >&2
echo "rename the new occurrence, or — if the rename is being intentionally" >&2
echo "reverted — renegotiate AC-001 in the team-rename spec first." >&2
exit 1
