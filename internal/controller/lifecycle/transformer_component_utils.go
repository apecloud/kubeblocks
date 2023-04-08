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

package lifecycle

import (
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	"reflect"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

func listObjWithLabelsInNamespace[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](
	reqCtx intctrlutil.RequestCtx, cli client.Client, _ func(T, L), namespace string, labels client.MatchingLabels) ([]PT, error) {
	var objList L
	if err := cli.List(reqCtx.Ctx, PL(&objList), labels, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	objs := make([]PT, 0)
	items := reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T)
	for i := range items {
		objs = append(objs, &items[i])
	}
	return objs, nil
}

func listStsOwnedByComponent(reqCtx intctrlutil.RequestCtx, cli client.Client,
	namespace string, labels client.MatchingLabels) ([]*appsv1.StatefulSet, error) {
	return listObjWithLabelsInNamespace(reqCtx, cli, generics.StatefulSetSignature, namespace, labels)
}

func listDeployOwnedByComponent(reqCtx intctrlutil.RequestCtx, cli client.Client,
	namespace string, labels client.MatchingLabels) ([]*appsv1.Deployment, error) {
	return listObjWithLabelsInNamespace(reqCtx, cli, generics.DeploymentSignature, namespace, labels)
}

// restartPod restarts a Pod through updating the pod's annotation
func restartPod(podTemplate *corev1.PodTemplateSpec) error {
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = map[string]string{}
	}

	// startTimestamp := opsRes.OpsRequest.Status.StartTimestamp
	startTimestamp := time.Now() // TODO(refactor): impl
	restartTimestamp := podTemplate.Annotations[constant.RestartAnnotationKey]
	// if res, _ := time.Parse(time.RFC3339, restartTimestamp); startTimestamp.After(res) {
	if res, _ := time.Parse(time.RFC3339, restartTimestamp); startTimestamp.Before(res) {
		podTemplate.Annotations[constant.RestartAnnotationKey] = startTimestamp.Format(time.RFC3339)
	}
	return nil
}

// mergeAnnotations keeps the original annotations.
// if annotations exist and are replaced, the Deployment/StatefulSet will be updated.
func mergeAnnotations(originalAnnotations map[string]string, targetAnnotations *map[string]string) {
	if targetAnnotations == nil {
		return
	}
	if *targetAnnotations == nil {
		*targetAnnotations = map[string]string{}
	}
	for k, v := range originalAnnotations {
		// if the annotation not exist in targetAnnotations, copy it from original.
		if _, ok := (*targetAnnotations)[k]; !ok {
			(*targetAnnotations)[k] = v
		}
	}
}

// mergeServiceAnnotations keeps the original annotations except prometheus scrape annotations.
// if annotations exist and are replaced, the Service will be updated.
func mergeServiceAnnotations(originalAnnotations, targetAnnotations map[string]string) map[string]string {
	if len(originalAnnotations) == 0 {
		return targetAnnotations
	}
	tmpAnnotations := make(map[string]string, len(originalAnnotations)+len(targetAnnotations))
	for k, v := range originalAnnotations {
		if !strings.HasPrefix(k, "prometheus.io") {
			tmpAnnotations[k] = v
		}
	}
	maps.Copy(tmpAnnotations, targetAnnotations)
	return tmpAnnotations
}

func ownedKinds() []client.ObjectList {
	return []client.ObjectList{
		&appsv1.StatefulSetList{},
		&appsv1.DeploymentList{},
		&corev1.ServiceList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
		&corev1.PersistentVolumeClaimList{},
		&policyv1.PodDisruptionBudgetList{},
	}
}

// read all objects owned by component
func readCacheSnapshot(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster) (clusterSnapshot, error) {
	// list what kinds of object cluster owns
	kinds := ownedKinds()
	snapshot := make(clusterSnapshot)
	ml := client.MatchingLabels{constant.AppInstanceLabelKey: cluster.GetName()}
	inNS := client.InNamespace(cluster.Namespace)
	for _, list := range kinds {
		if err := cli.List(reqCtx.Ctx, list, inNS, ml); err != nil {
			return nil, err
		}
		// reflect get list.Items
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		l := items.Len()
		for i := 0; i < l; i++ {
			// get the underlying object
			object := items.Index(i).Addr().Interface().(client.Object)
			// put to snapshot if owned by our cluster
			if isOwnerOf(cluster, object, scheme) {
				name, err := getGVKName(object, scheme)
				if err != nil {
					return nil, err
				}
				snapshot[*name] = object
			}
		}
	}
	return snapshot, nil
}

func resolveObjectAction(snapshot clusterSnapshot, vertex *ictrltypes.LifecycleVertex) (*ictrltypes.LifecycleAction, error) {
	gvk, err := getGVKName(vertex.Obj, scheme)
	if err != nil {
		return nil, err
	}
	if obj, ok := snapshot[*gvk]; ok {
		vertex.ObjCopy = obj
		return ictrltypes.ActionUpdatePtr(), nil
	} else {
		return ictrltypes.ActionCreatePtr(), nil
	}
}
