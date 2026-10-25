package match

import (
	"log/slog"
	"runtime"
	"time"
)

const (
	defaultBotSearchDeadline  = 500 * time.Millisecond
	defaultBotSearchFromTrick = 3
	maxDefaultSearchWorkers   = 4
	searchQueuePerWorker      = 4
)

// BotSearchSettings tunes the bots' card search: how many searches run at once, how long one may take and the first
// trick it plays from. A FromTrick of zero turns the search off.
type BotSearchSettings struct {
	Workers   int
	Deadline  time.Duration
	FromTrick int
}

// DefaultBotSearchSettings sizes the pool to half the CPUs, between one and four workers.
func DefaultBotSearchSettings() BotSearchSettings {
	return BotSearchSettings{
		Workers:   max(1, min(maxDefaultSearchWorkers, runtime.NumCPU()/2)),
		Deadline:  defaultBotSearchDeadline,
		FromTrick: defaultBotSearchFromTrick,
	}
}

// searchPool runs search jobs on a fixed set of workers behind a bounded queue, so a burst of bot turns never grows
// memory or blocks a timer goroutine.
type searchPool struct {
	jobs chan func()
}

func newSearchPool(workers int) *searchPool {
	workers = max(workers, 1)
	p := &searchPool{jobs: make(chan func(), searchQueuePerWorker*workers)}
	for range workers {
		go p.work()
	}
	return p
}

func (p *searchPool) work() {
	for job := range p.jobs {
		runSearchJob(job)
	}
}

// runSearchJob keeps a panicking search from taking the server down: the bot then plays by the rules.
func runSearchJob(job func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("bot: search job panicked; rules played", "panic", r)
		}
	}()
	job()
}

// submit queues the job and reports false, dropping it, when the queue is full.
func (p *searchPool) submit(job func()) bool {
	select {
	case p.jobs <- job:
		return true
	default:
		return false
	}
}
