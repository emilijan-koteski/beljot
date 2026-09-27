// Command sptune replays stored matches through the competitive Season Points
// formula, in memory, to tune its constants (Story 13.4). It is an OFFLINE tool:
// it never writes to any database (every read runs in a READ ONLY transaction)
// and it is never built into the production image, which builds ./cmd/api only.
//
//	go run ./cmd/sptune --db-url <replay source> [--from ...] [--to ...] [overrides]
//
// It prints the constants in force, a per-player table for the window (matches,
// win %, bot-only matches and win %, final SP, tier, division, and the match at
// which the player first reached Silver), and a synthetic bot-table sweep that
// gives each win rate's settle point and its SP after the first N matches.
// Without --db-url only the sweep runs, at a fixed margin.
//
// The replay itself is season.ReplayMatches, the same core the server's
// one-time season recalculation writes its rows from (season/recalc.go).
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
)

type options struct {
	dbURL  string
	from   time.Time
	to     time.Time
	f      season.SPFormula
	ladder season.Ladder
	sweep  sweepConfig
	trace  uint
}

func main() {
	opts, err := parseFlags(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "sptune:", err)
		os.Exit(2)
	}
	if err := run(opts, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "sptune:", err)
		os.Exit(1)
	}
}

func parseFlags(args []string, errOut io.Writer) (options, error) {
	def := season.DefaultSPFormula()
	fs := flag.NewFlagSet("sptune", flag.ContinueOnError)
	fs.SetOutput(errOut)

	dbURL := fs.String("db-url", "", "Postgres URL of the replay source, read only (empty: sweep only)")
	from := fs.String("from", "2026-07-01T00:00:00Z", "window start, inclusive (RFC 3339)")
	to := fs.String("to", "2026-10-01T00:00:00Z", "window end, exclusive (RFC 3339)")
	baseWin := fs.Float64("base-win", def.BaseWin, "W, the win scale")
	baseLoss := fs.Float64("base-loss", def.BaseLoss, "L, the loss scale")
	scale := fs.Float64("scale", def.Scale, "S, the expected-result scale")
	capot := fs.Int("capot-bonus", def.CapotBonus, "flat bonus for a team that made a Capot")
	floors := fs.String("floors", "", "eight tier floors, Iron to Grandmaster, comma separated (empty: the live ladder)")
	botSP := fs.Int("bot-sp", -1, "SP a bot seat counts as (-1: the ladder's Gold floor)")
	rates := fs.String("sweep-rates", "0.5,0.55,0.6,0.65,0.7,0.75,0.8,0.85,0.9", "win rates for the synthetic sweep")
	runs := fs.Int("sweep-runs", 400, "synthetic runs per win rate")
	horizon := fs.Int("sweep-horizon", 600, "matches per synthetic run; the settle point averages its second half")
	afterN := fs.Int("sweep-after", 20, "report each win rate's SP after this many matches")
	margin := fs.Float64("sweep-margin", 0, "fixed margin for the sweep (0: sample the window's bot-table matches)")
	seed := fs.Uint64("seed", 1, "random seed for the sweep")
	trace := fs.Uint("trace-user", 0, "print this user's match-by-match replay (0: none)")

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	var opts options
	var err error
	opts.dbURL = *dbURL
	if opts.from, err = time.Parse(time.RFC3339, *from); err != nil {
		return options{}, fmt.Errorf("--from: %w", err)
	}
	if opts.to, err = time.Parse(time.RFC3339, *to); err != nil {
		return options{}, fmt.Errorf("--to: %w", err)
	}
	if !opts.to.After(opts.from) {
		return options{}, errors.New("--to must be after --from")
	}

	opts.ladder = season.DefaultLadder()
	if *floors != "" {
		floorValues, err := parseInts(*floors)
		if err != nil {
			return options{}, fmt.Errorf("--floors: %w", err)
		}
		if opts.ladder, err = season.NewLadder(floorValues); err != nil {
			return options{}, fmt.Errorf("--floors: %w", err)
		}
	}

	opts.f = season.SPFormula{
		BaseWin:    *baseWin,
		BaseLoss:   *baseLoss,
		Scale:      *scale,
		BotSeatSP:  *botSP,
		CapotBonus: *capot,
	}
	if opts.f.BotSeatSP < 0 {
		opts.f.BotSeatSP, _ = opts.ladder.Floor(season.TierGold)
	}
	if opts.f.BaseWin <= 0 || opts.f.BaseLoss <= 0 || opts.f.Scale <= 0 {
		return options{}, errors.New("--base-win, --base-loss and --scale must be positive")
	}

	if opts.sweep.WinRates, err = parseRates(*rates); err != nil {
		return options{}, fmt.Errorf("--sweep-rates: %w", err)
	}
	opts.sweep.Runs = *runs
	opts.sweep.Horizon = *horizon
	opts.sweep.AfterN = *afterN
	opts.sweep.FixedMargin = *margin
	opts.sweep.Seed = *seed
	opts.trace = *trace
	return opts, nil
}

func parseInts(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func parseRates(s string) ([]float64, error) {
	parts := strings.Split(s, ",")
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, err
		}
		if v <= 0 || v >= 1 {
			return nil, fmt.Errorf("win rate %v must be strictly between 0 and 1", v)
		}
		out = append(out, v)
	}
	return out, nil
}

func run(opts options, out io.Writer) error {
	var (
		matches []match.Match
		names   map[uint]string
	)
	if opts.dbURL != "" {
		db, err := gorm.Open(postgres.Open(opts.dbURL), &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		})
		if err != nil {
			return fmt.Errorf("connecting: %w", err)
		}
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			defer sqlDB.Close()
		}
		err = readOnly(db, func(tx *gorm.DB) error {
			var loadErr error
			matches, names, loadErr = loadWindow(tx, opts.from, opts.to)
			return loadErr
		})
		if err != nil {
			return err
		}
	}

	printConstants(out, opts)

	if opts.dbURL != "" {
		players, sum, err := replay(matches, names, opts.f, opts.ladder, opts.trace)
		if err != nil {
			return err
		}
		printReplay(out, opts, players, sum)
		printTrace(out, opts, sum.Trace)
		opts.sweep.Won, opts.sweep.Lost = templatePools(matches)
	}

	rows, err := sweep(opts.sweep, opts.f, opts.ladder)
	if err != nil {
		return err
	}
	printSweep(out, opts, rows)
	return nil
}

// readOnly runs fn inside a READ ONLY transaction, so Postgres itself refuses
// any write the replay might attempt, and rolls it back whatever fn returns.
func readOnly(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	tx := db.Begin(&sql.TxOptions{ReadOnly: true})
	if tx.Error != nil {
		return fmt.Errorf("opening a read-only transaction: %w", tx.Error)
	}
	defer tx.Rollback()
	return fn(tx)
}

func rankLabel(l season.Ladder, sp int) string {
	tier, div := l.Rank(sp)
	if div == 0 {
		return tier
	}
	return fmt.Sprintf("%s %d", tier, div)
}

func printConstants(out io.Writer, opts options) {
	f := opts.f
	fmt.Fprintln(out, "Constants")
	fmt.Fprintf(out, "  W (base win) %g   L (base loss) %g   S (scale) %g   Capot bonus %+d   bot seat %d SP\n",
		f.BaseWin, f.BaseLoss, f.Scale, f.CapotBonus, f.BotSeatSP)
	// The extremes at the maximum margin against a certain result (E = 0 or 1);
	// the best win includes the Capot bonus, which is added after scaling.
	worstLoss := -int(math.Round(f.BaseLoss * 1.5 * 2))
	bestWin := int(math.Round(f.BaseWin*1.5*2)) + f.CapotBonus
	fmt.Fprintf(out, "  abandon penalty %d   worst possible loss %d   best possible win %+d\n",
		f.AbandonPenalty(), worstLoss, bestWin)
	var parts []string
	for _, r := range opts.ladder {
		parts = append(parts, fmt.Sprintf("%s %d", r.Tier, r.Floor))
	}
	fmt.Fprintf(out, "  floors: %s\n\n", strings.Join(parts, " · "))
}

func pct(n, d int) string {
	if d == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(n)/float64(d))
}

func printReplay(out io.Writer, opts options, players []playerResult, sum replaySummary) {
	fmt.Fprintf(out, "Replay %s to %s (completed_at, id order; every player starts on 0 SP)\n",
		opts.from.Format(time.RFC3339), opts.to.Format(time.RFC3339))
	fmt.Fprintf(out, "  loaded %d, scored %d (bot-only %d, surrenders %d, instant wins %d, abandonments %d, with a Capot %d)\n",
		sum.Loaded, sum.Scored, sum.BotOnly, sum.Surrenders, sum.InstantWins, sum.Abandonments, sum.CapotMatches)
	for _, reason := range slices.Sorted(maps.Keys(sum.Skipped)) {
		fmt.Fprintf(out, "  skipped %d: %s\n", sum.Skipped[reason], reason)
	}
	fmt.Fprintln(out)

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "user\tname\tmatches\twin %\tbot-only\tbot win %\tfinal SP\trank\tSilver at\t")
	for _, p := range players {
		silverAt := "-"
		if p.SilverAt > 0 {
			silverAt = strconv.Itoa(p.SilverAt)
		}
		rank := p.Tier
		if p.Division > 0 {
			rank = fmt.Sprintf("%s %d", p.Tier, p.Division)
		}
		fmt.Fprintf(tw, "%d\t%s\t%d\t%s\t%d\t%s\t%d\t%s\t%s\t\n",
			p.UserID, p.Name, p.Matches, pct(p.Wins, p.Matches),
			p.BotMatches, pct(p.BotWins, p.BotMatches), p.FinalSP, rank, silverAt)
	}
	_ = tw.Flush()

	counts := map[string]int{}
	for _, p := range players {
		counts[p.Tier]++
	}
	var parts []string
	for _, tier := range season.SeasonTiers() {
		parts = append(parts, fmt.Sprintf("%s %d", tier, counts[tier]))
	}
	fmt.Fprintf(out, "\n  players per tier: %s\n\n", strings.Join(parts, " · "))
}

func printSweep(out io.Writer, opts options, rows []sweepRow) {
	cfg := opts.sweep
	source := fmt.Sprintf("margins sampled from the window's bot-only matches (%d won, %d lost)", len(cfg.Won), len(cfg.Lost))
	switch {
	case cfg.FixedMargin > 0:
		source = fmt.Sprintf("fixed margin %.2f", cfg.FixedMargin)
	case len(cfg.Won) == 0 || len(cfg.Lost) == 0:
		source = fmt.Sprintf("fixed margin %.2f (no window to sample)", defaultSweepMargin)
	}
	fmt.Fprintf(out, "Synthetic sweep: one human + bot vs two bots from 0 SP, %d runs x %d matches, %s\n",
		cfg.Runs, cfg.Horizon, source)

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintf(tw, "win rate\tsettle (analytic)\trank\tsettle (simulated)\trank\tafter %d: p10\tmedian\tp90\tmedian rank\tSilver at (median)\treached\twithin %d\t\n",
		cfg.AfterN, cfg.AfterN)
	for _, r := range rows {
		silver := "-"
		if r.SilverMedian > 0 {
			silver = strconv.Itoa(r.SilverMedian)
		}
		fmt.Fprintf(tw, "%.0f%%\t%.0f\t%s\t%.0f\t%s\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t\n",
			100*r.P,
			r.AnalyticSettle, rankLabel(opts.ladder, int(r.AnalyticSettle+0.5)),
			r.SimSettle, rankLabel(opts.ladder, int(r.SimSettle+0.5)),
			r.AfterNP10, r.AfterNMedian, r.AfterNP90, rankLabel(opts.ladder, r.AfterNMedian),
			silver, pct(int(r.SilverReached*1000+0.5), 1000), pct(int(r.SilverWithinN*1000+0.5), 1000))
	}
	_ = tw.Flush()
}

func printTrace(out io.Writer, opts options, rows []traceRow) {
	if opts.trace == 0 {
		return
	}
	fmt.Fprintf(out, "Trace for user %d (%d matches)\n", opts.trace, len(rows))
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "#\tmatch\tcompleted\ttable\tresult\tbefore\tchange\tafter\trank\t")
	for _, r := range rows {
		table := "mixed"
		if r.BotOnly {
			table = "bots"
		}
		result := "loss"
		switch {
		case r.Abandoned:
			result = "abandoned"
		case r.Won && r.Surrender:
			result = "win (surr.)"
		case r.Won:
			result = "win"
		case r.Surrender:
			result = "loss (surr.)"
		}
		fmt.Fprintf(tw, "%d\t%d\t%s\t%s\t%s\t%d\t%+d\t%d\t%s\t\n",
			r.N, r.MatchID, r.At.UTC().Format("2006-01-02 15:04"), table, result,
			r.Before, r.Change, r.After, rankLabel(opts.ladder, r.After))
	}
	_ = tw.Flush()
	fmt.Fprintln(out)
}
