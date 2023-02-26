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
	"k8s.io/apimachinery/pkg/api/meta"
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

type kbObjects struct {
	// custom resources
	crs map[schema.GroupVersionResource]unstructured.UnstructuredList
	// custom resource definitions
	crds unstructured.UnstructuredList
	// deployments
	deploys unstructured.UnstructuredList
	// statefulsets
	stss unstructured.UnstructuredList
	// services
	svcs unstructured.UnstructuredList
	// configMaps
	cms unstructured.UnstructuredList
	// PVCs
	pvcs unstructured.UnstructuredList
}

func getKBObjects(dynamic dynamic.Interface, namespace string) (*kbObjects, error) {
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

	kbObjs := &kbObjects{}
	ctx := context.TODO()

	// get CRDs
	crds, err := dynamic.Resource(types.CRDGVR()).List(ctx, metav1.ListOptions{})
	appendErr(err)
	kbObjs.crs = map[schema.GroupVersionResource]unstructured.UnstructuredList{}
	for i, crd := range crds.Items {
		if !strings.Contains(crd.GetName(), "kubeblocks.io") {
			continue
		}
		kbObjs.crds.Items = append(kbObjs.crds.Items, crds.Items[i])

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
			kbObjs.crs[*gvr] = *crs
		}
	}

	// get deployments
	getObjects := func(labelSelector string, gvr schema.GroupVersionResource, target *unstructured.UnstructuredList) {
		objs, err := dynamic.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			appendErr(err)
			return
		}
		target.Items = append(target.Items, objs.Items...)
	}

	// build label selector
	instanceLabelSelector := fmt.Sprintf("%s=%s", intctrlutil.AppInstanceLabelKey, types.KubeBlocksChartName)
	releaseLabelSelector := fmt.Sprintf("release=%s", types.KubeBlocksChartName)

	// get resources which label matches app.kubernetes.io/instance=kubeblocks or
	// label matches release=kubeblocks, like prometheus-server
	for _, labelSelector := range []string{instanceLabelSelector, releaseLabelSelector} {
		getObjects(labelSelector, types.DeployGVR(), &kbObjs.deploys)
		getObjects(labelSelector, types.StatefulGVR(), &kbObjs.stss)
		getObjects(labelSelector, types.ServiceGVR(), &kbObjs.svcs)
		getObjects(labelSelector, types.ConfigmapGVR(), &kbObjs.cms)
		getObjects(labelSelector, types.PVCGVR(), &kbObjs.pvcs)
	}

	// get volume snapshot class
	if _, ok := kbObjs.crs[types.VolumeSnapshotClassGVR()]; !ok {
		kbObjs.crs[types.VolumeSnapshotClassGVR()] = unstructured.UnstructuredList{}
	}
	vscs := kbObjs.crs[types.VolumeSnapshotClassGVR()]
	getObjects(instanceLabelSelector, types.VolumeSnapshotClassGVR(), &vscs)

	return kbObjs, utilerrors.NewAggregate(allErrs)
}

func removeFinalizers(client dynamic.Interface, objs *kbObjects) error {
	removeFn := func(gvr schema.GroupVersionResource, crs *unstructured.UnstructuredList) error {
		if crs == nil {
			return nil
		}
		for _, cr := range crs.Items {
			if gvr == types.VolumeSnapshotClassGVR() {
				if err := client.Resource(gvr).Delete(context.TODO(), cr.GetName(), newDeleteOpts()); err != nil {
					return err
				}
			}
			if _, err := client.Resource(gvr).Patch(context.TODO(), cr.GetName(), k8sapitypes.JSONPatchType,
				[]byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{}); err != nil {
				return err
			}
		}
		return nil
	}

	for k, v := range objs.crs {
		if err := removeFn(k, &v); err != nil {
			return err
		}
	}
	return nil
}

func deleteObjects(dynamic dynamic.Interface, mapper meta.RESTMapper, objects *unstructured.UnstructuredList) error {
	if objects == nil {
		return nil
	}

	var (
		err error
		gvr schema.GroupVersionResource
	)
	for _, s := range objects.Items {
		if gvr.Empty() {
			gvr, err = getUnstructuredGVR(&s, mapper)
			if err != nil {
				return err
			}
		}

		// delete object
		klog.V(1).Infof("delete %s %s", gvr.String(), s.GetName())
		if err = dynamic.Resource(gvr).Namespace(s.GetNamespace()).Delete(context.TODO(), s.GetName(), newDeleteOpts()); err != nil {
			return err
		}

		// remove finalizers
		if _, err = dynamic.Resource(gvr).Namespace(s.GetNamespace()).Patch(context.TODO(), s.GetName(), k8sapitypes.JSONPatchType,
			[]byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func getRemainedResource(objs *kbObjects) map[string][]string {
	res := map[string][]string{}
	appendItems := func(key string, l *unstructured.UnstructuredList) {
		for _, item := range l.Items {
			res[key] = append(res[key], item.GetName())
		}
	}

	appendItems("CRDs", &objs.crds)

	// custom resources
	for k, v := range objs.crs {
		appendItems(k.Resource, &v)
	}

	// services
	for _, item := range objs.svcs.Items {
		res["services"] = append(res["services"], item.GetName())
	}

	// deployments
	for _, item := range objs.deploys.Items {
		res["deployments"] = append(res["deployments"], item.GetName())
	}

	return res
}

func newDeleteOpts() metav1.DeleteOptions {
	gracePeriod := int64(0)
	return metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}
}

func getUnstructuredGVR(objs *unstructured.Unstructured, mapper meta.RESTMapper) (schema.GroupVersionResource, error) {
	gvk := objs.GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}
