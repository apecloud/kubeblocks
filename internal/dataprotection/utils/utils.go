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

package utils

import (
	"context"
	"fmt"
	"reflect"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

func AddTolerations(podSpec *corev1.PodSpec) (err error) {
	if cmTolerations := viper.GetString(constant.CfgKeyCtrlrMgrTolerations); cmTolerations != "" {
		if err = json.Unmarshal([]byte(cmTolerations), &podSpec.Tolerations); err != nil {
			return err
		}
	}
	if cmAffinity := viper.GetString(constant.CfgKeyCtrlrMgrAffinity); cmAffinity != "" {
		if err = json.Unmarshal([]byte(cmAffinity), &podSpec.Affinity); err != nil {
			return err
		}
	}
	if cmNodeSelector := viper.GetString(constant.CfgKeyCtrlrMgrNodeSelector); cmNodeSelector != "" {
		if err = json.Unmarshal([]byte(cmNodeSelector), &podSpec.NodeSelector); err != nil {
			return err
		}
	}
	return nil
}

func IsJobFinished(job *batchv1.Job) (bool, batchv1.JobConditionType, string) {
	for _, c := range job.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		if c.Type == batchv1.JobComplete {
			return true, c.Type, ""
		}
		if c.Type == batchv1.JobFailed {
			return true, c.Type, c.Reason + "/" + c.Message
		}
	}
	return false, "", ""
}

func RemoveDataProtectionFinalizer(ctx context.Context, cli client.Client, obj client.Object) error {
	if !controllerutil.ContainsFinalizer(obj, dptypes.DataProtectionFinalizerName) {
		return nil
	}
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	controllerutil.RemoveFinalizer(obj, dptypes.DataProtectionFinalizerName)
	return cli.Patch(ctx, obj, patch)
}

// GetActionSetByName gets the ActionSet by name.
func GetActionSetByName(reqCtx intctrlutil.RequestCtx,
	cli client.Client, name string) (*dpv1alpha1.ActionSet, error) {
	if name == "" {
		return nil, nil
	}
	as := &dpv1alpha1.ActionSet{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: name}, as); err != nil {
		reqCtx.Log.Error(err, "failed to get ActionSet for backup.", "ActionSet", name)
		return nil, err
	}
	return as, nil
}

func GetPodListByLabelSelector(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	labelSelector metav1.LabelSelector) (*corev1.PodList, error) {
	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return nil, err
	}
	targetPodList := &corev1.PodList{}
	if err = cli.List(reqCtx.Ctx, targetPodList,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, err
	}
	return targetPodList, nil
}

func GetBackupVolumeSnapshotName(backupName, volumeSource string) string {
	return fmt.Sprintf("%s-%s", backupName, volumeSource)
}

// MergeEnv merges the targetEnv to original env. if original env exist the same name var, it will be replaced.
func MergeEnv(originalEnv, targetEnv []corev1.EnvVar) []corev1.EnvVar {
	if len(targetEnv) == 0 {
		return originalEnv
	}
	originalEnvIndexMap := map[string]int{}
	for i := range originalEnv {
		originalEnvIndexMap[originalEnv[i].Name] = i
	}
	for i := range targetEnv {
		if index, ok := originalEnvIndexMap[targetEnv[i].Name]; ok {
			originalEnv[index] = targetEnv[i]
		} else {
			originalEnv = append(originalEnv, targetEnv[i])
		}
	}
	return originalEnv
}

// VolumeSnapshotEnabled checks if the volumes support snapshot.
func VolumeSnapshotEnabled(ctx context.Context, cli client.Client, pod *corev1.Pod, volumes []string) (bool, error) {
	if pod == nil {
		return false, nil
	}
	var pvcNames []string
	// get the pvcs by volumes
	for _, v := range pod.Spec.Volumes {
		for i := range volumes {
			if v.Name != volumes[i] {
				continue
			}
			if v.PersistentVolumeClaim == nil {
				return false, fmt.Errorf(`the type of volume "%s" is not PersistentVolumeClaim on pod "%s"`, v.Name, pod.Name)
			}
			pvcNames = append(pvcNames, v.PersistentVolumeClaim.ClaimName)
		}
	}
	if len(pvcNames) == 0 {
		return false, fmt.Errorf(`can not find any volume by targetVolumes %v on pod "%s"`, volumes, pod.Name)
	}
	// get the storageClass by pvc
	storageClassMap := map[string]string{}
	for i := range pvcNames {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(ctx, types.NamespacedName{Name: pvcNames[i], Namespace: pod.Namespace}, pvc); err != nil {
			return false, nil
		}
		if pvc.Spec.StorageClassName == nil {
			return false, nil
		}
		storageClassMap[*pvc.Spec.StorageClassName] = pvcNames[i]
	}
	for k := range storageClassMap {
		enabled, err := intctrlutil.IsVolumeSnapshotEnabled(ctx, cli, k)
		if err != nil {
			return false, err
		}
		if !enabled {
			return false, fmt.Errorf(`cannot find CSI of persistentVolumeClaim "%s" to do volume snapshot on pod "%s"`, storageClassMap[k], pod.Name)
		}
	}
	return true, nil
}

func SetControllerReference(owner, controlled metav1.Object, scheme *runtime.Scheme) error {
	if owner == nil || reflect.ValueOf(owner).IsNil() {
		return nil
	}
	return controllerutil.SetControllerReference(owner, controlled, scheme)
}
