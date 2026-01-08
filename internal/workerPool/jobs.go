package workerpool

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Sayan-995/dwop/internal/executor"
	rabitmq "github.com/Sayan-995/dwop/internal/rabitMQ"
	rmq "github.com/Sayan-995/dwop/internal/rabitMQ"
	repo "github.com/Sayan-995/dwop/internal/repository"
	"github.com/Sayan-995/dwop/internal/service"
	"github.com/Sayan-995/dwop/internal/utils"
	"github.com/google/uuid"
)

func OutboxClaimJob(id int) {
	data, err := repo.ClaimOutboxEvents(id)
	if err != nil {
		fmt.Printf("[OutboxClaimJob] ERROR claiming events: %v\n", err)
		return
	}
	if data == "[]" {
		fmt.Printf("[OutboxClaimJob %d] No events to claim\n", id)
		return
	}
	fmt.Printf("[OutboxClaimJob %d] Claimed %d events, sending to RMQ\n", id, len(data))
	errCh := make(chan utils.OutboxEvent, 200)
	rmq.SendTaskEvents(id, data, errCh)
	for event := range errCh {
		if event.LastPublishError != nil {
			fmt.Printf("[OutboxClaimJob] Publish failed for event %s: %v\n", event.EventID, *event.LastPublishError)
			if event.PublishAttempts == 0 {
				fmt.Printf("[OutboxClaimJob] Max attempts reached for event %s, canceling workflow %s\n", event.EventID, event.WorkflowId)
				_ = service.CancelWorkflow(event.WorkflowId.String())
			} else {
				fmt.Printf("[OutboxClaimJob] Retrying event %s (attempts left: %d)\n", event.EventID, event.PublishAttempts-1)
				event.EventID = uuid.New()
				event.ClaimedAt = nil
				event.ClaimedBy = nil
				event.PublishAttempts--
				event.Type = utils.OutboxTaskRetryReady
				addErr := repo.AddOutboxEvent(event)
				if addErr != nil {
					fmt.Printf("[OutboxClaimJob] ERROR re-adding event for retry: %v\n", addErr)
				}
			}
		} else {
			fmt.Printf("[OutboxClaimJob] Event %s published successfully, marking in DB\n", event.EventID)
			t := time.Now()
			event.PublishedAt = &t
			updateErr := repo.UpdateOutboxEvent(event)
			if updateErr != nil {
				fmt.Printf("[OutboxClaimJob] ERROR updating published event %s: %v\n", event.EventID, updateErr)
			}
		}
	}
}

func ConsumeRabitMQJob(id int) {
	ch, err := rabitmq.RabbitMQClient.Conn.Channel()
	if err != nil {
		log.Fatalf("error getting the consumer channel: %v", err)
	}
	fmt.Printf("%dth consumer started\n", id)
	err = ch.Qos(1, 0, false)

	if err != nil {
		log.Fatalf("error while setting consumer's qos: %v", err)
	}
	msg, err := ch.Consume(
		rabitmq.QueueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)

	for d := range msg {
		var event utils.OutboxEvent
		err := json.Unmarshal(d.Body, &event)
		if err != nil {
			fmt.Println(err)
			_ = d.Reject(true)
			continue
		}
		taskInstance := utils.TaskRun{
			RunId:      uuid.New(),
			TaskId:     event.TaskID,
			WorkflowId: event.WorkflowId,
			LastError:  nil,
			CreatedAt:  time.Now(),
		}
		cnt, err := repo.UpsertTaskRun(taskInstance)
		if err != nil {
			fmt.Println(err)
			_ = d.Reject(true)
		} else if cnt == 0 {
			_ = d.Reject(false)
		} else {
			workflow, err := repo.GetWorkflowByID(event.WorkflowId)
			if err != nil {
				fmt.Printf("[ConsumeJob] Error getting workflow %s: %v\n", event.WorkflowId, err)
				_ = d.Reject(true)
				continue
			}
			if workflow == nil {
				fmt.Printf("[ConsumeJob] Workflow %s not found\n", event.WorkflowId)
				_ = d.Reject(true)
				continue
			}
			if workflow.Status != utils.RunRunning {
				fmt.Printf("[ConsumeJob] Workflow %s status is %s, skipping\n", event.WorkflowId, workflow.Status)
				_ = d.Ack(false)
				continue
			}
			task, err := repo.GetTaskByID(event.TaskID)
			if err != nil {
				fmt.Printf("[ConsumeJob] Error getting task %s: %v\n", event.TaskID, err)
				_ = d.Reject(true)
				continue
			}
			if task == nil {
				fmt.Printf("[ConsumeJob] Task %s not found\n", event.TaskID)
				_ = d.Reject(true)
				continue
			}
			if utils.Conf == nil || utils.Conf.K8s == nil {
				fmt.Printf("[ConsumeJob] ERROR: Kubernetes client not initialized\n")
				_ = d.Reject(true)
				continue
			}
			fmt.Printf("[ConsumeJob] Creating job for task %s (run %s) in workflow %s\n", task.TaskId, taskInstance.RunId, event.WorkflowId)
			_, err = executor.CreateJob(utils.Conf.K8s, utils.Conf.Namespace, utils.Conf.ImageName, *workflow, *task, taskInstance.RunId)
			if err != nil {
				fmt.Printf("[ConsumeJob] ERROR creating job: %v\n", err)
				_ = d.Reject(true)
				continue
			}
			fmt.Printf("[ConsumeJob] Job created successfully\n")
			_ = d.Ack(false)
		}

	}
	fmt.Printf("%dth consumer exitted\n", id)
}
