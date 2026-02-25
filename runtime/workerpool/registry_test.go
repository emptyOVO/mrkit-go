package workerpool

import (
	"testing"
	"time"
)

func TestRegistryLeaseAndEvict(t *testing.T) {
	r := New(10 * time.Millisecond)
	r.Register("w1", "127.0.0.1:9001")

	if ok := r.SetState("w1", WorkerBusy); !ok {
		t.Fatalf("set state failed")
	}
	if ok := r.Heartbeat("w1"); !ok {
		t.Fatalf("heartbeat failed")
	}

	if got := len(r.Snapshot()); got != 1 {
		t.Fatalf("expected 1 worker, got %d", got)
	}

	time.Sleep(15 * time.Millisecond)
	evicted := r.EvictExpired(time.Now())
	if len(evicted) != 1 {
		t.Fatalf("expected 1 evicted worker, got %d", len(evicted))
	}
	if got := len(r.Snapshot()); got != 0 {
		t.Fatalf("expected 0 worker after eviction, got %d", got)
	}
}
