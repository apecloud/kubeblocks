/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"encoding/json"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// ClusterDeletionTransformer handles cluster deletion
type ClusterDeletionTransformer struct{}

func (t *ClusterDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.OrigCluster
	if !isClusterDeleting(*cluster) {
		return nil
	}
	root, err := findRootVertex(dag)
	if err != nil {
		return err
	}

	// list all kinds to be deleted based on v1alpha1.TerminationPolicyType
	var toDeleteKinds, toPreserveKinds []client.ObjectList
	switch cluster.Spec.TerminationPolicy {
	case appsv1alpha1.DoNotTerminate:
		transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeWarning, "DoNotTerminate", "spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
		return graph.ErrPrematureStop
	case appsv1alpha1.Halt:
		toDeleteKinds = kindsForHalt()
		toPreserveKinds = []client.ObjectList{
			&corev1.PersistentVolumeClaimList{},
			&corev1.SecretList{},
			&corev1.ConfigMapList{},
		}
	case appsv1alpha1.Delete:
		toDeleteKinds = kindsForDelete()
	case appsv1alpha1.WipeOut:
		toDeleteKinds = kindsForWipeOut()
	}

	transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, constant.ReasonDeletingCR, "Deleting %s: %s",
		strings.ToLower(cluster.GetObjectKind().GroupVersionKind().Kind), cluster.GetName())

	// list all objects owned by this cluster in cache, and delete them all
	// there is chance that objects leak occurs because of cache stale
	// ignore the problem currently
	// TODO: GC the leaked objects
	ml := getAppInstanceML(*cluster)

	preserveObjects := func() error {
		if len(toPreserveKinds) == 0 {
			return nil
		}

		objs, err := getClusterOwningObjects(transCtx, *cluster, ml, toPreserveKinds...)
		if err != nil {
			return err
		}
		// construct cluster spec JSON string
		clusterSpec := cluster.DeepCopy()
		clusterSpec.ObjectMeta = metav1.ObjectMeta{
			Name: cluster.GetName(),
			UID:  cluster.GetUID(),
		}
		clusterSpec.Status = appsv1alpha1.ClusterStatus{}
		b, err := json.Marshal(*clusterSpec)
		if err != nil {
			return err
		}
		clusterJSON := string(b)
		for _, o := range objs {
			origObj := o.DeepCopyObject().(client.Object)
			controllerutil.RemoveFinalizer(o, constant.DBClusterFinalizerName)
			ownerRefs := o.GetOwnerReferences()
			for i, ref := range ownerRefs {
				if ref.Kind != appsv1alpha1.ClusterKind ||
					!strings.Contains(ref.APIVersion, appsv1alpha1.GroupVersion.Group) {
					continue
				}
				ownerRefs = append(ownerRefs[:i], ownerRefs[i+1:]...)
				break
			}
			o.SetOwnerReferences(ownerRefs)
			annot := o.GetAnnotations()
			if annot == nil {
				annot = map[string]string{}
			}
			// annotated last-applied Cluster spec
			annot[constant.LastAppliedClusterAnnotationKey] = clusterJSON
			o.SetAnnotations(annot)
			vertex := &lifecycleVertex{obj: o, oriObj: origObj, action: actionPtr(UPDATE)}
			dag.AddVertex(vertex)
			dag.Connect(root, vertex)
		}
		return nil
	}
	// handle preserved objects update vertex
	if err := preserveObjects(); err != nil {
		return err
	}

	// add objects deletion vertex
	objs, err := getClusterOwningObjects(transCtx, *cluster, ml, toDeleteKinds...)
	if err != nil {
		return err
	}
	for _, o := range objs {
		vertex := &lifecycleVertex{obj: o, action: actionPtr(DELETE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
	}
	root.action = actionPtr(DELETE)

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}

func kindsForDoNotTerminate() []client.ObjectList {
	return []client.ObjectList{}
}

func kindsForHalt() []client.ObjectList {
	kinds := kindsForDoNotTerminate()
	kindsPlus := []client.ObjectList{
		&appsv1.StatefulSetList{},
		&appsv1.DeploymentList{},
		&corev1.ServiceList{},
		&policyv1.PodDisruptionBudgetList{},
	}
	return append(kinds, kindsPlus...)
}

func kindsForDelete() []client.ObjectList {
	kinds := kindsForHalt()
	kindsPlus := []client.ObjectList{
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
		&corev1.PersistentVolumeClaimList{},
		&dataprotectionv1alpha1.BackupPolicyList{},
		&batchv1.JobList{},
	}
	return append(kinds, kindsPlus...)
}

func kindsForWipeOut() []client.ObjectList {
	kinds := kindsForDelete()
	kindsPlus := []client.ObjectList{
		&dataprotectionv1alpha1.BackupList{},
	}
	return append(kinds, kindsPlus...)
}

var _ graph.Transformer = &ClusterDeletionTransformer{}
