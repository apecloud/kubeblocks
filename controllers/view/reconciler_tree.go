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

package view

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	workloadsAPI "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps"
	"github.com/apecloud/kubeblocks/controllers/apps/configuration"
	"github.com/apecloud/kubeblocks/controllers/workloads"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

type ReconcilerTree interface {
	Run() error
}

type reconcilerFunc func(client.Client, record.EventRecorder) reconcile.Reconciler

var reconcilerFuncMap = map[viewv1.ObjectType]reconcilerFunc{
	objectType(appsv1alpha1.APIVersion, appsv1alpha1.ClusterKind):        newClusterReconciler,
	objectType(appsv1alpha1.APIVersion, appsv1alpha1.ComponentKind):      newComponentReconciler,
	objectType(appsv1alpha1.APIVersion, "Configuration"):                 newConfigurationReconciler,
	objectType(workloadsAPI.GroupVersion.String(), workloadsAPI.Kind):    newInstanceSetReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.PodKind):     newPodReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.ServiceKind): newServiceReconciler,
}

type reconcilerTree struct {
	ctx         context.Context
	cli         client.Client
	tree        *graph.DAG
	reconcilers map[viewv1.ObjectType]reconcile.Reconciler
}

func (r *reconcilerTree) Run() error {
	return r.tree.WalkTopoOrder(func(v graph.Vertex) error {
		objType, _ := v.(viewv1.ObjectType)
		reconciler, _ := r.reconcilers[objType]
		gvk, err := objectTypeToGVK(&objType)
		if err != nil {
			return err
		}
		objects, err := getObjectsByGVK(r.ctx, r.cli, gvk)
		if err != nil {
			return err
		}
		for _, object := range objects {
			// TODO(free6om): verify whether safe to ignore reconciliation result
			_, err = reconciler.Reconcile(r.ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(object)})
			if err != nil {
				return err
			}
		}
		return nil
	}, func(v1, v2 graph.Vertex) bool {
		t1, _ := v1.(viewv1.ObjectType)
		t2, _ := v2.(viewv1.ObjectType)
		if t1.APIVersion != t2.APIVersion {
			return t1.APIVersion < t2.APIVersion
		}
		return t1.Kind < t2.Kind
	})
}

func newReconcilerTree(ctx context.Context, mClient client.Client, recorder record.EventRecorder, rules []OwnershipRule) (ReconcilerTree, error) {
	dag := graph.NewDAG()
	reconcilers := make(map[viewv1.ObjectType]reconcile.Reconciler)
	for _, rule := range rules {
		dag.AddVertex(rule.Primary)
		reconciler, err := newReconciler(mClient, recorder, rule.Primary)
		if err != nil {
			return nil, err
		}
		reconcilers[rule.Primary] = reconciler
		for _, resource := range rule.OwnedResources {
			dag.AddVertex(resource.Secondary)
			dag.Connect(rule.Primary, resource.Secondary)
			reconciler, err = newReconciler(mClient, recorder, resource.Secondary)
			if err != nil {
				return nil, err
			}
			reconcilers[resource.Secondary] = reconciler
		}
	}
	// DAG should be valid(one and only one root without cycle)
	if err := dag.Validate(); err != nil {
		return nil, err
	}

	return &reconcilerTree{
		ctx:         ctx,
		cli:         mClient,
		tree:        dag,
		reconcilers: reconcilers,
	}, nil
}

func newReconciler(mClient client.Client, recorder record.EventRecorder, objectType viewv1.ObjectType) (reconcile.Reconciler, error) {
	reconcilerF, ok := reconcilerFuncMap[objectType]
	if ok {
		return reconcilerF(mClient, recorder), nil
	}
	return nil, fmt.Errorf("can't initialize a reconciler for GVK: %s/%s", objectType.APIVersion, objectType.Kind)
}

func objectType(apiVersion, kind string) viewv1.ObjectType {
	return viewv1.ObjectType{
		APIVersion: apiVersion,
		Kind:       kind,
	}
}

func newClusterReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &apps.ClusterReconciler{
		Client:   cli,
		Scheme:   cli.Scheme(),
		Recorder: recorder,
	}
}

func newComponentReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &apps.ComponentReconciler{
		Client:   cli,
		Scheme:   cli.Scheme(),
		Recorder: recorder,
	}
}

func newConfigurationReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &configuration.ConfigurationReconciler{
		Client:   cli,
		Scheme:   cli.Scheme(),
		Recorder: recorder,
	}
}

func newInstanceSetReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &workloads.InstanceSetReconciler{
		Client:   cli,
		Scheme:   cli.Scheme(),
		Recorder: recorder,
	}
}

type baseReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type podReconciler struct {
	baseReconciler
}

func (p *podReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	//TODO implement me
	panic("implement me")
}

func newPodReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &podReconciler{
		baseReconciler: baseReconciler{
			Client:   cli,
			Scheme:   cli.Scheme(),
			Recorder: recorder,
		},
	}
}

type serviceReconciler struct {
	baseReconciler
}

func (p *serviceReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	//TODO implement me
	panic("implement me")
}

func newServiceReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &serviceReconciler{
		baseReconciler: baseReconciler{
			Client:   cli,
			Scheme:   cli.Scheme(),
			Recorder: recorder,
		},
	}
}

var _ ReconcilerTree = &reconcilerTree{}
var _ reconcile.Reconciler = &podReconciler{}
var _ reconcile.Reconciler = &serviceReconciler{}
