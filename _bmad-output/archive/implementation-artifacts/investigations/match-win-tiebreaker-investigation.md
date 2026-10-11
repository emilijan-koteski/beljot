# Investigation: Match win resolution & 1001-tie tiebreaker

## Hand-off Brief

1. **What happened.** The engine resolves a match winner at hand-end with a three-branch rule (one team over → that team; both over → higher cumulative total; both over **and exactly tied** → the trump-calling team), but all four localized rule texts (mk/hr/en/sr) describe only the first two branches and omit the exact-tie tiebreaker.
2. **Where the case stands.** Implementation behavior is **Confirmed** from source + passing tests; the rules-text gap is **Confirmed** by reading all four content files. Investigation complete (exploration case).
3. **What's needed next.** Decide whether to (a) document the exact-tie tiebreaker in the four rule texts, or (b) treat the current rule text as intentionally simplified — no code change is required either way.

## Case Info

| Field            | Value                                                                |
| ---------------- | -------------------------------------------------------------------- |
| Ticket           | N/A                                                                  |
| Date opened      | 2026-06-06                                                           |
| Status           | Concluded                                                           |
| System           | Go game engine (`server/internal/game`), React rules UI (`client/src/features/rules`) |
| Evidence sources | Source code, co-located Go tests, i18n rule-content files            |

## Problem Statement

How does the system resolve the match winner when one or both teams reach/pass 1001 at the end of a hand, and what is the tiebreaker when both teams pass 1001 with the SAME total (e.g. 1024:1024)? Compare implementation against the localized rule text.

## Confirmed Findings

### Finding 1: Match-end is checked at hand-end, after scoring, before the next hand starts

**Evidence:** `server/internal/game/scoring.go:101-114`

**Detail:** `scoreHand` computes cumulative `TeamScores`, then `target := matchTarget(state.MatchMode)` (1001, or 501 in "501" mode — `scoring.go:233-239`). It sets `aOver = TeamScores[A] >= target`, `bOver = TeamScores[B] >= target`. If either crossed, it calls `determineMatchWinner`, sets `WinnerTeam`, transitions to `PhaseMatchEnd`, and returns **before** `startNewHand`. So `TrumpCallerSeat` is still populated when the winner is computed (it's only cleared inside `startNewHand`, `scoring.go:126`).

### Finding 2: The three-branch winner rule, including the exact-tie tiebreaker

**Evidence:** `server/internal/game/scoring.go:214-231`

**Detail:**
```go
func determineMatchWinner(state *GameState, aOver, bOver bool) int {
	if aOver && bOver {
		if state.TeamScores[TeamA] > state.TeamScores[TeamB] { return TeamA }
		if state.TeamScores[TeamB] > state.TeamScores[TeamA] { return TeamB }
		return TeamForSeat(*state.TrumpCallerSeat) // tied → trump caller's team
	}
	if aOver { return TeamA }
	return TeamB
}
```
- Only one team over → that team wins.
- Both over → **higher cumulative total** wins.
- Both over **and exactly tied** → `TeamForSeat(*state.TrumpCallerSeat)` — the team that called/took trump **in that final hand** wins. This is the real tiebreaker for 1024:1024.

### Finding 3: The exact-tie tiebreaker is intentional and tested

**Evidence:** `server/internal/game/scoring_test.go:330-351` (`TestMatchEnd_BothTeamsExceed1001_TiedScore_ContractingTeamWins`)

**Detail:** Prior totals A=1000, B=978; team B is trump caller and scores 92 vs A's 70 that hand → both reach 1070. Test asserts `*result.WinnerTeam == TeamB` ("contracting team wins tiebreaker"). Sibling tests confirm the other branches: `:303` single team, `:313` both-over-higher-wins.

### Finding 4: An exact cumulative tie is reachable only via asymmetric per-hand split

**Evidence:** Deduced from `scoring.go` failed-contract rule + `scoring_test.go:113-145`.

**Detail:** In a non-failed hand the trump caller must score **strictly more** than the opponents (a within-hand tie is a *failed* hand where the caller gets 0 and all points transfer — `scoring_test.go:120-125`). So both teams can only finish the match exactly equal when the pre-hand gap exactly offsets that hand's asymmetric split (as in the test: 1000/978 + 70/92 = 1070/1070). The tiebreaker exists precisely for this rare-but-reachable case.

### Finding 5: All four localized rule texts omit the exact-tie tiebreaker

**Evidence:** `client/src/features/rules/content/{mk,hr,en,sr}.ts`

**Detail:** Each describes only "more total points wins":
- `mk.ts:209` — "...страната што освоила повеќе вкупен број на поени ја освојува партијата."
- `hr.ts:208` — "...strana s više ukupnih bodova osvaja meč."
- `en.ts:209` — "...the side with more total points takes the match."
- `sr.ts:208` — "...strana sa više ukupnih poena osvaja meč."

None state what happens on an exact tie. A reader cannot derive "trump caller wins" from the rules text.

## Source Code Trace

| Element       | Detail                                                                  |
| ------------- | ----------------------------------------------------------------------- |
| Logic origin  | `server/internal/game/scoring.go:214-231` (`determineMatchWinner`)      |
| Trigger       | `scoreHand` at hand-end, `scoring.go:101-111`                           |
| Condition     | both `aOver && bOver` true and `TeamScores[A] == TeamScores[B]`         |
| Related files | `scoring.go` (`matchTarget`, `scoreHand`), `scoring_test.go:301-351`, rule content `client/src/features/rules/content/*.ts`, `_bmad-output/project-context.md:329` |

## Side Findings

- **`project-context.md:329` is imprecise.** It states "if both teams cross 1001/501 in the same hand, the taker's team (team that called trump) wins — taking trump is the tiebreaker." That reads as if the trump caller *always* wins when both cross; the code actually checks **higher score first** and only falls back to the trump caller on an **exact** tie. (Doc-only nuance; AI-context file, not user-facing.)
- **"Only one team crossed" branch is sound** — if only A crossed, A necessarily has the higher score (a higher B below 1001 is impossible when A ≥ 1001 and B > A would imply B ≥ 1001). No bug.
- **No nil-deref risk** on `*state.TrumpCallerSeat`: the match-end check runs before `startNewHand` clears it, and `scoreHand` only runs after trump was picked.

## Conclusion

**Confidence:** High — root behavior Confirmed from source + green tests; rules-text gap Confirmed by direct read of all four files.

The system resolves the winner at hand-end: one team over → that team; both over → higher cumulative total; both over **and exactly equal** → the **trump-calling team of the final hand** wins. At 1024:1024 the trump caller's team wins. The implementation is correct, deterministic, and tested. The only discrepancy is documentation: the four user-facing rule texts (and, loosely, `project-context.md:329`) do not mention the exact-tie tiebreaker.

## Recommended Next Steps

### Fix direction (documentation only — no engine change)

Add one clause to the win-resolution sentence in each of `client/src/features/rules/content/{mk,hr,en,sr}.ts`, e.g. EN: "...the side with more total points takes the match; if those are exactly level too, the side that called trump that hand wins." Mirror idiomatically in mk (all-Cyrillic), hr, sr per the localization-terminology reference. Note `RulesPage.test.tsx` asserts on rule content — update assertions in the same change.
