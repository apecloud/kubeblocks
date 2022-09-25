/*
Copyright 2022.

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

package operations

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

func init() {
	volumeExpansionBehaviour := &OpsBehaviour{
		FromClusterPhases:      []dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase},
		ToClusterPhase:         dbaasv1alpha1.UpdatingPhase,
		Action:                 VolumeExpansionAction,
		ActionStartedCondition: dbaasv1alpha1.NewVolumeExpandingCondition,
		ReconcileAction:        ReconcileActionWithComponentOps,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.VolumeExpansionType, volumeExpansionBehaviour)
}

// VolumeExpansionAction Modify Cluster.spec.components[*].VolumeClaimTemplates[*].spec.resources
func VolumeExpansionAction(opsRes *OpsResource) error {
	componentNameMap := getComponentsNameMap(opsRes.OpsRequest)
	volumeExpansionMap := map[string]string{}
	validateMsgs := make([]string, 0)
	for _, v := range opsRes.OpsRequest.Spec.ComponentOps.VolumeExpansion {
		volumeExpansionMap[v.Name] = v.Storage
	}
	for index, component := range opsRes.Cluster.Spec.Components {
		if _, ok := componentNameMap[component.Name]; !ok {
			continue
		}
		for i, vc := range component.VolumeClaimTemplates {
			if v, ok := volumeExpansionMap[vc.Name]; ok {
				if msg, err := validateStorageClass(opsRes, opsRes.Cluster.Spec.Components[index].
					VolumeClaimTemplates[i].Spec, component.Name, vc.Name); err != nil {
					return err
				} else if msg != nil {
					validateMsgs = append(validateMsgs, *msg)
				}
				opsRes.Cluster.Spec.Components[index].VolumeClaimTemplates[i].
					Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(v)
			}
		}
	}
	if len(validateMsgs) > 0 {
		return patchValidateVolumeExpansionError(opsRes, validateMsgs)
	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}

// validateStorageClass validate storageClass whether support allowVolumeExpansion
func validateStorageClass(opsRes *OpsResource, pvcSpec corev1.PersistentVolumeClaimSpec, componentName, vcName string) (*string, error) {
	if pvcSpec.StorageClassName != nil {
		return checkStorageClass(opsRes, pvcSpec)
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := opsRes.Client.List(opsRes.Ctx, pvcList, client.InNamespace(opsRes.Cluster.Namespace),
		client.MatchingLabels{
			AppInstanceLabelKey:      opsRes.Cluster.Name,
			AppComponentNameLabelKey: componentName,
		}); err != nil {
		return nil, err
	}
	for _, v := range pvcList.Items {
		if strings.HasPrefix(v.Name, vcName) && v.Spec.StorageClassName != nil {
			return checkStorageClass(opsRes, v.Spec)
		}
	}
	return nil, nil
}

func checkStorageClass(opsRes *OpsResource, pvc corev1.PersistentVolumeClaimSpec) (*string, error) {
	sc := &storagev1.StorageClass{}
	if err := opsRes.Client.Get(opsRes.Ctx, types.NamespacedName{Name: *pvc.StorageClassName}, sc); apierrors.IsNotFound(err) {
		message := fmt.Sprintf("StorageClass: %s is not found", sc.Name)
		return &message, nil
	} else if err != nil {
		return nil, err
	}
	return checkAllowVolumeExpansion(sc)
}

func checkAllowVolumeExpansion(sc *storagev1.StorageClass) (*string, error) {
	if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
		message := fmt.Sprintf("StorageClass: %s not support volumeExpansion", sc.Name)
		return &message, nil
	}
	return nil, nil
}

func patchValidateVolumeExpansionError(opsRes *OpsResource, msgs []string) error {
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonVolumeExpansionValidateError, strings.Join(msgs, ";"))
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}
