package utils

import (
	// "runtime"
	// "sync"

	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"k8s.io/client-go/kubernetes"
)

type RunStatus string
type TaskStatus string
type OutboxEventType string

const (
	RunRunning   RunStatus = "RUNNING"
	RunSucceeded RunStatus = "SUCCEEDED"
	RunFailed    RunStatus = "FAILED"
	RunCanceled  RunStatus = "CANCELED"
)

const (
	TaskPending   TaskStatus = "PENDING"
	TaskQueued    TaskStatus = "QUEUED"
	TaskRunning   TaskStatus = "RUNNING"
	TaskSucceeded TaskStatus = "SUCCEEDED"
	TaskFailed    TaskStatus = "FAILED"
	TaskCanceled  TaskStatus = "CANCELED"
)

const (
	OutboxTaskReady      OutboxEventType = "TASK_READY"
	OutboxTaskRetryReady OutboxEventType = "TASK_RETRY_READY"
)

const (
	WorkerCount = 15
)

var (
	Conf *Kubeconfigs
)

type Workflow struct {
	WorkflowId uuid.UUID  `json:"workflow_id" db:"workflow_id"`
	EnvLink    string     `json:"env_link" db:"env_link"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	FinishedAt *time.Time `json:"finished_at" db:"finished_at"`
	Status     RunStatus  `json:"status" db:"status"`
}
type WorkflowRun struct {
	WorkflowId uuid.UUID `json:"workflow_id" db:"workflow_id"`
	RunId      uuid.UUID `json:"run_id" db:"run_id"`
	EnvLink    string    `json:"env_link" db:"env_link"`
}

type Task struct {
	TaskId     uuid.UUID `json:"task_id" db:"task_id"`
	WorkflowId uuid.UUID `json:"workflow_id" db:"workflow_id"`

	Name     string `json:"name" db:"name"`
	CodeLink string `json:"code_link" db:"code_link"`

	PendingPreds int               `json:"pending_preds" db:"pending_preds"`
	FuncArgMap   map[string]string `json:"func_arg_map" db:"func_arg_map"`
	Predecessors []string          `json:"predecessors" db:"predecessors"`
	Successors   []string          `json:"successors" db:"successors"`
	Status       TaskStatus        `json:"status" db:"status"`
	Attempt      int               `json:"attempt" db:"attempt"`
	MaxAttempts  int               `json:"max_attempts" db:"max_attempts"`
	CreatedAt    time.Time         `json:"created_at" db:"created_at"`
}
type TaskRun struct {
	TaskId     uuid.UUID `json:"task_id" db:"task_run_id"`
	RunId      uuid.UUID `json:"run_id" db:"run_id"`
	WorkflowId uuid.UUID `json:"workflow_id" db:"workflow_id"`

	Status     TaskStatus `json:"status" db:"status"`
	LastError  *string    `json:"last_error" db:"last_error"`
	LeaseOwner *int       `json:"lease_owner" db:"lease_owner"`
	LeaseUntil *time.Time `json:"lease_until" db:"lease_until"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at" db:"updated_at"`
}
type OutboxEvent struct {
	EventID    uuid.UUID       `json:"event_id" db:"event_id"`
	TaskID     uuid.UUID       `json:"task_id" db:"task_id"`
	WorkflowId uuid.UUID       `json:"workflow_id" db:"workflow_id"`
	Type       OutboxEventType `json:"event_type" db:"event_type"`
	Payload    any             `json:"payload" db:"payload"`

	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	PublishedAt      *time.Time `json:"published_at" db:"published_at"`
	ClaimedAt        *time.Time `json:"claimed_at" db:"claimed_at"`
	ClaimedBy        *int       `json:"claimed_by" db:"claimed_by"`
	PublishAttempts  int        `json:"publish_attempts" db:"publish_attempts"`
	LastPublishError *string    `json:"last_publish_error" db:"last_publish_error"`
}

type Kubeconfigs struct {
	K8s       kubernetes.Interface
	Namespace string
	ImageName string
}

type RabbitMQ struct {
	Conn          *amqp.Connection
	PublisherPool chan *amqp.Channel
}
