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
	"errors"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var (
	ErrActionNotDefined     = errors.New("action is not defined")
	ErrActionNotImplemented = errors.New("action is not implemented")
	ErrActionInProgress     = errors.New("action is in progress")
	ErrActionBusy           = errors.New("action is busy")
	ErrActionTimeout        = errors.New("action timeout")
	ErrActionFailed         = errors.New("action failed")
	ErrActionCanceled       = errors.New("action canceled")
	ErrActionInternalError  = errors.New("action internal error")
)

func New(lifecycleActions *appsv1alpha1.ComponentLifecycleActions, pod *corev1.Pod, pods ...*corev1.Pod) (Lifecycle, error) {
	if len(pods) == 0 {
		pods = []*corev1.Pod{pod}
	}
	return &kbagent{
		lifecycleActions: lifecycleActions,
		pods:             pods,
		pod:              pod,
	}, nil
}

type Options struct {
	NonBlocking    *bool
	TimeoutSeconds *int32
	RetryPolicy    *appsv1alpha1.RetryPolicy
}

type Lifecycle interface {
	PostProvision(ctx context.Context, cli client.Reader, opts *Options) error

	PreTerminate(ctx context.Context, cli client.Reader, opts *Options) error

	// RoleProbe(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error)

	Switchover(ctx context.Context, cli client.Reader, opts *Options) error

	MemberJoin(ctx context.Context, cli client.Reader, opts *Options) error

	MemberLeave(ctx context.Context, cli client.Reader, opts *Options) error

	// Readonly(ctx context.Context, cli client.Reader, opts *Options) error

	// Readwrite(ctx context.Context, cli client.Reader, opts *Options) error

	DataDump(ctx context.Context, cli client.Reader, opts *Options) error

	DataLoad(ctx context.Context, cli client.Reader, opts *Options) error

	// Reconfigure(ctx context.Context, cli client.Reader, opts *Options) error

	AccountProvision(ctx context.Context, cli client.Reader, opts *Options, args ...any) error
}
