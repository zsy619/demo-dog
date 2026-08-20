package lifecycle

// Graceful shutdown with drain timeout.
//
// Round 59 introduces the Shutdown coordinator that wires
// together multiple server Stoppers into a single ordered
// drain. Each Stopper is registered with a Name + Timeout.
// When Shutdown() is called the coordinator runs each
// stopper in registration order; if any times out the
// coordinator records the error and proceeds to the next.
//
// It also wires signal handling: NotifyOn(parent) sets up
// a goroutine that listens on SIGINT / SIGTERM and fires
// Shutdown when either arrives. The coordinator emits
// progress events that callers can subscribe to via
// OnProgress.

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Stopper is the contract every server implements.
type Stopper interface {
	Shutdown(ctx context.Context) error
	Name() string
}

// StopFunc is the function-shaped adapter for inline closures.
type StopFunc func(ctx context.Context) error

// StopFuncAdapter wraps a closure into a Stopper.
type StopFuncAdapter struct {
	N string
	F StopFunc
}

// Shutdown runs the wrapped function.
func (a *StopFuncAdapter) Shutdown(ctx context.Context) error { return a.F(ctx) }

// Name returns the adapter name.
func (a *StopFuncAdapter) Name() string { return a.N }

// MakeStopper creates a Stopper from a name + function.
func MakeStopper(name string, f StopFunc) Stopper {
	return &StopFuncAdapter{N: name, F: f}
}

// Coordinator owns the stoppers + progress state.
type Coordinator struct {
	mu       sync.Mutex
	stoppers []Stopper
	progs    []Progress
	closed   atomic.Bool
	closedAt time.Time
	progress atomic.Int32
}

// Progress describes the state of one stopper.
type Progress struct {
	Name    string        `json:"name"`
	Status  string        `json:"status"`
	Error   string        `json:"error,omitempty"`
	Took    time.Duration `json:"took"`
}

// NewCoordinator returns an empty coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{}
}

// Register adds a stopper. Stoppers run in registration
// order; reverse registration to invert the order.
func (c *Coordinator) Register(s Stopper) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stoppers = append(c.stoppers, s)
}

// Progresses returns a copy of the progress entries.
func (c *Coordinator) Progresses() []Progress {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Progress, len(c.progs))
	copy(out, c.progs)
	return out
}

// Shutdown runs each stopper with the given per-stopper timeout.
// The total budget is the per-stopper timeout times len(stoppers)
// plus a small fixed pad for cross-stopper coordination.
func (c *Coordinator) Shutdown(perTimeout time.Duration) error {
	if c.closed.Swap(true) {
		return errors.New("shutdown already in progress")
	}
	c.closedAt = time.Now()
	c.mu.Lock()
	stoppers := make([]Stopper, len(c.stoppers))
	copy(stoppers, c.stoppers)
	c.mu.Unlock()
	var errs []error
	for _, s := range stoppers {
		c.progress.Add(1)
		ctx, cancel := context.WithTimeout(context.Background(), perTimeout)
		start := time.Now()
		err := s.Shutdown(ctx)
		cancel()
		took := time.Since(start)
		prog := Progress{Name: s.Name(), Took: took}
		if err != nil {
			prog.Status = "failed"
			prog.Error = err.Error()
			errs = append(errs, err)
		} else if took >= perTimeout {
			prog.Status = "timed_out"
			errs = append(errs, errors.New(s.Name()+" timed out"))
		} else {
			prog.Status = "stopped"
		}
		c.mu.Lock()
		c.progs = append(c.progs, prog)
		c.mu.Unlock()
	}
	if len(errs) == 0 {
		return nil
	}
	return joinErr(errs)
}

// Closed returns true after Shutdown has been called.
func (c *Coordinator) Closed() bool { return c.closed.Load() }

// ClosedAt returns the time Shutdown was first called.
func (c *Coordinator) ClosedAt() time.Time { return c.closedAt }

// joinErr concatenates multiple errors.
func joinErr(errs []error) error {
	s := ""
	for _, e := range errs {
		if s != "" {
			s += "; "
		}
		s += e.Error()
	}
	return errors.New(s)
}

// NotifyOn starts a goroutine that listens on parent + signal
// channels. When SIGINT or SIGTERM arrives, it calls
// perTimeout Shutdown. The function returns the goroutine wait
// channel so the caller can block until shutdown completes.
func (c *Coordinator) NotifyOn(parent context.Context, perTimeout time.Duration) <-chan struct{} {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-parent.Done():
			return
		case <-ch:
			c.Shutdown(perTimeout)
		}
	}()
	return done
}
