package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
)

func uid(v uint) *uint { return &v }
func team(v int) *int  { return &v }

var t0 = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

// botTable is user `human` at seat 0 with a bot partner, against two bots.
func botTable(id uint, human uint, winner int, a, b int) match.Match {
	return match.Match{
		ID: id, Status: "completed", MatchMode: "1001",
		Player1ID: uid(human), Player2IsBot: true, Player3IsBot: true, Player4IsBot: true,
		TeamAScore: a, TeamBScore: b, WinnerTeam: winner,
		CompletedAt: t0.Add(time.Duration(id) * time.Minute),
	}
}

// fourHumans seats the users at seats 0-3 (team A is seats 0 and 2).
func fourHumans(id uint, seats [4]uint, winner int, a, b int) match.Match {
	return match.Match{
		ID: id, Status: "completed", MatchMode: "1001",
		Player1ID: uid(seats[0]), Player2ID: uid(seats[1]), Player3ID: uid(seats[2]), Player4ID: uid(seats[3]),
		TeamAScore: a, TeamBScore: b, WinnerTeam: winner,
		CompletedAt: t0.Add(time.Duration(id) * time.Minute),
	}
}

func TestParseFlags_Defaults(t *testing.T) {
	opts, err := parseFlags(nil, io.Discard)
	require.NoError(t, err)

	assert.Equal(t, "", opts.dbURL)
	assert.Equal(t, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), opts.from, "the window defaults to 2026 Q3")
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), opts.to)

	assert.Equal(t, season.DefaultLadder(), opts.ladder, "the live ladder unless --floors says otherwise")

	def := season.DefaultSPFormula()
	assert.Equal(t, def.BaseWin, opts.f.BaseWin)
	assert.Equal(t, def.BaseLoss, opts.f.BaseLoss)
	assert.Equal(t, def.Scale, opts.f.Scale)
	assert.Equal(t, def.CapotBonus, opts.f.CapotBonus)
	assert.Equal(t, 600, opts.f.BotSeatSP, "a bot seat is the replayed ladder's Gold floor")
	assert.Equal(t, 20, opts.sweep.AfterN)
}

func TestParseFlags_Overrides(t *testing.T) {
	opts, err := parseFlags([]string{
		"--db-url", "postgres://example/db",
		"--from", "2026-08-01T00:00:00Z", "--to", "2026-09-01T00:00:00Z",
		"--base-win", "36", "--base-loss", "24", "--scale", "500", "--capot-bonus", "0",
		"--floors", "0,100,200,500,700,900,1100,1300",
		"--sweep-rates", "0.6,0.7", "--sweep-margin", "1.1", "--sweep-after", "30",
	}, io.Discard)
	require.NoError(t, err)

	assert.Equal(t, "postgres://example/db", opts.dbURL)
	assert.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), opts.from)
	assert.Equal(t, season.SPFormula{BaseWin: 36, BaseLoss: 24, Scale: 500, BotSeatSP: 500, CapotBonus: 0}, opts.f)
	assert.Equal(t, []float64{0.6, 0.7}, opts.sweep.WinRates)
	assert.Equal(t, 1.1, opts.sweep.FixedMargin)
	assert.Equal(t, 30, opts.sweep.AfterN)

	opts, err = parseFlags([]string{"--bot-sp", "750"}, io.Discard)
	require.NoError(t, err)
	assert.Equal(t, 750, opts.f.BotSeatSP, "--bot-sp overrides the Gold floor")
}

func TestParseFlags_Rejects(t *testing.T) {
	for _, args := range [][]string{
		{"--from", "yesterday"},
		{"--from", "2026-09-01T00:00:00Z", "--to", "2026-08-01T00:00:00Z"},
		{"--floors", "0,100,200"},
		{"--floors", "0,100,100,500,700,900,1100,1300"},
		{"--floors", "0,a,200,500,700,900,1100,1300"},
		{"--scale", "0"},
		{"--sweep-rates", "0.5,1"},
		{"--no-such-flag"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, err := parseFlags(args, io.Discard)
			assert.Error(t, err)
		})
	}
}

func TestOutcomeFor(t *testing.T) {
	t.Run("natural finish", func(t *testing.T) {
		m := fourHumans(1, [4]uint{1, 2, 3, 4}, 0, 1100, 700)
		o, err := outcomeFor(m)
		require.NoError(t, err)
		assert.Equal(t, match.MatchOutcome{
			Seats: [4]match.OutcomeSeat{
				{UserID: 1, Team: 0, Completed: true},
				{UserID: 2, Team: 1, Completed: true},
				{UserID: 3, Team: 0, Completed: true},
				{UserID: 4, Team: 1, Completed: true},
			},
			WinnerTeam: 0, TeamScores: [2]int{1100, 700}, Target: 1001, AbandonedSeat: -1,
		}, o)
	})

	t.Run("bot seats carry no user", func(t *testing.T) {
		o, err := outcomeFor(botTable(1, 7, 0, 1100, 700))
		require.NoError(t, err)
		assert.Equal(t, match.OutcomeSeat{UserID: 7, Team: 0, Completed: true}, o.Seats[0])
		for seat := 1; seat < 4; seat++ {
			assert.True(t, o.Seats[seat].IsBot)
			assert.Zero(t, o.Seats[seat].UserID)
			assert.Equal(t, seat%2, o.Seats[seat].Team)
		}
	})

	t.Run("surrender: surrendered_by set and the winners below the target", func(t *testing.T) {
		m := fourHumans(1, [4]uint{1, 2, 3, 4}, 1, 300, 500)
		m.SurrenderedBy = uid(1)
		o, err := outcomeFor(m)
		require.NoError(t, err)
		assert.True(t, o.Surrender)
		assert.False(t, o.InstantWin)
	})

	t.Run("a surrender that finalized at the target is a natural finish", func(t *testing.T) {
		m := fourHumans(1, [4]uint{1, 2, 3, 4}, 1, 640, 1012)
		m.SurrenderedBy = uid(1)
		o, err := outcomeFor(m)
		require.NoError(t, err)
		assert.False(t, o.Surrender)
		assert.False(t, o.InstantWin)
	})

	t.Run("instant win: completed, no surrender, winners below the target", func(t *testing.T) {
		o, err := outcomeFor(fourHumans(1, [4]uint{1, 2, 3, 4}, 1, 0, 0))
		require.NoError(t, err)
		assert.True(t, o.InstantWin)
		assert.False(t, o.Surrender)
	})

	t.Run("the 501 target", func(t *testing.T) {
		m := fourHumans(1, [4]uint{1, 2, 3, 4}, 0, 520, 200)
		m.MatchMode = "501"
		o, err := outcomeFor(m)
		require.NoError(t, err)
		assert.Equal(t, 501, o.Target)
		assert.False(t, o.InstantWin, "520 reached the 501 target")
	})

	t.Run("abandonment: the abandoner's seat, and the other team wins", func(t *testing.T) {
		m := fourHumans(1, [4]uint{1, 2, 3, 4}, 0, 200, 400)
		m.Status = "abandoned"
		m.AbandonedBy = uid(3)
		o, err := outcomeFor(m)
		require.NoError(t, err)
		assert.Equal(t, 2, o.AbandonedSeat)
		assert.False(t, o.Seats[2].Completed)
		assert.Equal(t, 1, o.WinnerTeam, "the winner is the non-abandoning team whatever winner_team says")
		assert.False(t, o.Surrender, "the formula scores an abandonment as a surrender itself")
		assert.False(t, o.InstantWin, "an abandonment below the target is not an instant win")
	})

	t.Run("Capot teams come from the hand rows", func(t *testing.T) {
		m := fourHumans(1, [4]uint{1, 2, 3, 4}, 0, 1100, 700)
		m.Hands = []match.HandResult{
			{HandNumber: 1, Capot: true, CapotTeam: team(1)},
			{HandNumber: 2},
			{HandNumber: 3, Capot: true, CapotTeam: team(1)},
			{HandNumber: 4, Capot: true},
		}
		o, err := outcomeFor(m)
		require.NoError(t, err)
		assert.Equal(t, [2]bool{false, true}, o.CapotTeams)
	})

	t.Run("skips", func(t *testing.T) {
		placeholder := fourHumans(1, [4]uint{1, 2, 3, 4}, 0, 200, 400)
		placeholder.Status = "abandoned"
		_, err := outcomeFor(placeholder)
		assert.ErrorIs(t, err, errReconcilePlaceholder)

		unknown := fourHumans(1, [4]uint{1, 2, 3, 4}, 0, 1100, 700)
		unknown.MatchMode = "classic"
		_, err = outcomeFor(unknown)
		assert.ErrorIs(t, err, errUnknownMode)

		stranger := fourHumans(1, [4]uint{1, 2, 3, 4}, 0, 200, 400)
		stranger.Status = "abandoned"
		stranger.AbandonedBy = uid(99)
		_, err = outcomeFor(stranger)
		assert.ErrorIs(t, err, errAbandonerNotSeated)
	})
}

// A short window worked by hand with W=30, L=20, S=430 and a bot seat on 600,
// on a test ladder whose Silver floor is 40.
func TestReplay(t *testing.T) {
	f := season.SPFormula{BaseWin: 30, BaseLoss: 20, Scale: 430, BotSeatSP: 600, CapotBonus: 5}
	ladder, err := season.NewLadder([]int{0, 20, 40, 600, 800, 1000, 1200, 1400})
	require.NoError(t, err)

	// 1. Bot table, user 1 wins 1100:700 from 0 SP. Own avg 300 vs 600, E = 0.167:
	//    +45. The bots' team made a Capot, which pays nobody.
	m1 := botTable(1, 1, 0, 1100, 700)
	m1.Hands = []match.HandResult{{HandNumber: 1, Capot: true, CapotTeam: team(1)}}
	// 2. Four humans, users 1 and 3 (avg 22.5) surrender to 2 and 4 (avg 0) at
	//    300:500. m = 1.2003, E_winner = 0.470: winners +38, losers -25, floored.
	m2 := fourHumans(2, [4]uint{1, 2, 3, 4}, 1, 300, 500)
	m2.SurrenderedBy = uid(1)
	// 3. A boot-reconcile placeholder: skipped.
	m3 := fourHumans(3, [4]uint{1, 2, 3, 4}, 0, 100, 100)
	m3.Status = "abandoned"
	// 4. 501, users 2 and 4 (avg 38) against user 1 and a bot (avg 310); user 4
	//    abandons at 200:400. m = 1.1008, E_winner = 0.811: user 1 +12, user 2
	//    half of -8.32 = -4, user 4 -120.
	m4 := match.Match{
		ID: 4, Status: "abandoned", MatchMode: "501", AbandonedBy: uid(4),
		Player1ID: uid(2), Player2ID: uid(1), Player3ID: uid(4), Player4IsBot: true,
		TeamAScore: 200, TeamBScore: 400, WinnerTeam: 1, CompletedAt: t0.Add(4 * time.Minute),
	}
	// 5. An unknown mode: skipped.
	m5 := fourHumans(5, [4]uint{1, 2, 3, 4}, 0, 1100, 700)
	m5.MatchMode = "301"

	names := map[uint]string{1: "alice", 2: "bob", 3: "carol"}
	players, sum, err := replay([]match.Match{m1, m2, m3, m4, m5}, names, f, ladder, 0)
	require.NoError(t, err)

	assert.Equal(t, []playerResult{
		{UserID: 2, Name: "bob", Matches: 2, Wins: 1, FinalSP: 34, Tier: "bronze", Division: 3},
		{UserID: 1, Name: "alice", Matches: 3, Wins: 2, BotMatches: 1, BotWins: 1, FinalSP: 32, Tier: "bronze", Division: 2, SilverAt: 1},
		{UserID: 4, Name: "", Matches: 2, Wins: 1, FinalSP: 0, Tier: "iron", Division: 1},
		{UserID: 3, Name: "carol", Matches: 1, Wins: 0, FinalSP: 0, Tier: "iron", Division: 1},
	}, players, "sorted by final SP, then matches; user 2 peaked at 38 and never reached Silver")

	assert.Equal(t, replaySummary{
		Loaded: 5, Scored: 3, Surrenders: 1, Abandonments: 1, CapotMatches: 1, BotOnly: 1,
		Skipped: map[string]int{
			errReconcilePlaceholder.Error(): 1,
			errUnknownMode.Error():          1,
		},
	}, sum)

	// --trace-user: the same replay, with user 1's match-by-match record.
	_, traced, err := replay([]match.Match{m1, m2, m3, m4, m5}, names, f, ladder, 1)
	require.NoError(t, err)
	assert.Equal(t, []traceRow{
		{N: 1, MatchID: 1, At: m1.CompletedAt, BotOnly: true, Won: true, Before: 0, Change: 45, After: 45},
		{N: 2, MatchID: 2, At: m2.CompletedAt, Surrender: true, Before: 45, Change: -25, After: 20},
		{N: 3, MatchID: 4, At: m4.CompletedAt, Won: true, Before: 20, Change: 12, After: 32},
	}, traced.Trace, "skipped matches leave no trace row")

	// A floored loss traces the APPLIED change: user 4's abandonment computes to
	// -120 but only 38 SP were there to lose.
	_, traced, err = replay([]match.Match{m1, m2, m3, m4, m5}, names, f, ladder, 4)
	require.NoError(t, err)
	require.Len(t, traced.Trace, 2)
	last := traced.Trace[1]
	assert.Equal(t, traceRow{N: 2, MatchID: 4, At: m4.CompletedAt, Abandoned: true, Before: 38, Change: -38, After: 0}, last)
	assert.Equal(t, last.After, last.Before+last.Change)
}

func TestPrintReplay_SkipReasonsInSortedOrder(t *testing.T) {
	opts := defaultOptions(t)
	var out bytes.Buffer
	printReplay(&out, opts, nil, replaySummary{Skipped: map[string]int{"zeta": 1, "alpha": 2, "mid": 3}})
	text := out.String()
	a, m, z := strings.Index(text, "alpha"), strings.Index(text, "mid"), strings.Index(text, "zeta")
	require.True(t, a >= 0 && m >= 0 && z >= 0)
	assert.True(t, a < m && m < z, "skip reasons print in sorted order, not map order")
}

func TestReplay_IsTheAwardPathArithmetic(t *testing.T) {
	// Replaying one match must give exactly ComputeSPChanges-then-floor for every
	// seat: the replay is only as useful as its agreement with the live formula.
	f := season.DefaultSPFormula()
	m := fourHumans(1, [4]uint{1, 2, 3, 4}, 0, 1100, 700)
	o, err := outcomeFor(m)
	require.NoError(t, err)
	want, err := f.Changes(o, map[uint]int{})
	require.NoError(t, err)

	players, _, err := replay([]match.Match{m}, nil, f, season.DefaultLadder(), 0)
	require.NoError(t, err)
	for _, p := range players {
		assert.Equal(t, season.ApplySPChange(0, want[p.UserID]), p.FinalSP, "user %d", p.UserID)
	}
}

func TestTemplatePools_RelabelToTheHumansSide(t *testing.T) {
	// The human sits at seat 1 (team B) and wins 1100 to 700 with a Capot.
	won := match.Match{
		ID: 1, Status: "completed", MatchMode: "1001",
		Player1IsBot: true, Player2ID: uid(5), Player3IsBot: true, Player4IsBot: true,
		TeamAScore: 700, TeamBScore: 1100, WinnerTeam: 1,
		Hands: []match.HandResult{{HandNumber: 1, Capot: true, CapotTeam: team(1)}},
	}
	lost := botTable(2, 5, 1, 400, 1010)
	surrendered := botTable(3, 5, 1, 300, 500)
	surrendered.SurrenderedBy = uid(5)
	abandoned := botTable(4, 5, 1, 300, 500)
	abandoned.Status = "abandoned"
	abandoned.AbandonedBy = uid(5)
	notBotOnly := fourHumans(5, [4]uint{5, 6, 7, 8}, 0, 1100, 700)

	w, l := templatePools([]match.Match{won, lost, surrendered, abandoned, notBotOnly})
	assert.Equal(t, []outcomeTemplate{
		{scores: [2]int{1100, 700}, target: 1001, capot: [2]bool{true, false}},
	}, w)
	assert.Equal(t, []outcomeTemplate{
		{scores: [2]int{400, 1010}, target: 1001},
		{scores: [2]int{300, 500}, target: 1001, surrender: true},
	}, l, "abandonments and multi-human tables stay out of the pools")
}

func defaultOptions(t *testing.T) options {
	t.Helper()
	opts, err := parseFlags(nil, io.Discard)
	require.NoError(t, err)
	return opts
}

func TestSweep(t *testing.T) {
	opts := defaultOptions(t)
	cfg := opts.sweep
	cfg.Runs = 200
	cfg.Horizon = 600
	cfg.FixedMargin = 0.84

	rows, err := sweep(cfg, opts.f, opts.ladder)
	require.NoError(t, err)
	require.Len(t, rows, len(cfg.WinRates))

	again, err := sweep(cfg, opts.f, opts.ladder)
	require.NoError(t, err)
	assert.Equal(t, rows, again, "the same seed gives the same sweep")

	for i, r := range rows {
		want := float64(opts.f.BotSeatSP) + 2*opts.f.Scale*math.Log10(r.P*opts.f.BaseWin/((1-r.P)*opts.f.BaseLoss))
		assert.InDelta(t, want, r.AnalyticSettle, 1e-9)
		// The simulated settle point tracks the analytic one closely; the 0 floor
		// and rounding only nudge it.
		assert.InDelta(t, r.AnalyticSettle, r.SimSettle, 25, "p=%.2f", r.P)
		if i > 0 {
			assert.Greater(t, r.SimSettle, rows[i-1].SimSettle, "a better win rate settles higher")
			assert.GreaterOrEqual(t, r.AfterNMedian, rows[i-1].AfterNMedian)
		}
		assert.LessOrEqual(t, r.AfterNP10, r.AfterNMedian)
		assert.LessOrEqual(t, r.AfterNMedian, r.AfterNP90)
	}

	// With equal win and loss margins the settle points sit on the analytic
	// line, which at the approved S = 385 is below the tuning targets: those were
	// met on Q3's REAL margins, where wins are wider than losses (see
	// TestSweep_WiderWinMarginsSettleHigher). What holds at any margin is the
	// bottom of the ladder.
	byP := map[string]sweepRow{}
	for _, r := range rows {
		byP[fmt.Sprintf("%.2f", r.P)] = r
	}
	got, _ := opts.ladder.Rank(int(byP["0.50"].AnalyticSettle))
	assert.Equal(t, "gold", got, "an even player settles in Gold")
	assert.LessOrEqual(t, byP["0.50"].SilverMedian, 21, "a 50% player reaches Silver in about 20 matches")
}

// Real wins are by wider margins than real losses, which is why S was tuned on
// sampled margins rather than on the equal-margin line: the same win rate
// settles higher when its wins pay on a wider margin than its losses cost.
func TestSweep_WiderWinMarginsSettleHigher(t *testing.T) {
	opts := defaultOptions(t)
	cfg := opts.sweep
	cfg.WinRates = []float64{0.85}
	cfg.Runs, cfg.Horizon, cfg.AfterN = 100, 600, 20

	cfg.FixedMargin = 0.84
	even, err := sweep(cfg, opts.f, opts.ladder)
	require.NoError(t, err)

	cfg.FixedMargin = 0
	cfg.Won = []outcomeTemplate{fixedTemplate(1.0, true)}
	cfg.Lost = []outcomeTemplate{fixedTemplate(0.7, false)}
	wide, err := sweep(cfg, opts.f, opts.ladder)
	require.NoError(t, err)

	assert.Greater(t, wide[0].SimSettle, even[0].SimSettle+50)
	gm, _ := opts.ladder.Floor(season.TierGrandmaster)
	assert.Greater(t, wide[0].SimSettle, float64(gm), "85% with wins at 1.0 and losses at 0.7 clears the Grandmaster floor")
}

func TestSweep_SamplesThePools(t *testing.T) {
	opts := defaultOptions(t)
	cfg := opts.sweep
	cfg.WinRates = []float64{0.5}
	cfg.Runs, cfg.Horizon, cfg.AfterN = 50, 40, 20
	// Every win is a whitewash (m = 1.5) and every loss the narrowest (m = 0.5),
	// so the sampled sweep must climb faster than the fixed-margin one.
	cfg.Won = []outcomeTemplate{{scores: [2]int{1001, 0}, target: 1001}}
	cfg.Lost = []outcomeTemplate{{scores: [2]int{1001, 1001}, target: 1001}}
	sampled, err := sweep(cfg, opts.f, opts.ladder)
	require.NoError(t, err)

	cfg.FixedMargin = 0.84
	fixed, err := sweep(cfg, opts.f, opts.ladder)
	require.NoError(t, err)

	assert.Greater(t, sampled[0].AfterNMedian, fixed[0].AfterNMedian)
}

func TestSweep_RejectsABadConfig(t *testing.T) {
	opts := defaultOptions(t)
	for _, mutate := range []func(c *sweepConfig){
		func(c *sweepConfig) { c.Runs = 0 },
		func(c *sweepConfig) { c.AfterN = 0 },
		func(c *sweepConfig) { c.AfterN = c.Horizon + 1 },
	} {
		cfg := opts.sweep
		mutate(&cfg)
		_, err := sweep(cfg, opts.f, opts.ladder)
		assert.Error(t, err)
	}
}

func TestRun_SweepOnlyWithoutADatabase(t *testing.T) {
	opts := defaultOptions(t)
	opts.sweep.Runs = 20
	opts.sweep.Horizon = 100

	var out bytes.Buffer
	require.NoError(t, run(opts, &out))
	text := out.String()
	assert.Contains(t, text, "W (base win) 30")
	assert.Contains(t, text, "abandon penalty -120   worst possible loss -60   best possible win +95",
		"the best win includes the +5 Capot bonus")
	assert.Contains(t, text, "Synthetic sweep")
	assert.NotContains(t, text, "Replay", "no replay table without a source")
}

// testDSN is the integration database, or "" to skip. There is deliberately no
// fallback: these tests run only against the database named in BELJOT_DB_URL.
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("BELJOT_DB_URL")
	if dsn == "" {
		t.Skip("skipping integration test: BELJOT_DB_URL not set")
	}
	return dsn
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(testDSN(t)), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Skip("skipping integration test: database not available")
	}
	if sqlDB, err := db.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return db
}

func TestReadOnly_PostgresRefusesWrites(t *testing.T) {
	db := openTestDB(t)

	err := readOnly(db, func(tx *gorm.DB) error {
		return tx.Exec("CREATE TEMP TABLE sptune_write_probe (i int)").Error
	})
	require.Error(t, err, "a READ ONLY transaction refuses even a temp table")
	assert.Contains(t, err.Error(), "read-only")

	sentinel := errors.New("from fn")
	assert.ErrorIs(t, readOnly(db, func(*gorm.DB) error { return sentinel }), sentinel)
}

func TestLoadWindow(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { tx.Rollback() })

	suffix := fmt.Sprintf("%08d", time.Now().UnixNano()%1e8)
	seedUser := func(tag string) uint {
		var id uint
		require.NoError(t, tx.Raw(`
INSERT INTO users (email, username, password_hash, language_preference)
VALUES (?, ?, 'x', 'en') RETURNING id`, tag+suffix+"@sptune.test", tag+suffix).Scan(&id).Error)
		return id
	}
	alice, bob := seedUser("spa"), seedUser("spb")
	require.NoError(t, tx.Exec(`UPDATE users SET deleted_at = now() WHERE id = ?`, bob).Error)

	var roomID uint
	require.NoError(t, tx.Raw(`
INSERT INTO rooms (name, code, owner_id, status) VALUES (?, ?, ?, 'completed') RETURNING id`,
		"sptune-"+suffix, "S"+suffix[len(suffix)-5:], alice).Scan(&roomID).Error)

	from := time.Date(2031, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 3, 0)
	insert := func(completedAt time.Time, status string, withHands bool) uint {
		m := match.Match{
			RoomID: roomID, Player1ID: uid(alice), Player2ID: uid(bob),
			Player3IsBot: true, Player4IsBot: true, HasBots: true,
			TeamAScore: 1100, TeamBScore: 700, WinnerTeam: 0, Variant: "bitola", MatchMode: "1001",
			StartedAt: completedAt.Add(-time.Hour), CompletedAt: completedAt, Status: status,
		}
		require.NoError(t, tx.Create(&m).Error)
		if withHands {
			for n, capot := range []*int{nil, team(1)} {
				require.NoError(t, tx.Create(&match.HandResult{
					MatchID: m.ID, HandNumber: 2 - n, Capot: capot != nil, CapotTeam: capot,
				}).Error)
			}
		}
		return m.ID
	}
	second := insert(from.Add(48*time.Hour), "completed", true)
	first := insert(from, "completed", false)
	tieA := insert(from.Add(72*time.Hour), "abandoned", false)
	tieB := insert(from.Add(72*time.Hour), "completed", false)
	insert(from.Add(-time.Second), "completed", false) // before the window
	insert(to, "completed", false)                     // the end is exclusive
	insert(from.Add(time.Hour), "in_progress", false)  // not finished

	matches, names, err := loadWindow(tx, from, to)
	require.NoError(t, err)

	ids := make([]uint, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.ID)
	}
	assert.Equal(t, []uint{first, second, min(tieA, tieB), max(tieA, tieB)}, ids,
		"completed_at order, id breaking ties, finished rows inside [from, to) only")

	require.Len(t, matches[1].Hands, 2)
	assert.Equal(t, 1, matches[1].Hands[0].HandNumber, "hands in hand order")
	o, err := outcomeFor(matches[1])
	require.NoError(t, err)
	assert.Equal(t, [2]bool{false, true}, o.CapotTeams)

	assert.Equal(t, "spa"+suffix, names[alice])
	assert.Equal(t, "spb"+suffix+" (deleted)", names[bob], "a deleted account is labelled, not dropped")
}
