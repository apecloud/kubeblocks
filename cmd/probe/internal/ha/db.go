package ha

import (
	"context"
)

type DB interface {
	Promote(podName string) error
	Demote(podName string) error

	GetStatus(ctx context.Context) (string, error)
	GetExtra(ctx context.Context) (map[string]string, error)
	IsLeader(ctx context.Context) bool
	IsHealthiest(ctx context.Context, podName string) bool

	DbConn
	DbTool
	ProcessControl
}

type DbConn interface {
	GetSysID(ctx context.Context) (string, error)
}

type DbTool interface {
	ExecCmd(ctx context.Context, podName, cmd string) (map[string]string, error)
}

type ProcessControl interface {
	Stop(ctx context.Context) error
}
