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

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

type kbObjects struct {
	// custom resources
	crs map[schema.GroupVersionResource]*unstructured.UnstructuredList
	// custom resource definitions
	crds *unstructured.UnstructuredList
	// deployments
	deploys *appv1.DeploymentList
	// services
	svcs *corev1.ServiceList
	// configMaps
	cms *corev1.ConfigMapList
}

func getKBObjects(client kubernetes.Interface, dynamic dynamic.Interface, namespace string) (*kbObjects, error) {
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

	objs := &kbObjects{}
	ctx := context.TODO()

	// get CRDs
	crds, err := dynamic.Resource(types.CRDGVR()).List(ctx, metav1.ListOptions{})
	appendErr(err)
	objs.crds = &unstructured.UnstructuredList{}
	objs.crs = map[schema.GroupVersionResource]*unstructured.UnstructuredList{}
	for i, crd := range crds.Items {
		if !strings.Contains(crd.GetName(), "kubeblocks.io") {
			continue
		}
		objs.crds.Items = append(objs.crds.Items, crds.Items[i])

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
			objs.crs[*gvr] = crs
		}
	}

	// get deployments
	getDeploys := func(labelSelector string) {
		deploys, err := client.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			appendErr(err)
			return
		}
		if objs.deploys == nil {
			objs.deploys = deploys
		} else {
			objs.deploys.Items = append(objs.deploys.Items, deploys.Items...)
		}
	}

	// get all deployments which label matches app.kubernetes.io/instance=kubeblocks
	getDeploys(fmt.Sprintf("%s=%s", types.InstanceLabelKey, types.KubeBlocksChartName))

	// get all deployments which label matches release=kubeblocks, like prometheus-server
	getDeploys(fmt.Sprintf("release=%s", types.KubeBlocksChartName))

	// get services
	getSvcs := func(labelSelector string) {
		svcs, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			appendErr(err)
			return
		}
		if objs.svcs == nil {
			objs.svcs = svcs
		} else {
			objs.svcs.Items = append(objs.svcs.Items, svcs.Items...)
		}
	}

	// get all services which label matches app.kubernetes.io/instance=kubeblocks
	getSvcs(fmt.Sprintf("%s=%s", types.InstanceLabelKey, types.KubeBlocksChartName))

	// get all services which label matches release=kubeblocks, like prometheus-server
	getSvcs(fmt.Sprintf("release=%s", types.KubeBlocksChartName))

	// get configMap
	getConfigMap := func(labelSelector string) {
		cms, err := client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			appendErr(err)
			return
		}
		if objs.cms == nil {
			objs.cms = cms
		} else {
			objs.cms.Items = append(objs.cms.Items, cms.Items...)
		}
	}

	// get all configmaps that belong to KubeBlocks
	getConfigMap("configuration.kubeblocks.io/configuration-template=true")
	getConfigMap("configuration.kubeblocks.io/configuration-type=tpl")

	return objs, utilerrors.NewAggregate(allErrs)
}

func removeFinalizers(client dynamic.Interface, objs *kbObjects) error {
	removeFn := func(gvr schema.GroupVersionResource, crs *unstructured.UnstructuredList) error {
		if crs == nil {
			return nil
		}
		for _, cr := range crs.Items {
			if _, err := client.Resource(gvr).Patch(context.TODO(), cr.GetName(), k8sapitypes.JSONPatchType,
				[]byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{}); err != nil {
				return err
			}
		}
		return nil
	}

	for k, v := range objs.crs {
		if err := removeFn(k, v); err != nil {
			return err
		}
	}
	return nil
}

func deleteCRDs(cli dynamic.Interface, crds *unstructured.UnstructuredList) error {
	if crds == nil {
		return nil
	}

	for _, crd := range crds.Items {
		if strings.Contains(crd.GetName(), "kubeblocks.io") {
			if err := cli.Resource(types.CRDGVR()).Delete(context.TODO(), crd.GetName(), newDeleteOpts()); err != nil {
				return err
			}
		}
	}
	return nil
}

func deleteDeploys(client kubernetes.Interface, deploys *appv1.DeploymentList) error {
	if deploys == nil {
		return nil
	}

	for _, d := range deploys.Items {
		if err := client.AppsV1().Deployments(d.Namespace).Delete(context.TODO(), d.Name, newDeleteOpts()); err != nil {
			return err
		}
	}
	return nil
}

func deleteServices(client kubernetes.Interface, svcs *corev1.ServiceList) error {
	if svcs == nil {
		return nil
	}

	for _, s := range svcs.Items {
		if err := client.CoreV1().Services(s.Namespace).Delete(context.TODO(), s.Name, newDeleteOpts()); err != nil {
			return err
		}
	}
	return nil
}

func deleteConfigMaps(client kubernetes.Interface, cms *corev1.ConfigMapList) error {
	if cms == nil {
		return nil
	}

	for _, s := range cms.Items {
		// delete object
		if err := client.CoreV1().ConfigMaps(s.Namespace).Delete(context.TODO(), s.Name, newDeleteOpts()); err != nil {
			return err
		}

		// remove finalizers
		if _, err := client.CoreV1().ConfigMaps(s.Namespace).Patch(context.TODO(), s.Name, k8sapitypes.JSONPatchType,
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

	if objs.crds != nil {
		appendItems("CRDs", objs.crds)
	}

	for k, v := range objs.crs {
		appendItems(k.Resource, v)
	}

	if objs.svcs != nil {
		for _, item := range objs.svcs.Items {
			res["services"] = append(res["services"], item.GetName())
		}
	}

	if objs.deploys != nil {
		for _, item := range objs.deploys.Items {
			res["deployments"] = append(res["deployments"], item.GetName())
		}
	}
	return res
}

func newDeleteOpts() metav1.DeleteOptions {
	gracePeriod := int64(0)
	return metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}
}
