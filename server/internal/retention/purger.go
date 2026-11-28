package retention

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Task deletes one kind of stale row and reports how many rows went.
type Task struct {
	Name  string
	Purge func(now time.Time) (int64, error)
}

// Purger is the daily job that deletes rows past their retention period.
// A failed task is logged and never stops the others or the loop.
type Purger struct {
	tasks    []Task
	interval time.Duration
	now      func() time.Time

	done     chan struct{}
	started  atomic.Bool
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewPurger builds the job; pass 0 and nil for the production defaults of 24h and time.Now.
func NewPurger(tasks []Task, interval time.Duration, now func() time.Time) *Purger {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if now == nil {
		now = time.Now
	}
	return &Purger{tasks: tasks, interval: interval, now: now, done: make(chan struct{})}
}

// Start runs one pass immediately, then one per interval until Shutdown. A second Start is a no-op.
func (p *Purger) Start() {
	if p.started.Swap(true) {
		return
	}
	select {
	case <-p.done:
		return
	default:
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.RunOnce()
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		for {
			select {
			case <-p.done:
				return
			case <-ticker.C:
				p.RunOnce()
			}
		}
	}()
}

// Shutdown stops the loop and waits for an in-flight pass. It is idempotent and safe without Start.
func (p *Purger) Shutdown() {
	p.stopOnce.Do(func() { close(p.done) })
	p.wg.Wait()
}

// RunOnce runs every task once and returns the rows each deleted, keyed by task name.
func (p *Purger) RunOnce() map[string]int64 {
	now := p.now()
	deleted := make(map[string]int64, len(p.tasks))
	for _, task := range p.tasks {
		n, err := task.Purge(now)
		if err != nil {
			slog.Error("retention: purge failed", "task", task.Name, "error", err)
			continue
		}
		deleted[task.Name] = n
		slog.Info("retention: purge done", "task", task.Name, "deleted", n)
	}
	return deleted
}
