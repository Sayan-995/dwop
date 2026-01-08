package repository

import (
	"encoding/json"

	"github.com/Sayan-995/dwop/internal/utils"
	"github.com/google/uuid"
)

// "sync"

func InsertTask(tasks any) error {
	_, _, err := DB.
		From("tasks").
		Insert(tasks, false, "", "", "").
		Execute()
	return err
}
func InsertTaskRun(taskRuns any) error {
	_, _, err := DB.
		From("task_runs").
		Insert(taskRuns, false, "", "", "").
		Execute()
	return err
}

func UpsertTaskRun(taskRuns any) (int64, error) {
	_, count, err := DB.
		From("task_runs").
		Insert(taskRuns, true, "", "minimal", "exact").
		Execute()
	return count, err
}
func UpdateTaskStatus(taskId string, status utils.TaskStatus) error {
	_, _, err := DB.From("tasks").
		Update(map[string]any{"status": status}, "minimal", "").
		Eq("task_id", taskId).
		Execute()
	return err
}

func GetTaskByID(id uuid.UUID) (*utils.Task, error) {
	data, _, err := DB.From("tasks").Select("*", "", false).Eq("task_id", id.String()).Execute()
	if err != nil {
		return nil, err
	}
	var rows []utils.Task
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}
