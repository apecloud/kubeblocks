/*
Copyright ApeCloud Inc.

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
	clusterDefs     *unstructured.UnstructuredList
	clusterVersions *unstructured.UnstructuredList
	crds            *unstructured.UnstructuredList
	deploys         *appv1.DeploymentList
	svcs            *corev1.ServiceList
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

	// get ClusterDefinition
	if objs.clusterDefs, err = dynamic.Resource(types.ClusterDefGVR()).List(ctx, metav1.ListOptions{}); err != nil {
		appendErr(err)
	}

	// get ClusterVersion
	if objs.clusterVersions, err = dynamic.Resource(types.ClusterVersionGVR()).List(ctx, metav1.ListOptions{}); err != nil {
		appendErr(err)
	}

	// get CRDs
	if objs.crds, err = dynamic.Resource(types.CRDGVR()).List(ctx, metav1.ListOptions{}); err != nil {
		appendErr(err)
	}

	// get deployments
	getDeploysFn := func(labelSelector string) error {
		deploys, err := client.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return err
		}
		if objs.deploys == nil {
			objs.deploys = deploys
		} else {
			objs.deploys.Items = append(objs.deploys.Items, deploys.Items...)
		}
		return nil
	}

	// get all deployments which label matches app.kubernetes.io/instance=kubeblocks
	if err = getDeploysFn(fmt.Sprintf("%s=%s", types.InstanceLabelKey, types.KubeBlocksChartName)); err != nil {
		appendErr(err)
	}

	// get all deployments which label matches release=kubeblocks, like prometheus-server
	if err = getDeploysFn(fmt.Sprintf("release=%s", types.KubeBlocksChartName)); err != nil {
		appendErr(err)
	}

	// get services
	getSvcsFn := func(labelSelector string) error {
		svcs, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return err
		}
		if objs.svcs == nil {
			objs.svcs = svcs
		} else {
			objs.svcs.Items = append(objs.svcs.Items, svcs.Items...)
		}
		return nil
	}

	// get all services which label matches app.kubernetes.io/instance=kubeblocks
	if err = getSvcsFn(fmt.Sprintf("%s=%s", types.InstanceLabelKey, types.KubeBlocksChartName)); err != nil {
		appendErr(err)
	}

	// get all services which label matches release=kubeblocks, like prometheus-server
	if err = getSvcsFn(fmt.Sprintf("release=%s", types.KubeBlocksChartName)); err != nil {
		appendErr(err)
	}

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

	// patch ClusterDefinition's finalizer
	if err := removeFn(types.ClusterDefGVR(), objs.clusterDefs); err != nil {
		return err
	}

	// patch ClusterVersion's finalizer
	return removeFn(types.ClusterVersionGVR(), objs.clusterVersions)
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

func checkIfRemainedResource(objs *kbObjects) bool {
	checkUnstructuredList := func(l *unstructured.UnstructuredList) bool {
		if l == nil || len(l.Items) == 0 {
			return false
		}
		return true
	}

	if checkUnstructuredList(objs.crds) ||
		checkUnstructuredList(objs.clusterDefs) ||
		checkUnstructuredList(objs.clusterVersions) {
		return true
	}

	if objs.svcs != nil && len(objs.svcs.Items) > 0 {
		return true
	}

	if objs.deploys != nil && len(objs.svcs.Items) > 0 {
		return true
	}
	return false
}

func newDeleteOpts() metav1.DeleteOptions {
	gracePeriod := int64(0)
	return metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}
}
