/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubeblocks

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type kbObjects map[schema.GroupVersionResource]*unstructured.UnstructuredList

var (
	resourceGVRs = []schema.GroupVersionResource{
		types.DeployGVR(),
		types.StatefulGVR(),
		types.ServiceGVR(),
		types.ConfigmapGVR(),
		types.PVCGVR(),
	}
)

func getKBObjects(dynamic dynamic.Interface, namespace string) (kbObjects, error) {
	var (
		err     error
		allErrs []error
	)

	appendErr := func(err error) {
		if err == nil || apierrors.IsNotFound(err) {
			return
		}
		allErrs = append(allErrs, err)
	}

	kbObjs := kbObjects{}
	ctx := context.TODO()

	// get CRDs
	crds, err := dynamic.Resource(types.CRDGVR()).List(ctx, metav1.ListOptions{})
	appendErr(err)
	kbObjs[types.CRDGVR()] = &unstructured.UnstructuredList{}
	for i, crd := range crds.Items {
		if !strings.Contains(crd.GetName(), "kubeblocks.io") {
			continue
		}
		crdObjs := kbObjs[types.CRDGVR()]
		crdObjs.Items = append(crdObjs.Items, crds.Items[i])

		// get built-in CRs belonging to this CRD
		gvr, err := getGVRByCRD(&crd)
		if err != nil {
			appendErr(err)
			continue
		}
		if crs, err := dynamic.Resource(*gvr).List(ctx, metav1.ListOptions{}); err != nil {
			appendErr(err)
			continue
		} else {
			kbObjs[*gvr] = crs
		}
	}

	// get objects by group version resource
	getObjects := func(labelSelector string, gvr schema.GroupVersionResource) {
		objs, err := dynamic.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			appendErr(err)
			return
		}

		if _, ok := kbObjs[gvr]; !ok {
			kbObjs[gvr] = &unstructured.UnstructuredList{}
		}
		target := kbObjs[gvr]
		target.Items = append(target.Items, objs.Items...)
	}

	// build label selector
	instanceLabelSelector := fmt.Sprintf("%s=%s", intctrlutil.AppInstanceLabelKey, types.KubeBlocksChartName)
	releaseLabelSelector := fmt.Sprintf("release=%s", types.KubeBlocksChartName)

	// get resources which label matches app.kubernetes.io/instance=kubeblocks or
	// label matches release=kubeblocks, like prometheus-server
	for _, labelSelector := range []string{instanceLabelSelector, releaseLabelSelector} {
		for _, gvr := range resourceGVRs {
			getObjects(labelSelector, gvr)
		}
	}

	// get volume snapshot class
	if _, ok := kbObjs[types.VolumeSnapshotClassGVR()]; !ok {
		kbObjs[types.VolumeSnapshotClassGVR()] = &unstructured.UnstructuredList{}
	}
	getObjects(instanceLabelSelector, types.VolumeSnapshotClassGVR())

	return kbObjs, utilerrors.NewAggregate(allErrs)
}

func removeCustomResources(dynamic dynamic.Interface, objs kbObjects) error {
	// get all CRDs
	crds, ok := objs[types.CRDGVR()]
	if !ok {
		return nil
	}

	// get CRs for every CRD
	for _, crd := range crds.Items {
		// get built-in CRs belonging to this CRD
		gvr, err := getGVRByCRD(&crd)
		if err != nil {
			return err
		}

		crs, ok := objs[*gvr]
		if !ok {
			continue
		}
		if err = deleteObjects(dynamic, *gvr, crs); err != nil {
			return err
		}
	}
	return nil
}

func deleteObjects(dynamic dynamic.Interface, gvr schema.GroupVersionResource, objects *unstructured.UnstructuredList) error {
	if objects == nil {
		return nil
	}

	for _, s := range objects.Items {
		// the object is not being deleted, delete it
		if s.GetDeletionTimestamp().IsZero() {
			klog.V(1).Infof("delete %s %s", gvr.String(), s.GetName())
			if err := dynamic.Resource(gvr).Namespace(s.GetNamespace()).Delete(context.TODO(), s.GetName(), newDeleteOpts()); err != nil {
				return err
			}
		}

		// if object has finalizers, remove it
		if len(s.GetFinalizers()) == 0 {
			continue
		}

		klog.V(1).Infof("remove finalizers of %s %s", gvr.String(), s.GetName())
		if _, err := dynamic.Resource(gvr).Namespace(s.GetNamespace()).Patch(context.TODO(), s.GetName(), k8sapitypes.JSONPatchType,
			[]byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func getRemainedResource(objs kbObjects) map[string][]string {
	res := map[string][]string{}
	appendItems := func(key string, l *unstructured.UnstructuredList) {
		for _, item := range l.Items {
			res[key] = append(res[key], item.GetName())
		}
	}

	for k, v := range objs {
		appendItems(k.Resource, v)
	}

	return res
}

func newDeleteOpts() metav1.DeleteOptions {
	gracePeriod := int64(0)
	return metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}
}
