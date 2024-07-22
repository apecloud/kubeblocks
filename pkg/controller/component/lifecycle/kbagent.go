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

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
)

type lifecycleAction interface {
	name() string
	parameters(ctx context.Context, cli client.Reader) (map[string]string, error)
}

type kbagent struct {
	lifecycleActions *appsv1alpha1.ComponentLifecycleActions
	agentCli         kbacli.Client
}

func (a *kbagent) PostProvision(ctx context.Context, cli client.Reader, opts *Options) error {
	la := &postProvision{}
	if a.lifecycleActions.PostProvision == nil || a.lifecycleActions.PostProvision.CustomHandler == nil {
		return errors.Wrap(ErrActionNotDefined, la.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.PostProvision.CustomHandler, la, opts)
}

func (a *kbagent) PreTerminate(ctx context.Context, cli client.Reader, opts *Options) error {
	la := &preTerminate{}
	if a.lifecycleActions.PreTerminate == nil || a.lifecycleActions.PreTerminate.CustomHandler == nil {
		return errors.Wrap(ErrActionNotDefined, la.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.PreTerminate.CustomHandler, la, opts)
}

// func (a *kbagent) RoleProbe(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
//	return nil, nil
// }

func (a *kbagent) Switchover(ctx context.Context, cli client.Reader, opts *Options) error {
	la := &switchover{}
	if a.lifecycleActions.Switchover == nil || a.lifecycleActions.Switchover.WithoutCandidate == nil {
		return errors.Wrap(ErrActionNotDefined, la.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.Switchover.WithoutCandidate, la, opts)
}

func (a *kbagent) MemberJoin(ctx context.Context, cli client.Reader, opts *Options) error {
	la := &memberJoin{}
	if a.lifecycleActions.MemberJoin == nil || a.lifecycleActions.MemberJoin.CustomHandler == nil {
		return errors.Wrap(ErrActionNotDefined, la.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.MemberJoin.CustomHandler, la, opts)
}

func (a *kbagent) MemberLeave(ctx context.Context, cli client.Reader, opts *Options) error {
	la := &memberLeave{}
	if a.lifecycleActions.MemberLeave == nil || a.lifecycleActions.MemberLeave.CustomHandler == nil {
		return errors.Wrap(ErrActionNotDefined, la.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.MemberLeave.CustomHandler, la, opts)
}

// func (a *kbagent) Readonly(ctx context.Context, cli client.Reader, opts *Options) error {
//	return nil
// }
//
// func (a *kbagent) Readwrite(ctx context.Context, cli client.Reader, opts *Options) error {
//	return nil
// }

func (a *kbagent) DataDump(ctx context.Context, cli client.Reader, opts *Options) error {
	la := &dataDump{}
	if a.lifecycleActions.DataDump == nil || a.lifecycleActions.DataDump.CustomHandler == nil {
		return errors.Wrap(ErrActionNotDefined, la.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.DataDump.CustomHandler, la, opts)
}

func (a *kbagent) DataLoad(ctx context.Context, cli client.Reader, opts *Options) error {
	la := &dataLoad{}
	if a.lifecycleActions.DataLoad == nil || a.lifecycleActions.DataLoad.CustomHandler == nil {
		return errors.Wrap(ErrActionNotDefined, la.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.DataLoad.CustomHandler, la, opts)
}

// func (a *kbagent) Reconfigure(ctx context.Context, cli client.Reader, opts *Options) error {
//	return nil
// }

func (a *kbagent) AccountProvision(ctx context.Context, cli client.Reader, opts *Options, args ...any) error {
	la := &accountProvision{args: args}
	if a.lifecycleActions.AccountProvision == nil || a.lifecycleActions.AccountProvision.CustomHandler == nil {
		return errors.Wrap(ErrActionNotDefined, la.name())
	}
	return a.callAction(ctx, cli, a.lifecycleActions.AccountProvision.CustomHandler, la, opts)
}

func (a *kbagent) callAction(ctx context.Context, cli client.Reader, spec *appsv1alpha1.Action, la lifecycleAction, opts *Options) error {
	_, err := a.callActionByKBAgent(ctx, cli, spec, la, opts)
	return err
}

func (a *kbagent) callActionByKBAgent(ctx context.Context, cli client.Reader,
	_ *appsv1alpha1.Action, la lifecycleAction, opts *Options) ([]byte, error) {
	req, err := a.buildActionRequest(ctx, cli, la, opts)
	if err != nil {
		return nil, err
	}
	rsp, err := a.agentCli.CallAction(ctx, *req)
	if err != nil {
		return nil, a.error2(la, err)
	}
	return rsp.Output, nil
}

func (a *kbagent) buildActionRequest(ctx context.Context, cli client.Reader, la lifecycleAction, opts *Options) (*proto.ActionRequest, error) {
	parameters, err := la.parameters(ctx, cli)
	if err != nil {
		return nil, err
	}

	req := &proto.ActionRequest{
		Action:     la.name(),
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

func (a *kbagent) error2(la lifecycleAction, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, service.ErrNotDefined):
		return errors.Wrap(ErrActionNotDefined, la.name())
	case errors.Is(err, service.ErrNotImplemented):
		return errors.Wrap(ErrActionNotImplemented, la.name())
	case errors.Is(err, service.ErrInProgress):
		return errors.Wrap(ErrActionInProgress, la.name())
	case errors.Is(err, service.ErrBusy):
		return errors.Wrap(ErrActionBusy, la.name())
	default: // TODO: timeout, failed & internal error
		return err
	}
}
