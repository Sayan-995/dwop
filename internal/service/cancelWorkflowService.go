package service

import (
	"github.com/Sayan-995/dwop/internal/observer"
	"github.com/Sayan-995/dwop/internal/repository"
	"github.com/Sayan-995/dwop/internal/utils"
)

func CancelWorkflow(workflowId string) error {
	err := repository.CancelWorkflowById(workflowId)
	if err != nil {
		return err
	}
	return observer.ForceStopJobs(utils.Conf.K8s, utils.Conf.Namespace, workflowId)
}
