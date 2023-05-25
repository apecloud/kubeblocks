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

package kubeblocks

import (
	"context"
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
	"github.com/apecloud/kubeblocks/internal/constant"
)

type resourceScope string

const (
	ResourceScopeGlobal resourceScope = "global"
	ResourceScopeLocal  resourceScope = "namespaced"
)

type kbObjects map[schema.GroupVersionResource]*unstructured.UnstructuredList

var (
	// addon resources
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

	getObjectsByLabels := func(labelSelector string, gvr schema.GroupVersionResource, scope resourceScope) {
		ns := namespace
		if scope == ResourceScopeGlobal {
			ns = metav1.NamespaceAll
		}

		klog.V(1).Infof("search objects by labels, namespace: %s, name: %s, gvr: %s", labelSelector, gvr, scope)
		objs, err := dynamic.Resource(gvr).Namespace(ns).List(context.TODO(), metav1.ListOptions{
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
		if klog.V(1).Enabled() {
			for _, item := range objs.Items {
				klog.Infof("\tget object: %s, %s, %s", item.GetNamespace(), item.GetKind(), item.GetName())
			}
		}
	}

	// get object by name
	getObjectByName := func(name string, gvr schema.GroupVersionResource) {
		klog.V(1).Infof("search object by name, namespace: %s, name: %s, gvr: %s ", namespace, name, gvr)
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
		klog.V(1).Infof("\tget object: %s, %s, %s", obj.GetNamespace(), obj.GetKind(), obj.GetName())
	}
	// get RBAC resources, such as ClusterRole, ClusterRoleBinding, Role, RoleBinding, ServiceAccount
	getObjectsByLabels(buildKubeBlocksSelectorLabels(), types.ClusterRoleGVR(), ResourceScopeGlobal)
	getObjectsByLabels(buildKubeBlocksSelectorLabels(), types.ClusterRoleBindingGVR(), ResourceScopeGlobal)
	getObjectsByLabels(buildKubeBlocksSelectorLabels(), types.RoleGVR(), ResourceScopeLocal)
	getObjectsByLabels(buildKubeBlocksSelectorLabels(), types.RoleBindingGVR(), ResourceScopeLocal)
	getObjectsByLabels(buildKubeBlocksSelectorLabels(), types.ServiceAccountGVR(), ResourceScopeLocal)
	// get webhooks
	getObjectsByLabels(buildKubeBlocksSelectorLabels(), types.ValidatingWebhookConfigurationGVR(), ResourceScopeGlobal)
	getObjectsByLabels(buildKubeBlocksSelectorLabels(), types.MutatingWebhookConfigurationGVR(), ResourceScopeGlobal)
	// get configmap for config template
	getObjectsByLabels(buildConfigTypeSelectorLabels(), types.ConfigmapGVR(), ResourceScopeLocal)
	getObjectsByLabels(buildKubeBlocksSelectorLabels(), types.ConfigmapGVR(), ResourceScopeLocal)

	// get resources which label matches app.kubernetes.io/instance=kubeblocks or
	// label matches release=kubeblocks, like prometheus-server
	for _, selector := range buildResourceLabelSelectors(addons) {
		for _, gvr := range resourceGVRs {
			getObjectsByLabels(selector, gvr, ResourceScopeLocal)
		}
	}

	// get PVs by PVC
	if pvcs, ok := kbObjs[types.PVCGVR()]; ok {
		for _, obj := range pvcs.Items {
			pvc := &corev1.PersistentVolumeClaim{}
			if err = apiruntime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, pvc); err != nil {
				appendErr(err)
				continue
			}
			getObjectByName(pvc.Spec.VolumeName, types.PVGVR())
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
