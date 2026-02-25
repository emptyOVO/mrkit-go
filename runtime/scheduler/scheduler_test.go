package scheduler

import (
	"testing"
	"time"
)

func TestSchedulerRetryAndComplete(t *testing.T) {
	s := New(RetryPolicy{
		BaseDelay: 5 * time.Millisecond,
		MaxDelay:  10 * time.Millisecond,
	})
	err := s.Submit(Task{
		ID:       "t1",
		JobID:    "j1",
		StageID:  "s1",
		Type:     TaskTypeMap,
		MaxRetry: 1,
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	task, err := s.Assign("w1")
	if err != nil {
		t.Fatalf("assign#1: %v", err)
	}
	if task.State != TaskRunning {
		t.Fatalf("expected running, got %s", task.State)
	}

	if err := s.Fail(task.ID, "boom"); err != nil {
		t.Fatalf("fail#1: %v", err)
	}

	time.Sleep(15 * time.Millisecond)
	task2, err := s.Assign("w2")
	if err != nil {
		t.Fatalf("assign#2: %v", err)
	}
	if task2.Attempt != 2 {
		t.Fatalf("expected attempt=2, got %d", task2.Attempt)
	}
	if err := s.Complete(task2.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}
}

func TestSchedulerExhaustedRetry(t *testing.T) {
	s := New(RetryPolicy{
		BaseDelay: 1 * time.Millisecond,
		MaxDelay:  2 * time.Millisecond,
	})
	if err := s.Submit(Task{
		ID:       "t2",
		JobID:    "j1",
		StageID:  "s1",
		Type:     TaskTypeReduce,
		MaxRetry: 0,
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	task, err := s.Assign("w1")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := s.Fail(task.ID, "fatal"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	time.Sleep(5 * time.Millisecond)
	_, err = s.Assign("w2")
	if err != ErrNoTask {
		t.Fatalf("expected ErrNoTask, got %v", err)
	}
}
