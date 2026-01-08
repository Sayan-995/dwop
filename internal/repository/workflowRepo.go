package repository

import (
	"encoding/json"
	"fmt"

	// "log"
	"strings"
	"time"

	u "github.com/Sayan-995/dwop/internal/utils"
	"github.com/google/uuid"
)

func InsertWorkflow(workflow u.Workflow, tasks []u.Task) error {
	var outboxEvents []u.OutboxEvent
	for _, task := range tasks {
		if task.PendingPreds != 0 {
			continue
		}
		fmt.Println(task.TaskId)
		event := u.OutboxEvent{
			EventID:         uuid.New(),
			WorkflowId:      workflow.WorkflowId,
			TaskID:          task.TaskId,
			Type:            u.OutboxTaskReady,
			CreatedAt:       time.Now(),
			PublishAttempts: 5,
		}
		fmt.Println(event.EventID)
		outboxEvents = append(outboxEvents, event)
	}

	if workflow.Status == "" {
		workflow.Status = u.RunRunning
	}

	args := map[string]any{
		"workflow":      workflow,
		"tasks":         tasks,
		"outbox_events": outboxEvents,
	}

	fmt.Printf("[InsertWorkflow] Creating workflow %s with status %s\n", workflow.WorkflowId, workflow.Status)
	data := DB.Rpc("create_workflow_with_tasks_and_outbox", "", args)
	if strings.Contains(data, `"code"`) {
		return fmt.Errorf("rpc create_workflow_with_tasks_and_outbox failed: %v", data)
	}

	_, _, err := DB.From("workflows").Update(map[string]string{
		"status": string(u.RunRunning),
	}, "", "").Eq("workflow_id", workflow.WorkflowId.String()).Execute()
	if err != nil {
		fmt.Printf("[InsertWorkflow] WARNING: Failed to set workflow status to RUNNING: %v\n", err)
	} else {
		fmt.Printf("[InsertWorkflow] Workflow %s status set to RUNNING\n", workflow.WorkflowId)
	}

	return nil
}

func GetWorkflowByID(id uuid.UUID) (*u.Workflow, error) {
	data, _, err := DB.From("workflows").Select("*", "", false).Eq("workflow_id", id.String()).Execute()
	if err != nil {
		return nil, err
	}
	var rows []u.Workflow
	if err = json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func CancelWorkflowById(id string) error {
	_, _, err := DB.From("workflows").Update(map[string]string{
		"status": string(u.RunCanceled),
	}, "", "").Eq("workflow_id", id).Execute()
	return err
}
