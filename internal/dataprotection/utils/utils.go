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
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true, c.Type, ""
		}
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
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

func VolumeSnapshotEnabled() bool {
	return viper.GetBool("VOLUMESNAPSHOT")
}

func SetControllerReference(owner, controlled metav1.Object, scheme *runtime.Scheme) error {
	if owner == nil || reflect.ValueOf(owner).IsNil() {
		return nil
	}
	return ctrlutil.SetControllerReference(owner, controlled, scheme)
}
