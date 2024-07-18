/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package lifecycle

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	kbagentcli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type actions struct {
	lifecycleActions *appsv1alpha1.ComponentLifecycleActions
	agentCli         kbagentcli.Client
	// jobExecutor      *jobExecutor
}

func (a *actions) PostProvision(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
	action := &postProvision{}
	if a.lifecycleActions.PostProvision == nil || a.lifecycleActions.PostProvision.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.PostProvision.CustomHandler, action, opts)
}

func (a *actions) PreTerminate(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
	action := &preTerminate{}
	if a.lifecycleActions.PreTerminate == nil || a.lifecycleActions.PreTerminate.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.PreTerminate.CustomHandler, action, opts)
}

// func (a *actions) RoleProbe(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
//	return nil, nil
// }

func (a *actions) Switchover(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
	action := &switchover{}
	if a.lifecycleActions.Switchover == nil || a.lifecycleActions.Switchover.WithoutCandidate == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.Switchover.WithoutCandidate, action, opts)
}

func (a *actions) MemberJoin(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
	action := &memberJoin{}
	if a.lifecycleActions.MemberJoin == nil || a.lifecycleActions.MemberJoin.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.MemberJoin.CustomHandler, action, opts)
}

func (a *actions) MemberLeave(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
	action := &memberLeave{}
	if a.lifecycleActions.MemberLeave == nil || a.lifecycleActions.MemberLeave.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.MemberLeave.CustomHandler, action, opts)
}

// func (a *actions) Readonly(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
//	return nil, nil
// }
//
// func (a *actions) Readwrite(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
//	return nil, nil
// }

func (a *actions) DataDump(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
	action := &dataDump{}
	if a.lifecycleActions.DataDump == nil || a.lifecycleActions.DataDump.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.DataDump.CustomHandler, action, opts)
}

func (a *actions) DataLoad(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
	action := &dataLoad{}
	if a.lifecycleActions.DataLoad == nil || a.lifecycleActions.DataLoad.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.DataLoad.CustomHandler, action, opts)
}

// func (a *actions) Reconfigure(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
//	return nil, nil
// }

func (a *actions) AccountProvision(ctx context.Context, cli client.Reader, opts *Options, args ...any) error {
	action := &accountProvision{args: args}
	if a.lifecycleActions.AccountProvision == nil || a.lifecycleActions.AccountProvision.CustomHandler == nil {
		return fmt.Errorf("action %s is not defined", action.name())
	}
	_, err := a.callAction(ctx, cli, a.lifecycleActions.AccountProvision.CustomHandler, action, opts)
	return err
}

func (a *actions) callAction(ctx context.Context, cli client.Reader,
	spec *appsv1alpha1.Action, action lifecycleAction, opts *Options) ([]byte, error) {
	if spec.Exec != nil && len(spec.Exec.Image) > 0 {
		return nil, fmt.Errorf("NotImplemented") // TODO: job executor
	}
	return a.callActionByKBAgent(ctx, cli, spec, action, opts)
}

func (a *actions) callActionByKBAgent(ctx context.Context, cli client.Reader,
	_ *appsv1alpha1.Action, action lifecycleAction, opts *Options) ([]byte, error) {
	req, err := a.buildActionRequest(ctx, cli, action, opts)
	if err != nil {
		return nil, err
	}
	rsp, err := a.agentCli.CallAction(ctx, *req)
	if err != nil {
		return nil, err
	}
	return rsp.Output, nil
}

func (a *actions) buildActionRequest(ctx context.Context, cli client.Reader, action lifecycleAction, opts *Options) (*proto.ActionRequest, error) {
	parameters, err := action.parameters(ctx, cli)
	if err != nil {
		return nil, err
	}

	req := &proto.ActionRequest{
		Action:     action.name(),
		Parameters: parameters,
	}
	if opts != nil {
		if opts.NonBlocking != nil {
			req.NonBlocking = opts.NonBlocking
		}
		if opts.TimeoutSeconds != nil {
			req.TimeoutSeconds = opts.TimeoutSeconds
		}
		if opts.RetryPolicy != nil {
			req.RetryPolicy = &proto.RetryPolicy{
				MaxRetries:    opts.RetryPolicy.MaxRetries,
				RetryInterval: opts.RetryPolicy.RetryInterval,
			}
		}
	}
	return req, nil
}

type lifecycleAction interface {
	name() string
	parameters(ctx context.Context, cli client.Reader) (map[string]string, error)
}
