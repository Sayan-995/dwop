package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	api "github.com/Sayan-995/dwop/cmd/api"
	inboxpublisher "github.com/Sayan-995/dwop/cmd/inbox-publisher"
	jobobserver "github.com/Sayan-995/dwop/cmd/job-observer"
	outboxclaimer "github.com/Sayan-995/dwop/cmd/outbox-claimer"
	"github.com/Sayan-995/dwop/internal/utils"
	"github.com/joho/godotenv"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	godotenv.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	if v := os.Getenv("KUBECONFIG"); v != "" {
		kubeconfig = v
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("error setting up the config: %v", err)
	}
	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("error setting up the K8s client: %v", err)
	}

	namespace := os.Getenv("DWOP_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
	namespace = strings.Trim(namespace, "\"")
	imageName := os.Getenv("DWOP_IMAGE")
	if imageName == "" {
		log.Fatalf("DWOP_IMAGE must be set")
	}
	utils.Conf = &utils.Kubeconfigs{K8s: k8s, Namespace: namespace, ImageName: imageName}

	port := os.Getenv("DWOP_PORT")
	if port == "" {
		port = "8080"
	}

	srv := api.NewServer(":" + port)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("api server error: %v", err)
		}
	}()

	go inboxpublisher.Run(ctx)
	go outboxclaimer.Run(ctx)
	go func() {
		err := jobobserver.Run(ctx, k8s, namespace)
		if err != nil && ctx.Err() == nil {
			log.Printf("observer error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
