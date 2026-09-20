package runtime

import (
	"fmt"
	"slices"
	"sync"
)

type TaskAction int

type TaskStatus int

type TaskPriority int

type TaskFunc func(t *Task) (TaskAction, error)

type Task struct {
	Func     TaskFunc
	Status   TaskStatus
	Priority TaskPriority
}

var (
	taskMu sync.Mutex
	tasks  = []*Task{}
)

const (
	TaskActionYield TaskAction = iota
	TaskActionFinish
)

const (
	TaskStatusReady TaskStatus = iota
	TaskStatusIgnored
)

const (
	TaskPriorityLow TaskPriority = iota
	TaskPriorityMedium
	TaskPriorityHigh
)

func (tp TaskPriority) String() string {
	switch tp {
	case TaskPriorityLow:
		return "Low"
	case TaskPriorityMedium:
		return "Medium"
	case TaskPriorityHigh:
		return "High"
	}
	return ""
}

// SpawnTask pushes the provided pointer to [Task] onto the internal tasks slice.
func SpawnTask(t *Task) {
	taskMu.Lock()
	defer taskMu.Unlock()
	tasks = append(tasks, t)
}

func TaskExecutor() error {
	if len(tasks) == 0 {
		return nil
	}

	slices.SortFunc(tasks, func(a *Task, b *Task) int {
		if a.Priority < b.Priority {
			return -1
		}
		if a.Priority > b.Priority {
			return 1
		}
		return 0
	})

	for i, task := range tasks {
		if task.Status == TaskStatusIgnored {
			task.Status = TaskStatusReady
			continue
		}
		action, err := task.Func(task)
		if err != nil {
			return fmt.Errorf("task %d with priority %q returned an error: %v", i, task.Priority, err)
		}
		switch action {
		case TaskActionYield:
			task.Status = TaskStatusIgnored
			continue
		case TaskActionFinish:
			tasks = slices.Delete(tasks, i, i+1)
		}
	}
	return nil
}
