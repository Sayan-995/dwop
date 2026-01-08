package service

import (
	"os"

	"github.com/Sayan-995/dwop/internal/utils"
)

func UpdateWorkflow(workflowId string, file *os.File, requirements *os.File) (*utils.Workflow, error) {
	err := CancelWorkflow(workflowId)
	if err != nil {
		return nil, err
	}
	return UploadWorkflowfile(file, requirements)
}
