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
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
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

// IsJobFinished if the job is completed or failed, return true.
// if the job is failed, return the failed message.
func IsJobFinished(job *batchv1.Job) (bool, batchv1.JobConditionType, string) {
	if job == nil {
		return false, "", ""
	}
	for _, c := range job.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		if c.Type == batchv1.JobComplete {
			return true, c.Type, ""
		}
		if c.Type == batchv1.JobFailed {
			return true, c.Type, c.Reason + ":" + c.Message
		}
	}
	return false, "", ""
}

func GetAssociatedPodsOfJob(ctx context.Context, cli client.Client, namespace, jobName string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	// from https://github.com/kubernetes/kubernetes/issues/24709
	err := cli.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabels{
		"job-name": jobName,
	})
	return podList, err
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

func GetBackupPolicyByName(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	name string) (*dpv1alpha1.BackupPolicy, error) {
	backupPolicy := &dpv1alpha1.BackupPolicy{}
	key := client.ObjectKey{
		Namespace: reqCtx.Req.Namespace,
		Name:      name,
	}
	if err := cli.Get(reqCtx.Ctx, key, backupPolicy); err != nil {
		return nil, err
	}
	return backupPolicy, nil
}

func GetBackupMethodByName(name string, backupPolicy *dpv1alpha1.BackupPolicy) *dpv1alpha1.BackupMethod {
	for _, m := range backupPolicy.Spec.BackupMethods {
		if m.Name == name {
			return &m
		}
	}
	return nil
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
	for i := range pvcNames {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(ctx, types.NamespacedName{Name: pvcNames[i], Namespace: pod.Namespace}, pvc); err != nil {
			return false, nil
		}
		enabled, err := intctrlutil.IsVolumeSnapshotEnabled(ctx, cli, pvc.Spec.VolumeName)
		if err != nil {
			return false, err
		}
		if !enabled {
			return false, fmt.Errorf(`cannot find any VolumeSnapshotClass of persistentVolumeClaim "%s" to do volume snapshot on pod "%s"`, pvc.Name, pod.Name)
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

// CovertEnvToMap coverts env array to map.
func CovertEnvToMap(env []corev1.EnvVar) map[string]string {
	envMap := map[string]string{}
	for _, v := range env {
		if v.ValueFrom != nil {
			continue
		}
		envMap[v.Name] = v.Value
	}
	return envMap
}

func GetBackupType(actionSet *dpv1alpha1.ActionSet, useSnapshot *bool) dpv1alpha1.BackupType {
	if actionSet != nil {
		return actionSet.Spec.BackupType
	} else if boolptr.IsSetToTrue(useSnapshot) {
		return dpv1alpha1.BackupTypeFull
	}
	return ""
}

// PrependSpaces prepends spaces to each line of the content.
func PrependSpaces(content string, spaces int) string {
	prefix := ""
	for i := 0; i < spaces; i++ {
		prefix += " "
	}
	r := bytes.NewBufferString(content)
	w := bytes.NewBuffer(nil)
	w.Grow(r.Len())
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			w.WriteString(prefix)
			w.WriteString(line)
		}
		if err != nil {
			break
		}
	}
	return w.String()
}

// GetFirstIndexRunningPod gets the first running pod with index.
func GetFirstIndexRunningPod(podList *corev1.PodList) *corev1.Pod {
	if podList == nil {
		return nil
	}
	sort.Slice(podList.Items, func(i, j int) bool {
		return podList.Items[i].Name < podList.Items[j].Name
	})
	for _, v := range podList.Items {
		if intctrlutil.IsAvailable(&v, 0) {
			return &v
		}
	}
	return nil
}

// GetKubeVersion get the version of Kubernetes and return the version major and minor
func GetKubeVersion() (int, int, error) {
	var err error
	verIf := viper.Get(constant.CfgKeyServerInfo)
	ver, ok := verIf.(version.Info)
	if !ok {
		return 0, 0, fmt.Errorf("failed to get kubernetes version, major %s, minor %s", ver.Major, ver.Minor)
	}

	major, err := strconv.Atoi(ver.Major)
	if err != nil {
		return 0, 0, err
	}

	// split the "normal" + and - for semver stuff to get the leading minor number
	minorStrs := strings.FieldsFunc(ver.Minor, func(r rune) bool {
		return !unicode.IsDigit(r)
	})
	if len(minorStrs) == 0 {
		return 0, 0, fmt.Errorf("failed to get kubernetes version, major %s, minor %s", ver.Major, ver.Minor)
	}

	minor, err := strconv.Atoi(minorStrs[0])
	if err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}
