package ha

import (
	"context"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/configuration_store"
)

type DB interface {
	Promote(podName string) error
	Demote(podName string) error

	GetStatus(ctx context.Context) (string, error)
	GetExtra(ctx context.Context) (map[string]string, error)
	GetOpTime(ctx context.Context) (int64, error)
	IsLeader(ctx context.Context) bool
	IsHealthiest(ctx context.Context, podName string) bool
	HandleFollow(ctx context.Context, leader *configuration_store.Leader, podName string) error

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
	IsRunning(ctx context.Context) bool
}
