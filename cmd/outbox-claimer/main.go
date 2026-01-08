package outboxclaimer

import (
	"context"
	"time"

	workerpool "github.com/Sayan-995/dwop/internal/workerPool"
)

func Run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			workerpool.JobChan <- workerpool.GetJob(workerpool.OutboxClaimJob)
		}
	}
}
