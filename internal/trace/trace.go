// Package trace records optional, invocation-local sync timings.
package trace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
)

const (
	Sync         = "sync"
	ModelOpen    = "model_open"
	DeviceAttach = "device_attach"
)

type scopeKey struct{}

type scope struct {
	run     *Run
	section string
	window  int
}

// Run owns one append-only capture. Its spans can end on worker goroutines.
type Run struct {
	file    *os.File
	started time.Time
	mu      sync.Mutex
	totals  map[string]total
	err     error
	closed  bool
}

type total struct {
	Calls int     `json:"calls"`
	MS    float64 `json:"ms"`
	Share float64 `json:"share"`
}

// Open appends to path. The caller closes the run after all spans end.
func Open(path string) (*Run, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &Run{file: f, started: time.Now(), totals: make(map[string]total)}, nil
}

// Context enables tracing for work done with ctx.
func (r *Run) Context(ctx context.Context) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope{run: r})
}

func current(ctx context.Context) scope {
	s, _ := ctx.Value(scopeKey{}).(scope)
	return s
}

func WithSection(ctx context.Context, section string) context.Context {
	s := current(ctx)
	if s.run == nil {
		return ctx
	}
	s.section = section
	return context.WithValue(ctx, scopeKey{}, s)
}

func WithWindow(ctx context.Context, window int) context.Context {
	s := current(ctx)
	if s.run == nil {
		return ctx
	}
	s.window = window
	return context.WithValue(ctx, scopeKey{}, s)
}

// Active reports whether expensive trace-only fields should be computed.
func Active(ctx context.Context) bool { return current(ctx).run != nil }

type Fields struct {
	Device         string `json:"device,omitempty"`
	Inputs         int    `json:"inputs,omitempty"`
	TotalPositions int    `json:"total_positions,omitempty"`
	MaxPositions   int    `json:"max_positions,omitempty"`
	Failed         bool   `json:"failed,omitempty"`
}

type Span struct {
	scope
	name    string
	started time.Time
}

// Start avoids taking a timestamp or allocating with tracing disabled.
func Start(ctx context.Context, name string) Span {
	s := current(ctx)
	if s.run == nil {
		return Span{}
	}
	return Span{scope: s, name: name, started: time.Now()}
}

func (s *Span) End() { s.EndWith(Fields{}) }

// EndWith emits one event and adds it to the run totals. A span ends once.
func (s *Span) EndWith(fields Fields) {
	r := s.run
	if r == nil {
		return
	}
	s.run = nil
	ms := milliseconds(time.Since(s.started))
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	t := r.totals[s.name]
	t.Calls++
	t.MS += ms
	r.totals[s.name] = t
	r.write(struct {
		Span    string  `json:"span"`
		Run     int64   `json:"run"`
		StartMS float64 `json:"start_ms"`
		MS      float64 `json:"ms"`
		Section string  `json:"section,omitempty"`
		Window  int     `json:"window,omitempty"`
		Fields
	}{s.name, r.started.UnixNano(), milliseconds(s.started.Sub(r.started)), ms, s.section, s.window, fields})
}

func milliseconds(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// Close writes one summary. Shares are inclusive and use sync wall time.
func (r *Run) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return r.err
	}
	r.closed = true
	wall := r.totals[Sync].MS
	if wall == 0 {
		wall = milliseconds(time.Since(r.started))
	}
	for name, t := range r.totals {
		if wall > 0 {
			t.Share = t.MS / wall
		}
		r.totals[name] = t
	}
	r.write(struct {
		Span                string           `json:"span"`
		Run                 int64            `json:"run"`
		MS                  float64          `json:"ms"`
		ModelOpens          int              `json:"model_opens"`
		AcceleratorAttaches int              `json:"accelerator_attaches"`
		Spans               map[string]total `json:"spans"`
	}{"summary", r.started.UnixNano(), wall, r.totals[ModelOpen].Calls, r.totals[DeviceAttach].Calls, r.totals})
	r.err = errors.Join(r.err, r.file.Close())
	return r.err
}

// write holds mu and makes one file write per JSON line.
func (r *Run) write(value any) {
	if r.err != nil {
		return
	}
	line, err := json.Marshal(value)
	if err == nil {
		_, err = r.file.Write(append(line, '\n'))
	}
	r.err = err
}
