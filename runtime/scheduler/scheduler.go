package scheduler

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrNoTask means no pending task is ready for assignment.
	ErrNoTask = errors.New("no pending task")
	// ErrTaskNotFound means task ID is not known by scheduler.
	ErrTaskNotFound = errors.New("task not found")
	// ErrInvalidState means state transition is not allowed.
	ErrInvalidState = errors.New("invalid task state transition")
)

// Scheduler manages task queue and retry lifecycle.
type Scheduler struct {
	mu         sync.Mutex
	retry      RetryPolicy
	taskByID   map[string]*Task
	pendingIDs []string
}

func New(retry RetryPolicy) *Scheduler {
	return &Scheduler{
		retry:    retry,
		taskByID: make(map[string]*Task),
	}
}

func (s *Scheduler) Submit(task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		return errors.New("task id is required")
	}
	if _, ok := s.taskByID[task.ID]; ok {
		return errors.New("task id already exists")
	}
	now := time.Now()
	task.State = TaskPending
	task.Attempt = 0
	task.WorkerID = ""
	task.LastError = ""
	task.CreatedAt = now
	task.UpdatedAt = now
	cp := task
	s.taskByID[task.ID] = &cp
	s.pendingIDs = append(s.pendingIDs, task.ID)
	return nil
}

func (s *Scheduler) Assign(workerID string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pendingIDs) == 0 {
		return nil, ErrNoTask
	}
	id := s.pendingIDs[0]
	s.pendingIDs = s.pendingIDs[1:]
	t := s.taskByID[id]
	t.State = TaskRunning
	t.WorkerID = workerID
	t.Attempt++
	t.UpdatedAt = time.Now()
	cp := *t
	return &cp, nil
}

func (s *Scheduler) Complete(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.taskByID[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if t.State != TaskRunning {
		return ErrInvalidState
	}
	t.State = TaskSuccess
	t.UpdatedAt = time.Now()
	return nil
}

func (s *Scheduler) Fail(taskID, errMsg string) error {
	s.mu.Lock()
	t, ok := s.taskByID[taskID]
	if !ok {
		s.mu.Unlock()
		return ErrTaskNotFound
	}
	if t.State != TaskRunning {
		s.mu.Unlock()
		return ErrInvalidState
	}
	t.LastError = errMsg
	t.WorkerID = ""
	t.UpdatedAt = time.Now()
	if !t.canRetry() {
		t.State = TaskFailed
		s.mu.Unlock()
		return nil
	}
	t.State = TaskPending
	delay := s.retry.nextDelay(t.Attempt)
	taskIDCopy := taskID
	s.mu.Unlock()

	// Re-enqueue asynchronously after backoff.
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
		s.mu.Lock()
		defer s.mu.Unlock()
		task, ok := s.taskByID[taskIDCopy]
		if !ok {
			return
		}
		// Skip if state changed by external cancellation/update.
		if task.State != TaskPending {
			return
		}
		s.pendingIDs = append(s.pendingIDs, taskIDCopy)
	}()
	return nil
}

func (s *Scheduler) Snapshot() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Task, 0, len(s.taskByID))
	for _, t := range s.taskByID {
		out = append(out, *t)
	}
	return out
}
