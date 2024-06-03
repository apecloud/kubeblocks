package exec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/dcs"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

func TestHandler_JoinMember(t *testing.T) {
	handler := &Handler{
		HandlerBase: handlers.HandlerBase{},
		Executor:    &util.ExecutorImpl{},
		actionCommands: map[string][]string{
			constant.MemberJoinAction: {"join-member-command"},
		},
	}

	cluster := &dcs.Cluster{}
	memberName := "member-1"

	err := handler.JoinMember(context.Background(), cluster, memberName)

	assert.NoError(t, err)
}

func TestHandler_LeaveMember(t *testing.T) {
	handler := &Handler{
		HandlerBase: handlers.HandlerBase{},
		Executor:    &util.ExecutorImpl{},
		actionCommands: map[string][]string{
			constant.MemberLeaveAction: {"leave-member-command"},
		},
	}

	cluster := &dcs.Cluster{}
	memberName := "member-1"

	err := handler.LeaveMember(context.Background(), cluster, memberName)

	assert.NoError(t, err)
}

func TestHandler_MemberHealthCheck(t *testing.T) {
	handler := &Handler{
		HandlerBase: handlers.HandlerBase{},
		Executor:    &util.ExecutorImpl{},
		actionCommands: map[string][]string{
			constant.CheckHealthyAction: {"member-health-check-command"},
		},
	}

	cluster := &dcs.Cluster{}
	member := &dcs.Member{}

	err := handler.MemberHealthCheck(context.Background(), cluster, member)

	assert.NoError(t, err)
}

func TestHandler_Lock(t *testing.T) {
	handler := &Handler{
		HandlerBase: handlers.HandlerBase{},
		Executor:    &util.ExecutorImpl{},
		actionCommands: map[string][]string{
			constant.ReadonlyAction: {"lock-command"},
		},
	}

	reason := "lock-reason"

	err := handler.Lock(context.Background(), reason)

	assert.NoError(t, err)
}

func TestHandler_Unlock(t *testing.T) {
	handler := &Handler{
		HandlerBase: handlers.HandlerBase{},
		Executor:    &util.ExecutorImpl{},
		actionCommands: map[string][]string{
			constant.ReadWriteAction: {"unlock-command"},
		},
	}

	reason := "unlock-reason"

	err := handler.Unlock(context.Background(), reason)

	assert.NoError(t, err)
}

func TestHandler_PostProvision(t *testing.T) {
	handler := &Handler{
		HandlerBase: handlers.HandlerBase{},
		Executor:    &util.ExecutorImpl{},
		actionCommands: map[string][]string{
			constant.PostProvisionAction: {"post-provision-command"},
		},
	}

	cluster := &dcs.Cluster{}

	err := handler.PostProvision(context.Background(), cluster)

	assert.NoError(t, err)
}

func TestHandler_PreTerminate(t *testing.T) {
	handler := &Handler{
		HandlerBase: handlers.HandlerBase{},
		Executor:    &util.ExecutorImpl{},
		actionCommands: map[string][]string{
			constant.PreTerminateAction: {"pre-terminate-command"},
		},
	}

	cluster := &dcs.Cluster{}

	err := handler.PreTerminate(context.Background(), cluster)

	assert.NoError(t, err)
}
