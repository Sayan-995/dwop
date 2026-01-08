package inboxpublisher

import (
	"context"

	rabitmq "github.com/Sayan-995/dwop/internal/rabitMQ"
	workerpool "github.com/Sayan-995/dwop/internal/workerPool"
)

func Run(ctx context.Context) {
	for i := 0; i < rabitmq.ConsumerChannelCount; i++ {
		workerpool.JobChan <- workerpool.GetJob(workerpool.ConsumeRabitMQJob)
	}
	<-ctx.Done()
}
