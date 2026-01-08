package rabitmq

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sayan-995/dwop/internal/repository"
	u "github.com/Sayan-995/dwop/internal/utils"
)

func SendTaskEvents(id int, outboxEvents string, ch chan u.OutboxEvent) {
	defer close(ch)
	var events []u.OutboxEvent
	if err := json.Unmarshal([]byte(outboxEvents), &events); err != nil {
		fmt.Printf("[SendTaskEvents] ERROR unmarshaling outbox events: %v\n", err)
		return
	}
	fmt.Printf("[SendTaskEvents] Claimer %d: Processing %d outbox events\n", id, len(events))
	for _, event := range events {
		res, _ := json.Marshal(event)
		fmt.Printf("[SendTaskEvents] Publishing event %s for task %s\n", event.EventID, event.TaskID)
		wf, err := repository.GetWorkflowByID(event.WorkflowId)
		if err != nil {
			fmt.Printf("[SendTaskEvents] ERROR getting workflow %s: %v\n", event.WorkflowId, err)
			msg := err.Error()
			event.LastPublishError = &msg
			ch <- event
			continue
		}
		if wf == nil {
			fmt.Printf("[SendTaskEvents] Workflow %s not found\n", event.WorkflowId)
			errMsg := "workflow not found"
			event.LastPublishError = &errMsg
			ch <- event
			continue
		}
		if wf.Status != u.RunRunning {
			fmt.Printf("[SendTaskEvents] Workflow %s not running (status: %s), skipping event %s\n", event.WorkflowId, wf.Status, event.EventID)
			t := time.Now()
			event.PublishedAt = &t
			event.LastPublishError = nil
			ch <- event
			continue
		}
		err = PublishTask(res)
		if err != nil {
			fmt.Printf("[SendTaskEvents] ERROR publishing event %s: %v\n", event.EventID, err)
			msg := err.Error()
			event.LastPublishError = &msg
		} else {
			fmt.Printf("[SendTaskEvents] Event %s published successfully\n", event.EventID)
			t := time.Now()
			event.PublishedAt = &t
			event.LastPublishError = nil
		}
		ch <- event
	}
}
