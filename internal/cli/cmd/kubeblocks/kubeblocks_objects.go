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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type kbObjects map[schema.GroupVersionResource]*unstructured.UnstructuredList

var (
	resourceGVRs = []schema.GroupVersionResource{
		types.DeployGVR(),
		types.StatefulSetGVR(),
		types.ServiceGVR(),
		types.PVCGVR(),
		types.ConfigmapGVR(),
		types.VolumeSnapshotClassGVR(),
	}
)

// getKBObjects returns all KubeBlocks objects include addons objects
func getKBObjects(dynamic dynamic.Interface, namespace string, addons []*extensionsv1alpha1.Addon) (kbObjects, error) {
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
		if !strings.Contains(crd.GetName(), constant.APIGroup) {
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

	getWebhooks := func(gvr schema.GroupVersionResource) {
		objs, err := dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			appendErr(err)
			return
		}
		result := &unstructured.UnstructuredList{}
		for _, obj := range objs.Items {
			if !strings.Contains(obj.GetName(), strings.ToLower(string(util.KubeBlocksApp))) {
				continue
			}
			result.Items = append(result.Items, obj)
		}
		kbObjs[gvr] = result
	}
	getWebhooks(types.ValidatingWebhookConfigurationGVR())
	getWebhooks(types.MutatingWebhookConfigurationGVR())

	// get objects by label selector
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

	// get object by name
	getObject := func(name string, gvr schema.GroupVersionResource) {
		obj, err := dynamic.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			appendErr(err)
			return
		}
		if _, ok := kbObjs[gvr]; !ok {
			kbObjs[gvr] = &unstructured.UnstructuredList{}
		}
		target := kbObjs[gvr]
		target.Items = append(target.Items, *obj)
	}

	// get resources which label matches app.kubernetes.io/instance=kubeblocks or
	// label matches release=kubeblocks, like prometheus-server
	for _, selector := range buildResourceLabelSelectors(addons) {
		for _, gvr := range resourceGVRs {
			getObjects(selector, gvr)
		}
	}

	// build label selector
	configMapLabelSelector := fmt.Sprintf("%s=%s", constant.CMConfigurationTypeLabelKey, constant.ConfigTemplateType)

	// get configmap
	getObjects(configMapLabelSelector, types.ConfigmapGVR())

	// get PVs by PVC
	if pvcs, ok := kbObjs[types.PVCGVR()]; ok {
		for _, obj := range pvcs.Items {
			pvc := &corev1.PersistentVolumeClaim{}
			if err = apiruntime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, pvc); err != nil {
				appendErr(err)
				continue
			}
			getObject(pvc.Spec.VolumeName, types.PVGVR())
		}
	}

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
	const (
		helmResourcePolicyKey  = "helm.sh/resource-policy"
		helmResourcePolicyKeep = "keep"
	)

	if objects == nil {
		return nil
	}

	// if resource has annotation "helm.sh/resource-policy": "keep", skip it
	// TODO: maybe a flag to control this behavior
	keepResource := func(obj unstructured.Unstructured) bool {
		annotations := obj.GetAnnotations()
		if len(annotations) == 0 {
			return false
		}
		if annotations[helmResourcePolicyKey] == helmResourcePolicyKeep {
			return true
		}
		return false
	}

	for _, s := range objects.Items {
		if keepResource(s) {
			continue
		}

		// the object is not being deleted, delete it
		if s.GetDeletionTimestamp().IsZero() {
			klog.V(1).Infof("delete %s %s", gvr.String(), s.GetName())
			if err := dynamic.Resource(gvr).Namespace(s.GetNamespace()).Delete(context.TODO(), s.GetName(), newDeleteOpts()); err != nil && !apierrors.IsNotFound(err) {
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
		// ignore PVC and PV
		if k == types.PVCGVR() || k == types.PVGVR() {
			continue
		}
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

func deleteNamespace(client kubernetes.Interface, namespace string) error {
	return client.CoreV1().Namespaces().Delete(context.TODO(), namespace, newDeleteOpts())
}
