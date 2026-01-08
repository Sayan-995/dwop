package repository

import (
	"fmt"
	"strings"

	"github.com/Sayan-995/dwop/internal/utils"
)

var (
	TableName = "outbox_events"
)

func ClaimOutboxEvents(id int) (string, error) {
	args := map[string]any{
		"batch_size": 200,
		"claimer_id": id,
	}
	data := DB.Rpc("claim_outbox_events", "", args)
	// Only treat as error if response starts with error structure
	if strings.HasPrefix(strings.TrimSpace(data), `{"code"`) {
		return "", fmt.Errorf("[ClaimOutboxEvents] RPC error: %s", data)
	}
	return data, nil
}

func AddOutboxEvent(event utils.OutboxEvent) error {
	_, _, err := DB.From(TableName).Insert(event, false, "", "minimal", "").Execute()
	if err != nil {
		return fmt.Errorf("[AddOutboxEvent] failed to insert event %s: %v", event.EventID, err)
	}
	return nil
}

func UpdateOutboxEvent(event utils.OutboxEvent) error {
	_, _, err := DB.From(TableName).
		Update(event, "minimal", "").
		Eq("event_id", event.EventID.String()).
		Execute()
	if err != nil {
		return fmt.Errorf("[UpdateOutboxEvent] failed to update event %s: %v", event.EventID, err)
	}
	return nil
}

func CompleteRunAndEnqueueSuccessors(runId string) error {
	data := DB.Rpc("complete_run_and_enqueue_successors", "", map[string]string{
		"p_run_id": runId,
	})
	if strings.HasPrefix(strings.TrimSpace(data), `{"code"`) {
		return fmt.Errorf("rpc complete_run_and_enqueue_successors failed: %v", data)
	}
	return nil
}

func IncreaseAttempt(runID string, errmsg string) error {
	data := DB.Rpc("increase_attempt", "", map[string]string{
		"p_run_id": runID,
		"p_error":  errmsg,
	})
	if strings.HasPrefix(strings.TrimSpace(data), `{"code"`) {
		return fmt.Errorf("rpc increase_attempt failed: %v", data)
	}
	return nil
}

// func InsertTask(tasks any)error{
// 	_,_,err:=	 DB.
// 		From("Tasks").
// 		Insert(tasks,false,"","","").
// 		Execute()
// 	return err
// }
