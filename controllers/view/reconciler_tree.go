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
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dataprotection"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	vsv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	rbacv1 "k8s.io/api/rbac/v1"

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
	objectType(appsv1alpha1.SchemeGroupVersion.String(), appsv1alpha1.ClusterKind):     newClusterReconciler,
	objectType(appsv1alpha1.SchemeGroupVersion.String(), appsv1alpha1.ComponentKind):   newComponentReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.SecretKind):                newSecretReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.ServiceKind):               newServiceReconciler,
	objectType(workloadsAPI.SchemeGroupVersion.String(), workloadsAPI.Kind):            newInstanceSetReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.ConfigMapKind):             newConfigMapReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.PersistentVolumeClaimKind): newPVCReconciler,
	objectType(rbacv1.SchemeGroupVersion.String(), constant.ClusterRoleBindingKind):    newClusterRoleBindingReconciler,
	objectType(rbacv1.SchemeGroupVersion.String(), constant.RoleBindingKind):           newRoleBindingReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.ServiceAccountKind):        newSAReconciler,
	objectType(batchv1.SchemeGroupVersion.String(), constant.JobKind):                  newJobReconciler,
	objectType(dpv1alpha1.SchemeGroupVersion.String(), types.BackupKind):               newBackupReconciler,
	objectType(dpv1alpha1.SchemeGroupVersion.String(), types.RestoreKind):              newRestoreReconciler,
	objectType(appsv1alpha1.SchemeGroupVersion.String(), constant.ConfigurationKind):   newConfigurationReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.PodKind):                   newPodReconciler,
	objectType(appsv1.SchemeGroupVersion.String(), constant.StatefulSetKind):           newSTSReconciler,
	objectType(vsv1.SchemeGroupVersion.String(), constant.VolumeSnapshotKind):          newVolumeSnapshotV1Reconciler,
	objectType(vsv1beta1.SchemeGroupVersion.String(), constant.VolumeSnapshotKind):     newVolumeSnapshotV1Beta1Reconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.PersistentVolumeKind):      newPVReconciler,
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
		objects, err := getObjectsByGVK(r.ctx, r.cli, gvk, nil)
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

type doNothingReconciler struct {
	baseReconciler
}

func (r *doNothingReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func newPVReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	// TODO(free6om): finish me
	panic("implement me")
}

func newVolumeSnapshotV1Beta1Reconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	// TODO(free6om): finish me
	panic("implement me")
}

func newVolumeSnapshotV1Reconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	// TODO(free6om): finish me
	panic("implement me")
}

func newSTSReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	// TODO(free6om): set sts to ready
	panic("implement me")
}

func newRestoreReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &dataprotection.RestoreReconciler{
		Client:   c,
		Scheme:   c.Scheme(),
		Recorder: recorder,
	}
}

func newBackupReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	config := intctrlutil.GeKubeRestConfig("kubeblocks-view")
	return &dataprotection.BackupReconciler{
		Client:     c,
		Scheme:     c.Scheme(),
		Recorder:   recorder,
		RestConfig: config,
	}
}

func newJobReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	// TODO(free6om): set job to succeed
	panic("implement me")
}

func newSAReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

func newRoleBindingReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

func newClusterRoleBindingReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

func newPVCReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	// TODO(free6om): set pvc to bound
	panic("implement me")
}

func newConfigMapReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

func newSecretReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

func newDoNothingReconciler() reconcile.Reconciler {
	return &doNothingReconciler{}
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
var _ reconcile.Reconciler = &doNothingReconciler{}
var _ reconcile.Reconciler = &podReconciler{}
var _ reconcile.Reconciler = &serviceReconciler{}
