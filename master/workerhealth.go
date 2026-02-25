package master

import (
	"time"

	"github.com/emptyOVO/mrkit-go/runtime/workerpool"
)

// If there are 3 continuous unknow, we thought that that worker is dead.
// stop that work and make other deal with that.

func (ms *Master) PeriodicHealthCheck() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		ms.checkWorkersHealth()
	}

}

func (ms *Master) checkWorkersHealth() {
	ms.mux.Lock()
	workers := make([]WorkerInfo, len(ms.Workers))
	copy(workers, ms.Workers)
	ms.mux.Unlock()

	now := time.Now()
	for _, w := range workers {
		state := ms.client.Health(w.IP)
		ms.setWorkerState(w.UUID, state)
		if ms.registry == nil {
			continue
		}
		switch state {
		case WORKER_IDLE:
			ms.registry.Heartbeat(w.UUID)
			ms.registry.SetState(w.UUID, workerpool.WorkerIdle)
		case WORKER_BUSY:
			ms.registry.Heartbeat(w.UUID)
			ms.registry.SetState(w.UUID, workerpool.WorkerBusy)
		default:
			ms.registry.SetState(w.UUID, workerpool.WorkerUnknown)
		}
	}

	if ms.registry == nil {
		return
	}
	evicted := ms.registry.EvictExpired(now)
	for _, dead := range evicted {
		ms.setWorkerState(dead.ID, WORKER_UNKNOWN)
	}
}

// func checkHealth(worker WorkerInfo) {
// worker.WorkerState = Health(worker.IP)
// state := ""
// switch worker.WorkerState {
// case WORKER_IDLE:
// 	state = "Worker IDLE"
// case WORKER_BUSY:
// 	state = "Worker Busy"
// case WORKER_UNKNOWN:
// 	state = "Worker Dead"
// }

// log.Info("[Master] Worker state is :", state)
// }
