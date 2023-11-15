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

package apps

import (
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func newRequeueError(after time.Duration, reason string) error {
	return intctrlutil.NewRequeueError(after, reason)
}

func getGVKName(object client.Object, scheme *runtime.Scheme) (*gvkNObjKey, error) {
	gvk, err := apiutil.GVKForObject(object, scheme)
	if err != nil {
		return nil, err
	}
	return &gvkNObjKey{
		GroupVersionKind: gvk,
		ObjectKey: client.ObjectKey{
			Namespace: object.GetNamespace(),
			Name:      object.GetName(),
		},
	}, nil
}

func getAppInstanceML(cluster appsv1alpha1.Cluster) client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.Name,
	}
}

// getClusterOwningNamespacedObjects reads namespaced objects owned by our cluster with kinds.
func getClusterOwningNamespacedObjects(transCtx *clusterTransformContext,
	cluster appsv1alpha1.Cluster,
	labels client.MatchingLabels,
	kinds []client.ObjectList) (clusterOwningObjects, error) {
	inNS := client.InNamespace(cluster.Namespace)
	return getClusterOwningObjectsWithOptions(transCtx, kinds, inNS, labels)
}

// getClusterOwningNonNamespacedObjects reads non-namespaced objects owned by our cluster with kinds.
func getClusterOwningNonNamespacedObjects(transCtx *clusterTransformContext,
	_ appsv1alpha1.Cluster,
	labels client.MatchingLabels,
	kinds []client.ObjectList) (clusterOwningObjects, error) {
	return getClusterOwningObjectsWithOptions(transCtx, kinds, labels)
}

// getClusterOwningObjectsWithOptions reads objects owned by our cluster with kinds and specified options.
func getClusterOwningObjectsWithOptions(transCtx *clusterTransformContext,
	kinds []client.ObjectList,
	opts ...client.ListOption) (clusterOwningObjects, error) {
	// list what kinds of object cluster owns
	objs := make(clusterOwningObjects)
	for _, list := range kinds {
		if err := transCtx.Client.List(transCtx.Context, list, opts...); err != nil {
			return nil, err
		}
		// reflect get list.Items
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		l := items.Len()
		for i := 0; i < l; i++ {
			// get the underlying object
			object := items.Index(i).Addr().Interface().(client.Object)
			name, err := getGVKName(object, rscheme)
			if err != nil {
				return nil, err
			}
			objs[*name] = object
		}
	}
	return objs, nil
}

// sendWarningEventWithError sends a warning event when occurs error.
func sendWarningEventWithError(
	recorder record.EventRecorder,
	cluster *appsv1alpha1.Cluster,
	reason string,
	err error) {
	// ignore requeue error
	if err == nil || intctrlutil.IsRequeueError(err) {
		return
	}
	controllerErr := intctrlutil.UnwrapControllerError(err)
	if controllerErr != nil {
		reason = string(controllerErr.Type)
	}
	recorder.Event(cluster, corev1.EventTypeWarning, reason, err.Error())
}

func isResourceRequirementsEqual(a, b corev1.ResourceRequirements) bool {
	return isResourceEqual(a.Requests, b.Requests) && isResourceEqual(a.Limits, b.Limits)
}

func isResourceEqual(a, b corev1.ResourceList) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !v.Equal(b[k]) {
			return false
		}
	}
	return true
}

func isVolumeClaimTemplatesEqual(a, b []appsv1alpha1.ClusterComponentVolumeClaimTemplate) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		// first check resource requirements
		c := a[i].DeepCopy()
		d := b[i].DeepCopy()
		if !isResourceRequirementsEqual(c.Spec.Resources, d.Spec.Resources) {
			return false
		}

		// then clear resource requirements and check other fields
		c.Spec.Resources = corev1.ResourceRequirements{}
		d.Spec.Resources = corev1.ResourceRequirements{}
		if !reflect.DeepEqual(c, d) {
			return false
		}
	}
	return true
}
