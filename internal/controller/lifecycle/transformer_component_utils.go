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
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
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
	startTimestamp := time.Now() // TODO: impl
	restartTimestamp := podTemplate.Annotations[constant.RestartAnnotationKey]
	// if res, _ := time.Parse(time.RFC3339, restartTimestamp); startTimestamp.After(res) {
	if res, _ := time.Parse(time.RFC3339, restartTimestamp); startTimestamp.Before(res) {
		podTemplate.Annotations[constant.RestartAnnotationKey] = startTimestamp.Format(time.RFC3339)
	}
	return nil
}

// mergeAnnotations keeps the original annotations.
// if annotations exist and are replaced, the Deployment/StatefulSet will be updated.
func mergeAnnotations(originalAnnotations, targetAnnotations map[string]string) map[string]string {
	if restartAnnotation, ok := originalAnnotations[constant.RestartAnnotationKey]; ok {
		if targetAnnotations == nil {
			targetAnnotations = map[string]string{}
		}
		targetAnnotations[constant.RestartAnnotationKey] = restartAnnotation
	}
	return targetAnnotations
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

// updateComponentPhaseWithOperation if workload of component changes, should update the component phase.
func updateComponentPhaseWithOperation(cluster *appsv1alpha1.Cluster, componentName string) {
	componentPhase := appsv1alpha1.SpecReconcilingClusterCompPhase
	if cluster.Status.Phase == appsv1alpha1.CreatingClusterPhase {
		componentPhase = appsv1alpha1.CreatingClusterCompPhase
	}
	compStatus := cluster.Status.Components[componentName]
	// synchronous component phase is consistent with cluster phase
	compStatus.Phase = componentPhase
	cluster.Status.SetComponentStatus(componentName, compStatus)
}

func updateTLSVolumeAndVolumeMount(podSpec *corev1.PodSpec, clusterName string, component component.SynthesizedComponent) error {
	if !component.TLS {
		return nil
	}

	// update volume
	volumes := podSpec.Volumes
	volume, err := composeTLSVolume(clusterName, component)
	if err != nil {
		return err
	}
	volumes = append(volumes, *volume)
	podSpec.Volumes = volumes

	// update volumeMount
	for index, container := range podSpec.Containers {
		volumeMounts := container.VolumeMounts
		volumeMount := composeTLSVolumeMount()
		volumeMounts = append(volumeMounts, volumeMount)
		podSpec.Containers[index].VolumeMounts = volumeMounts
	}

	return nil
}

func composeTLSVolume(clusterName string, component component.SynthesizedComponent) (*corev1.Volume, error) {
	if !component.TLS {
		return nil, errors.New("can't compose TLS volume when TLS not enabled")
	}
	if component.Issuer == nil {
		return nil, errors.New("issuer shouldn't be nil when TLS enabled")
	}
	if component.Issuer.Name == appsv1alpha1.IssuerUserProvided && component.Issuer.SecretRef == nil {
		return nil, errors.New("secret ref shouldn't be nil when issuer is UserProvided")
	}

	var secretName, ca, cert, key string
	switch component.Issuer.Name {
	case appsv1alpha1.IssuerKubeBlocks:
		secretName = plan.GenerateTLSSecretName(clusterName, component.Name)
		ca = builder.CAName
		cert = builder.CertName
		key = builder.KeyName
	case appsv1alpha1.IssuerUserProvided:
		secretName = component.Issuer.SecretRef.Name
		ca = component.Issuer.SecretRef.CA
		cert = component.Issuer.SecretRef.Cert
		key = component.Issuer.SecretRef.Key
	}
	volume := corev1.Volume{
		Name: builder.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{
					{Key: ca, Path: builder.CAName},
					{Key: cert, Path: builder.CertName},
					{Key: key, Path: builder.KeyName},
				},
				Optional: func() *bool { o := false; return &o }(),
			},
		},
	}

	return &volume, nil
}

func composeTLSVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      builder.VolumeName,
		MountPath: builder.MountPath,
		ReadOnly:  true,
	}
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

func resolveObjectAction(snapshot clusterSnapshot, vertex *lifecycleVertex) (*Action, error) {
	gvk, err := getGVKName(vertex.obj, scheme)
	if err != nil {
		return nil, err
	}
	if obj, ok := snapshot[*gvk]; ok {
		vertex.oriObj = obj
		return actionPtr(UPDATE), nil
	} else {
		return actionPtr(CREATE), nil
	}
}
