package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Sayan-995/dwop/internal/service"
)

func UploadWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("invalid multipart form: %w", err))
		return
	}

	workflowFile, err := multipartToTempFile(r, "file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	defer os.Remove(workflowFile.Name())
	defer workflowFile.Close()

	reqFile, err := multipartToTempFile(r, "requirements")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	defer os.Remove(reqFile.Name())
	defer reqFile.Close()

	workflow, err := service.UploadWorkflowfile(workflowFile, reqFile)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, workflow)
}

func UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("invalid multipart form: %w", err))
		return
	}

	workflowID := r.FormValue("workflowId")
	if workflowID == "" {
		workflowID = r.URL.Query().Get("workflowId")
	}
	if workflowID == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("missing workflowId"))
		return
	}

	workflowFile, err := multipartToTempFile(r, "file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	defer os.Remove(workflowFile.Name())
	defer workflowFile.Close()

	reqFile, err := multipartToTempFile(r, "requirements")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	defer os.Remove(reqFile.Name())
	defer reqFile.Close()

	workflow, err := service.UpdateWorkflow(workflowID, workflowFile, reqFile)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, workflow)
}

func CancelWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	workflowID := r.URL.Query().Get("workflowId")
	if workflowID == "" {
		workflowID = r.FormValue("workflowId")
	}
	if workflowID == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("missing workflowId"))
		return
	}

	if err := service.CancelWorkflow(workflowID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func multipartToTempFile(r *http.Request, field string) (*os.File, error) {
	src, _, err := r.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("missing %s file: %w", field, err)
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "dwop-*")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, err
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, err
	}
	return tmp, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}
