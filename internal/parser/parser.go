package parser

import (
	// "bufio"
	"fmt"
	"regexp"
	"strings"
	"time"

	// "os"

	r "github.com/Sayan-995/dwop/internal/repository"
	u "github.com/Sayan-995/dwop/internal/utils"
	"github.com/google/uuid"
)

var (
	headerRe = regexp.MustCompile(`^fun\s+(\w+)\((.*?)\):`)
)

func ParseWorkflow(workflowId uuid.UUID, content []string) ([]u.Task, error) {
	var tasks []u.Task

	for i := 0; i < len(content); {
		line := content[i]
		if match := headerRe.FindStringSubmatch(line); match != nil {
			name := match[1]
			params := match[2]

			task := u.Task{
				TaskId:      uuid.New(),
				WorkflowId:  workflowId,
				Name:        name,
				FuncArgMap:  make(map[string]string),
				Status:      u.TaskPending,
				Attempt:     0,
				MaxAttempts: 5,
				CreatedAt:   time.Now(),
			}

			if strings.TrimSpace(params) != "" {
				for _, arg := range strings.Split(params, ",") {
					parts := strings.Split(strings.TrimSpace(arg), ":")
					if len(parts) != 2 {
						return nil, fmt.Errorf("invalid parameter syntax")
					}
					parts[0] = strings.TrimSpace(parts[0])
					parts[1] = strings.TrimSpace(parts[1])
					task.Predecessors = append(task.Predecessors, parts[1])
					task.FuncArgMap[parts[1]] = parts[0]
				}
			}
			currIndent := countIndent(line)
			i++
			var code string
			leadingIndent := countIndent(content[i])
			for ; i < len(content); i++ {
				line := content[i]
				if countIndent(line) > currIndent {
					if countIndent(line) < leadingIndent {
						return nil, fmt.Errorf("indentation error of the uplaoded file")
					}
					code += line[leadingIndent:] + "\n"
				} else {
					break
				}
			}
			_, err := r.StorageClient.UploadFile("Task_Code", fmt.Sprintf("%v/code", task.TaskId), strings.NewReader(code))
			if err != nil {
				return nil, fmt.Errorf("error while uploading code to supabase: %v", err)
			}
			task.CodeLink = fmt.Sprintf("%v/code", task.TaskId)
			task.PendingPreds = len(task.Predecessors)
			tasks = append(tasks, task)
		} else {
			i++
		}
	}
	for _, task := range tasks {
		for _, p := range task.Predecessors {
			for id, task2 := range tasks {
				if task2.Name == p {
					tasks[id].Successors = append(tasks[id].Successors, task.Name)
				}
			}
		}
	}
	return tasks, nil
}

func countIndent(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}
