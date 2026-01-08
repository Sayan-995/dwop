package service

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	p "github.com/Sayan-995/dwop/internal/parser"
	repo "github.com/Sayan-995/dwop/internal/repository"
	u "github.com/Sayan-995/dwop/internal/utils"
	"github.com/google/uuid"
)

func UploadWorkflowfile(file *os.File, requirements *os.File) (*u.Workflow, error) {

	byteContent, err := io.ReadAll(file)

	if err != nil {
		return nil, fmt.Errorf("error while reading contents from file: %v", err)
	}

	workflow := u.Workflow{
		WorkflowId: uuid.New(),
		CreatedAt:  time.Now(),
		Status:     u.RunRunning,
	}

	_, err = repo.StorageClient.UploadFile("Workflow_Env", fmt.Sprintf("%v/env", workflow.WorkflowId), requirements)

	if err != nil {
		return nil, fmt.Errorf("error while uploading requirements to supabase: %v", err)
	}

	workflow.EnvLink = fmt.Sprintf("%v/env", workflow.WorkflowId)

	content := strings.Split(string(byteContent), "\n")

	tasks, err := p.ParseWorkflow(workflow.WorkflowId, content)

	if err != nil {
		return nil, fmt.Errorf("Error while generating tasks: %v", err)
	}

	err = repo.InsertWorkflow(workflow, tasks)

	return &workflow, err
}
