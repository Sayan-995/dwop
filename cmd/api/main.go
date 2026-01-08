package api

import (
	"net/http"

	"github.com/Sayan-995/dwop/internal/controllers"
	"github.com/gorilla/mux"
)

func NewServer(addr string) *http.Server {
	r := mux.NewRouter()
	r.HandleFunc("/health", controllers.Health).Methods(http.MethodGet)
	r.HandleFunc("/upload", controllers.UploadWorkflow).Methods(http.MethodPost)
	r.HandleFunc("/update", controllers.UpdateWorkflow).Methods(http.MethodPost)
	r.HandleFunc("/cancel", controllers.CancelWorkflow).Methods(http.MethodPost)
	return &http.Server{Addr: addr, Handler: r}
}
