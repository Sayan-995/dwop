package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Sayan-995/dwop/internal/repository"
	"github.com/Sayan-995/dwop/internal/utils"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	expiry = 30 * 60 * 60
)

func normalizeURL(raw string) string {
	godotenv.Load()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	projectBase := strings.TrimRight(strings.TrimSpace(os.Getenv("SUPABASE_PROJECT_URL")), "/")
	if projectBase == "" {
		return raw
	}
	storageBase := projectBase + "/storage/v1"

	trimmed := strings.TrimPrefix(raw, "/")
	if strings.HasPrefix(trimmed, "object/") {
		return storageBase + "/" + trimmed
	}
	if strings.HasPrefix(trimmed, "storage/v1/") {
		return projectBase + "/" + trimmed
	}

	if strings.HasPrefix(raw, "/") {
		return projectBase + raw
	}
	return projectBase + "/" + raw
}

func CreateJob(k8s kubernetes.Interface, namespace, imageName string, workflow utils.Workflow,
	task utils.Task, runID uuid.UUID) (*batchv1.Job, error) {
	codeUrl, err := repository.StorageClient.CreateSignedUrl("Task_Code", task.CodeLink, expiry)
	if err != nil {
		return nil, fmt.Errorf("error while creating signed url: %v", err)
	}
	codeSignedURL := normalizeURL(codeUrl.SignedURL)
	requirementsUrl, err := repository.StorageClient.CreateSignedUrl("Workflow_Env", workflow.EnvLink, expiry)
	if err != nil {
		return nil, fmt.Errorf("error while creating signed url: %v", err)
	}
	reqSignedURL := normalizeURL(requirementsUrl.SignedURL)
	predUrls := map[string]any{}
	for _, pred := range task.Predecessors {
		outputUrl, err := repository.StorageClient.CreateSignedUrl("Task_Output", fmt.Sprintf("%s/%s/output.txt", task.WorkflowId, pred), expiry)
		if err != nil {
			return nil, fmt.Errorf("error while creating signed url: %v", err)
		}
		predUrls[pred] = normalizeURL(outputUrl.SignedURL)
	}

	outputUrl, err := repository.StorageClient.CreateSignedUploadUrl("Task_Output", fmt.Sprintf("%s/%s/output.txt", task.WorkflowId, task.Name))
	if err != nil {
		return nil, fmt.Errorf("error while creating signed upload url: %v", err)
	}
	if strings.TrimSpace(outputUrl.Url) == "" {
		return nil, fmt.Errorf("signed upload url is empty")
	}
	outputSignedUploadURL := normalizeURL(outputUrl.Url)
	predUrlsJson, _ := json.Marshal(predUrls)
	funcArgMapJson, _ := json.Marshal(task.FuncArgMap)

	jobName := strings.ToLower(runID.String())
	backoff := int32(0)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":        "dwop",
				"runID":      runID.String(),
				"workflowId": workflow.WorkflowId.String(),
				"taskId":     task.TaskId.String(),
				"taskName":   task.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoff,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":   "dwop",
						"runID": runID.String(),
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "worker",
							Image:           imageName,
							ImagePullPolicy: corev1.PullNever,
							Env: []corev1.EnvVar{
								{Name: "RUN_ID", Value: runID.String()},
								{Name: "WORKFLOW_ID", Value: workflow.WorkflowId.String()},
								{Name: "TASK_ID", Value: task.TaskId.String()},
								{Name: "TASK_NAME", Value: task.Name},
								{Name: "CODE_URL", Value: codeSignedURL},
								{Name: "REQ_URL", Value: reqSignedURL},
								{Name: "PRED_URLS_JSON", Value: string(predUrlsJson)},
								{Name: "FUNC_ARG_MAP_JSON", Value: string(funcArgMapJson)},
								{Name: "OUTPUT_SIGNED_URL", Value: outputSignedUploadURL},
							},
						},
					},
				},
			},
		},
	}
	fmt.Printf("[CreateJob] Creating job %s in namespace %s with image %s\n", jobName, namespace, imageName)
	fmt.Printf("[CreateJob] Task: %s, Workflow: %s, RunID: %s\n", task.TaskId, workflow.WorkflowId, runID)
	created, err := k8s.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			fmt.Printf("[CreateJob] Job already exists: %s\n", jobName)
			return job, nil
		}
		fmt.Printf("[CreateJob] ERROR creating job %s: %v\n", jobName, err)
		return nil, err
	}
	fmt.Printf("[CreateJob] Successfully created job: %s\n", jobName)
	return created, nil
}
