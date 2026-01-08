package jobobserver

import (
	"context"
	"time"

	"github.com/Sayan-995/dwop/internal/observer"
	"k8s.io/client-go/kubernetes"
)

func Run(ctx context.Context, k8s kubernetes.Interface, namespace string) error {
	resync := 10 * time.Minute
	return observer.Observe(ctx, k8s, namespace, resync)
}
