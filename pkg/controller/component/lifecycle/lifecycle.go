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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

type Options struct {
	NonBlocking    *bool
	TimeoutSeconds *int32
	RetryPolicy    *appsv1alpha1.RetryPolicy
}

type Lifecycle interface {
	PostProvision(ctx context.Context, cli client.Reader, opts *Options) error

	PreTerminate(ctx context.Context, cli client.Reader, opts *Options) error

	// RoleProbe(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error)

	Switchover(ctx context.Context, cli client.Reader, opts *Options, candidate string) error

	MemberJoin(ctx context.Context, cli client.Reader, opts *Options) error

	MemberLeave(ctx context.Context, cli client.Reader, opts *Options) error

	// Readonly(ctx context.Context, cli client.Reader, opts *Options) error

	// Readwrite(ctx context.Context, cli client.Reader, opts *Options) error

	DataDump(ctx context.Context, cli client.Reader, opts *Options) error

	DataLoad(ctx context.Context, cli client.Reader, opts *Options) error

	// Reconfigure(ctx context.Context, cli client.Reader, opts *Options) error

	AccountProvision(ctx context.Context, cli client.Reader, opts *Options, statement, user, password string) error
}

func New(synthesizedComp *component.SynthesizedComponent, pod *corev1.Pod, pods ...*corev1.Pod) (Lifecycle, error) {
	if pod == nil && len(pods) == 0 {
		return nil, fmt.Errorf("either pod or pods must be provided to call lifecycle actions")
	}
	if pod == nil {
		pod = pods[0]
	}
	if len(pods) == 0 {
		pods = []*corev1.Pod{pod}
	}
	return &kbagent{
		synthesizedComp: synthesizedComp,
		pods:            pods,
		pod:             pod,
	}, nil
}

func NewWithCompDef(namespace, clusterName, clusterUID, compName string,
	compDef *appsv1alpha1.ComponentDefinition, pod *corev1.Pod, pods ...*corev1.Pod) (Lifecycle, error) {
	if pod == nil && len(pods) == 0 {
		return nil, fmt.Errorf("either pod or pods must be provided to call lifecycle actions")
	}
	if pod == nil {
		pod = pods[0]
	}
	if len(pods) == 0 {
		pods = []*corev1.Pod{pod}
	}
	return &kbagent{
		synthesizedComp: &component.SynthesizedComponent{
			Namespace:        namespace,
			ClusterName:      clusterName,
			ClusterUID:       clusterUID,
			Name:             compName,
			Roles:            compDef.Spec.Roles,
			LifecycleActions: compDef.Spec.LifecycleActions,
		},
		pods: pods,
		pod:  pod,
	}, nil
}
