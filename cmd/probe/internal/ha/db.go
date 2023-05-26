package ha

import (
	"context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type DB interface {
	Promote() error
	Demote() error

	GetSysID(ctx context.Context) (string, error)
	GetState(ctx context.Context) (string, error)
	GetExtra(ctx context.Context) map[string]string

	ExecCmd(ctx context.Context, clientSet *kubernetes.Clientset, config *rest.Config, podName, namespace, cmd string) (map[string]string, error)
	Stop(ctx context.Context) error
}

type DbConn interface {
}

type DbTool interface {
}

type ProcessControl interface {
}
