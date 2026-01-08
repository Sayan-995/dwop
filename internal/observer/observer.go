package observer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Sayan-995/dwop/internal/repository"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func Run(ctx context.Context, k8s kubernetes.Interface, namespace string) {
	fmt.Printf("[Observer] Starting observer for namespace %s\n", namespace)
	resyncInterval := 10 * time.Minute
	for {
		if err := Observe(ctx, k8s, namespace, resyncInterval); err != nil {
			fmt.Printf("[Observer] ERROR: %v\n", err)
			time.Sleep(2 * time.Second)
		}
	}
}

func Observe(ctx context.Context, k8s kubernetes.Interface, namespace string, resync time.Duration) error {
	fmt.Printf("[Observer] Starting watch on namespace %s\n", namespace)
	if err := ReconcileJob(ctx, k8s, namespace); err != nil {
		fmt.Printf("[Observer] Reconcile error: %v\n", err)
		return err
	}

	w, err := k8s.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "app=dwop",
	})
	if err != nil {
		fmt.Printf("[Observer] Watch error: %v\n", err)
		return err
	}
	defer w.Stop()

	resyncTicker := time.NewTicker(resync)
	defer resyncTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[Observer] Context cancelled\n")
			return ctx.Err()
		case <-resyncTicker.C:
			fmt.Printf("[Observer] Resync tick - reconciling jobs\n")
			_ = ReconcileJob(ctx, k8s, namespace)
		case ev, ok := <-w.ResultChan():
			if !ok {
				fmt.Printf("[Observer] Watch channel closed\n")
				return nil
			}
			if ev.Type == watch.Deleted {
				fmt.Printf("[Observer] Job event: %s (ignored)\n", ev.Type)
				continue
			}
			job, ok := ev.Object.(*batchv1.Job)
			if !ok {
				fmt.Printf("[Observer] Received non-Job object\n")
				continue
			}
			fmt.Printf("[Observer] Job event: %s, Job: %s\n", ev.Type, job.Name)
			HandleJobStatus(job, k8s, namespace)
		}
	}
}

func ReconcileJob(ctx context.Context, k8s kubernetes.Interface, namespace string) error {
	fmt.Printf("[Observer] Reconciling jobs in namespace %s\n", namespace)
	jobs, err := k8s.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=dwop",
	})
	if err != nil {
		fmt.Printf("[Observer] List jobs error: %v\n", err)
		return err
	}
	fmt.Printf("[Observer] Found %d jobs to reconcile\n", len(jobs.Items))
	for i := range jobs.Items {
		HandleJobStatus(&jobs.Items[i], k8s, namespace)
	}
	return nil
}

func HandleJobStatus(job *batchv1.Job, k8s kubernetes.Interface, namespace string) {
	runId := job.Labels["runID"]
	if runId == "" {
		fmt.Printf("[Observer] Job %s has no runID label\n", job.Name)
		return
	}
	fmt.Printf("[Observer] Handling job %s (runID: %s), conditions: %d\n", job.Name, runId, len(job.Status.Conditions))

	var isCompleted bool
	var isFailed bool

	for _, c := range job.Status.Conditions {
		fmt.Printf("[Observer] Job %s condition: Type=%s, Status=%v\n", job.Name, c.Type, c.Status)
		if c.Status != corev1.ConditionTrue {
			continue
		}
		if c.Type == batchv1.JobComplete {
			isCompleted = true
		}
		if c.Type == batchv1.JobFailed {
			isFailed = true
		}
	}

	if isFailed {
		fmt.Printf("[Observer] Job %s FAILED, getting error message for runID %s\n", job.Name, runId)
		var errmsg string
		pods, err := k8s.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: "job-name=" + job.Name,
		})
		if err != nil {
			errmsg = fmt.Sprintf("job failed (error listing pods): %v", err)
			fmt.Printf("[Observer] Could not list pods: %v\n", err)
		} else if len(pods.Items) == 0 {
			errmsg = "job failed (no pods found)"
			fmt.Printf("[Observer] No pods found for job\n")
		} else {
			pod := pods.Items[0]
			fmt.Printf("[Observer] Pod %s status phase: %s\n", pod.Name, pod.Status.Phase)

			if len(pod.Status.ContainerStatuses) > 0 {
				cs := pod.Status.ContainerStatuses[0]
				fmt.Printf("[Observer] Container status - Ready: %v, State: %+v\n", cs.Ready, cs.State)
				if cs.State.Terminated != nil {
					errmsg = fmt.Sprintf("Exit code: %d, Reason: %s, Message: %s",
						cs.State.Terminated.ExitCode,
						cs.State.Terminated.Reason,
						cs.State.Terminated.Message)
				} else if cs.State.Waiting != nil {
					errmsg = fmt.Sprintf("Waiting: %s - %s", cs.State.Waiting.Reason, cs.State.Waiting.Message)
				} else {
					errmsg = "Running, but job failed - check pod logs"
				}
			} else {
				errmsg = fmt.Sprintf("No container statuses found, pod phase: %s", pod.Status.Phase)
			}

			logReq := k8s.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: "worker", TailLines: func() *int64 { v := int64(200); return &v }()})
			logBytes, logErr := logReq.DoRaw(context.Background())
			if logErr != nil {
				fmt.Printf("[Observer] Could not read pod logs: %v\n", logErr)
			} else {
				logs := strings.TrimSpace(string(logBytes))
				if logs != "" {
					errmsg = fmt.Sprintf("%s\n--- pod logs ---\n%s", errmsg, logs)
				}
			}
			fmt.Printf("[Observer] Pod error message: %s\n", errmsg)
		}

		fmt.Printf("[Observer] Calling increase_attempt RPC with error: %s\n", errmsg)
		err = repository.IncreaseAttempt(runId, errmsg)
		if err != nil {
			fmt.Printf("[Observer] ERROR in increase_attempt: %v\n", err)
		} else {
			fmt.Printf("[Observer] Attempt increased for runID %s\n", runId)
			policy := metav1.DeletePropagationBackground
			grace := int64(0)
			delErr := k8s.BatchV1().Jobs(namespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{
				PropagationPolicy:  &policy,
				GracePeriodSeconds: &grace,
			})
			if delErr != nil && !apierrors.IsNotFound(delErr) {
				fmt.Printf("[Observer] ERROR deleting failed job %s: %v\n", job.Name, delErr)
			}
		}
		return
	}

	if isCompleted {
		fmt.Printf("[Observer] Job %s COMPLETED, calling complete RPC for runID %s\n", job.Name, runId)
		pods, podErr := k8s.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: "job-name=" + job.Name,
		})
		if podErr != nil {
			fmt.Printf("[Observer] Could not list pods for completed job logs: %v\n", podErr)
		} else if len(pods.Items) > 0 {
			pod := pods.Items[0]
			logReq := k8s.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: "worker", TailLines: func() *int64 { v := int64(200); return &v }()})
			logBytes, logErr := logReq.DoRaw(context.Background())
			if logErr != nil {
				fmt.Printf("[Observer] Could not read completed pod logs: %v\n", logErr)
			} else {
				logs := strings.TrimSpace(string(logBytes))
				if logs != "" {
					fmt.Printf("[Observer] Completed job pod logs (tail):\n%s\n", logs)
				}
			}
		}
		err := repository.CompleteRunAndEnqueueSuccessors(runId)
		if err != nil {
			fmt.Printf("[Observer] ERROR completing job %s: %v\n", job.Name, err)
		} else {
			fmt.Printf("[Observer] Job %s marked as completed\n", job.Name)
			policy := metav1.DeletePropagationBackground
			grace := int64(0)
			delErr := k8s.BatchV1().Jobs(namespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{
				PropagationPolicy:  &policy,
				GracePeriodSeconds: &grace,
			})
			if delErr != nil && !apierrors.IsNotFound(delErr) {
				fmt.Printf("[Observer] ERROR deleting completed job %s: %v\n", job.Name, delErr)
			}
		}
		return
	}
}

func ForceStopJobs(k8s kubernetes.Interface, namespace, workflowId string) error {
	policy := metav1.DeletePropagationBackground
	grace := int64(0)

	err := k8s.BatchV1().Jobs(namespace).DeleteCollection(context.Background(), metav1.DeleteOptions{
		PropagationPolicy:  &policy,
		GracePeriodSeconds: &grace,
	}, metav1.ListOptions{
		LabelSelector: "app=dwop,workflowId=" + workflowId,
	})
	return err
}
