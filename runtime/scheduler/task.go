package scheduler

import "time"

// TaskState is the lifecycle state for one schedulable task.
type TaskState string

const (
	TaskPending  TaskState = "pending"
	TaskRunning  TaskState = "running"
	TaskSuccess  TaskState = "success"
	TaskFailed   TaskState = "failed"
	TaskCanceled TaskState = "canceled"
)

// TaskType classifies task role in distributed execution.
type TaskType string

const (
	TaskTypeMap    TaskType = "map"
	TaskTypeReduce TaskType = "reduce"
	TaskTypeOther  TaskType = "other"
)

// Task is a minimal unit to schedule and retry.
type Task struct {
	ID        string
	JobID     string
	StageID   string
	Type      TaskType
	Payload   map[string]string
	State     TaskState
	WorkerID  string
	Attempt   int
	MaxRetry  int
	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (t *Task) canRetry() bool {
	return t.Attempt <= t.MaxRetry
}
