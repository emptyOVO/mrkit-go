package workerpool

import (
	"sync"
	"time"
)

// WorkerState captures current scheduler view for one worker.
type WorkerState string

const (
	WorkerIdle    WorkerState = "idle"
	WorkerBusy    WorkerState = "busy"
	WorkerUnknown WorkerState = "unknown"
)

// Worker is an in-memory lease entry for one runtime worker.
type Worker struct {
	ID          string
	Addr        string
	State       WorkerState
	LastSeen    time.Time
	LeaseExpire time.Time
}

// Registry tracks workers by heartbeat lease.
type Registry struct {
	mu    sync.Mutex
	ttl   time.Duration
	items map[string]*Worker
}

func New(ttl time.Duration) *Registry {
	if ttl <= 0 {
		ttl = 5 * time.Second
	}
	return &Registry{
		ttl:   ttl,
		items: make(map[string]*Worker),
	}
}

func (r *Registry) Register(id, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.items[id] = &Worker{
		ID:          id,
		Addr:        addr,
		State:       WorkerIdle,
		LastSeen:    now,
		LeaseExpire: now.Add(r.ttl),
	}
}

func (r *Registry) Heartbeat(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.items[id]
	if !ok {
		return false
	}
	now := time.Now()
	w.LastSeen = now
	w.LeaseExpire = now.Add(r.ttl)
	if w.State == WorkerUnknown {
		w.State = WorkerIdle
	}
	return true
}

func (r *Registry) SetState(id string, state WorkerState) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.items[id]
	if !ok {
		return false
	}
	w.State = state
	return true
}

func (r *Registry) EvictExpired(now time.Time) []Worker {
	r.mu.Lock()
	defer r.mu.Unlock()
	var evicted []Worker
	for id, w := range r.items {
		if now.After(w.LeaseExpire) {
			evicted = append(evicted, *w)
			delete(r.items, id)
		}
	}
	return evicted
}

func (r *Registry) Snapshot() []Worker {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Worker, 0, len(r.items))
	for _, w := range r.items {
		out = append(out, *w)
	}
	return out
}
