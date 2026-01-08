package workerpool

import "github.com/Sayan-995/dwop/internal/utils"

type Job struct {
	Execute func(int)
}

var (
	JobChan chan Job
)

func worker(id int, jobs chan Job) {
	for job := range jobs {
		job.Execute(id)
	}
}

func init() {
	JobChan = make(chan Job, utils.WorkerCount)
	for i := 0; i < utils.WorkerCount; i++ {
		go worker(i, JobChan)
	}
}

func GetJob(fun func(int)) Job {
	return Job{
		Execute: fun,
	}
}
