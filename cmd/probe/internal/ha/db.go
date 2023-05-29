package ha

import (
	"context"
)

type DB interface {
	Promote() error
	Demote() error

	GetStatus(ctx context.Context) (string, error)
	GetExtra(ctx context.Context) map[string]string

	DbConn
	DbTool
	ProcessControl
}

type DbConn interface {
	GetSysID(ctx context.Context) (string, error)
}

type DbTool interface {
	ExecCmd(ctx context.Context, podName, namespace, cmd string) (map[string]string, error)
}

type ProcessControl interface {
	Stop(ctx context.Context) error
}
