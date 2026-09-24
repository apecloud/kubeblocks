/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package operations

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// switchoverRuntime contains the data-plane access that Switchover still
// needs. It is deliberately private so other Ops cannot build a shared
// dependency on Pods or lifecycle actions.
type switchoverRuntime struct {
	ctx          context.Context
	cli          client.Client
	multiCluster bool
	dataCtx      context.Context
	dataGetOpts  []client.GetOption
	dataListOpts []client.ListOption
}

func newSwitchoverRuntime(ctx context.Context, cli client.Client, cluster *appsv1.Cluster) *switchoverRuntime {
	if ctx == nil {
		ctx = context.Background()
	}
	placement := ""
	if cluster != nil && cluster.Annotations != nil && enabledMultiCluster(cluster) {
		placement = cluster.Annotations[constant.KBAppMultiClusterPlacementKey]
	}
	r := &switchoverRuntime{
		ctx:          ctx,
		cli:          cli,
		multiCluster: len(strings.TrimSpace(placement)) > 0,
	}
	if !r.multiCluster {
		return r
	}
	r.dataCtx = multicluster.IntoContext(ctx, placement)
	r.dataGetOpts = []client.GetOption{multicluster.InDataContext()}
	r.dataListOpts = []client.ListOption{multicluster.InDataContext()}
	return r
}

func enabledMultiCluster(obj client.Object) bool {
	return multicluster.Enabled4Object(obj)
}

func (r *switchoverRuntime) getInstance(namespace, clusterName, compName, instanceName string) (*switchoverInstance, error) {
	pod := &corev1.Pod{}
	if err := r.cli.Get(r.dataContext(), client.ObjectKey{Name: instanceName, Namespace: namespace}, pod, r.dataGetOpts...); err != nil {
		return nil, err
	}
	if pod.Labels[constant.AppInstanceLabelKey] != clusterName || pod.Labels[constant.KBAppComponentLabelKey] != compName {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" does not belong to component "%s"`, instanceName, compName))
	}
	return &switchoverInstance{pod: pod}, nil
}

func (r *switchoverRuntime) switchover(ctx context.Context, synthesizedComp *component.SynthesizedComponent, instanceName, candidateName string) error {
	return r.doSwitchover(ctx, r.cli, synthesizedComp, instanceName, candidateName)
}

// We consider a switchover action succeeds if the action returns without error.
// We don't need to know if a switchover is actually executed.
func (r *switchoverRuntime) doSwitchover(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent,
	instanceName, candidateName string) error {
	pods, err := component.ListOwnedPods(r.dataContext(), cli, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, r.dataListOpts...)
	if err != nil {
		return err
	}

	pod := &corev1.Pod{}
	for _, p := range pods {
		if p.Name == instanceName {
			pod = p
			break
		}
	}
	if pod.Name == "" {
		return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" not found`, instanceName))
	}
	if candidateName != "" {
		_, err := r.getInstance(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, candidateName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return intctrlutil.NewFatalError(fmt.Sprintf(`candidate instance "%s" not found`, candidateName))
			}
			return err
		}
	}

	lfa, err := lifecycle.New(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name,
		synthesizedComp.LifecycleActions.ComponentLifecycleActions, synthesizedComp.TemplateVars, pod, pods)
	if err != nil {
		return err
	}

	// NOTE: switchover is a blocking action currently. May change to non-blocking for better performance.
	// Lifecycle preconditions still use the lifecycle reader contract as-is. If a multi-cluster
	// action needs data-plane runtime readiness checks, model that explicitly in the lifecycle API.
	return lfa.Switchover(ctx, cli, nil, candidateName)
}

func (r *switchoverRuntime) dataContext() context.Context {
	if !r.multiCluster {
		return r.ctx
	}
	return r.dataCtx
}

type switchoverInstance struct {
	pod *corev1.Pod
}

func (i *switchoverInstance) hasPod() bool {
	return i.pod != nil
}

func (i *switchoverInstance) getRole() string {
	if i.pod == nil {
		return ""
	}
	return i.pod.Labels[constant.RoleLabelKey]
}
