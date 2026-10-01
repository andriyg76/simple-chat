package chatdispatch

import (
	"context"
	"strings"
	"sync"
	"time"
)

type Handler func(context.Context, string)

type queuedMessage struct {
	id      string
	body    string
	handler Handler
}

type state struct {
	mu     sync.Mutex
	buffer []queuedMessage
	timer  *time.Timer
	busy   bool
	cancel context.CancelFunc
}

// Dispatcher serializes work per conversation and combines messages received
// during debounce or an active turn into the next newline-separated turn.
type Dispatcher struct {
	delay  time.Duration
	mu     sync.Mutex
	states map[string]*state
}

func New(delay time.Duration) *Dispatcher {
	return &Dispatcher{delay: delay, states: make(map[string]*state)}
}

func (d *Dispatcher) Submit(key, id, body string, handler Handler) string {
	st := d.state(key)
	st.mu.Lock()
	st.buffer = append(st.buffer, queuedMessage{id: id, body: body, handler: handler})
	batchID := st.buffer[0].id
	if st.busy {
		st.mu.Unlock()
		return batchID
	}
	if st.timer != nil {
		st.timer.Stop()
	}
	st.timer = time.AfterFunc(d.delay, func() { d.flush(key, st) })
	st.mu.Unlock()
	return batchID
}

func (d *Dispatcher) Cancel(key string) bool {
	st := d.state(key)
	st.mu.Lock()
	if st.timer != nil {
		st.timer.Stop()
		st.timer = nil
	}
	st.buffer = nil
	cancel, busy := st.cancel, st.busy
	st.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return busy
}

func (d *Dispatcher) state(key string) *state {
	d.mu.Lock()
	defer d.mu.Unlock()
	if st := d.states[key]; st != nil {
		return st
	}
	st := &state{}
	d.states[key] = st
	return st
}

func (d *Dispatcher) flush(key string, st *state) {
	st.mu.Lock()
	if st.busy || len(st.buffer) == 0 {
		st.mu.Unlock()
		return
	}
	batch := st.buffer
	st.buffer = nil
	st.busy = true
	st.timer = nil
	ctx, cancel := context.WithCancel(context.Background())
	st.cancel = cancel
	st.mu.Unlock()

	bodies := make([]string, len(batch))
	for i := range batch {
		bodies[i] = batch[i].body
	}
	batch[0].handler(ctx, strings.Join(bodies, "\n"))
	cancel()

	st.mu.Lock()
	st.busy = false
	st.cancel = nil
	if len(st.buffer) > 0 {
		st.timer = time.AfterFunc(d.delay, func() { d.flush(key, st) })
	}
	st.mu.Unlock()
}
