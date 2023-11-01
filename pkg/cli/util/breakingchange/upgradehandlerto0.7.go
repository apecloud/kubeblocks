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

package breakingchange

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ upgradeHandler = &upgradeHandlerTo7{}

func init() {
	registerUpgradeHandler([]string{"0.5", "0.6"}, "0.7", &upgradeHandlerTo7{})
}

type upgradeHandlerTo7 struct {
}

func (u *upgradeHandlerTo7) snapshot(dynamic dynamic.Interface) (map[string][]unstructured.Unstructured, error) {
	resourcesMap := map[string][]unstructured.Unstructured{}
	// get backupPolicy objs
	if err := fillResourcesMap(dynamic, resourcesMap, types.BackupPolicyGVR(), metav1.ListOptions{}); err != nil {
		return nil, err
	}

	// get backup objs
	if err := fillResourcesMap(dynamic, resourcesMap, types.BackupGVR(), metav1.ListOptions{}); err != nil {
		return nil, err
	}

	// get stateful_set objs
	if err := fillResourcesMap(dynamic, resourcesMap, types.StatefulSetGVR(), metav1.ListOptions{}); err != nil {
		return nil, err
	}

	// get configmap objs for pulsar
	if err := fillResourcesMap(dynamic, resourcesMap, types.ConfigmapGVR(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", constant.AppNameLabelKey, "pulsar"),
	}); err != nil {
		return nil, err
	}

	// get cluster objs for qdrant
	if err := fillResourcesMap(dynamic, resourcesMap, types.ClusterGVR(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", constant.ClusterDefLabelKey, "qdrant"),
	}); err != nil {
		return nil, err
	}

	return resourcesMap, nil
}

func (u *upgradeHandlerTo7) transform(dynamic dynamic.Interface, resourcesMap map[string][]unstructured.Unstructured) error {
	for _, resources := range resourcesMap {
		for _, obj := range resources {
			switch obj.GetKind() {
			case types.KindBackupPolicy:
				if err := u.transformBackupPolicy(dynamic, obj); err != nil {
					return err
				}
			case types.KindBackup:
				if err := u.transformBackup(dynamic, obj); err != nil {
					return err
				}
			case types.KindStatefulSet:
				if err := u.transformStatefulSet(dynamic, obj); err != nil {
					return err
				}
			case types.KindConfigMap:
				if err := u.transformConfigMap(dynamic, obj); err != nil {
					return err
				}
			case types.KindCluster:
				if err := u.transformQdrantCluster(dynamic, obj); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (u *upgradeHandlerTo7) transformBackupPolicy(dynamic dynamic.Interface, obj unstructured.Unstructured) error {
	var (
		backupMethods    []dpv1alpha1.BackupMethod
		backupTarget     *dpv1alpha1.BackupTarget
		backupRepoName   string
		newSpecData      = map[string]interface{}{}
		componentDefName = obj.GetLabels()["apps.kubeblocks.io/component-def-ref"]
		specMap, _, _    = unstructured.NestedMap(obj.Object, "spec")
	)

	isMysqlHScalePolicy := componentDefName == componentMysql && strings.Contains(obj.GetName(), "hscale")
	if !isMysqlHScalePolicy {
		// ignore mysql hscale backup policy
		if err := u.createBackupSchedule(dynamic, obj); err != nil {
			return err
		}
	}

	_, found, _ := unstructured.NestedMap(specMap, "backupMethods")
	if found {
		// if exist backupMethods, nothing to do.
		return nil
	}

	// build backup target info.
	buildBackupTarget := func(source map[string]interface{}) {
		if backupTarget != nil {
			return
		}
		matchLabels, found, _ := unstructured.NestedStringMap(source, "target", "labelsSelector", "matchLabels")
		if found {
			backupTarget = &dpv1alpha1.BackupTarget{
				PodSelector: &dpv1alpha1.PodSelector{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: matchLabels,
					},
				},
			}
			secretName, _, _ := unstructured.NestedString(source, "target", "secret", "name")
			passwordKey, _, _ := unstructured.NestedString(source, "target", "secret", "passwordKey")
			usernameKey, _, _ := unstructured.NestedString(source, "target", "secret", "usernameKey")
			backupTarget.ConnectionCredential = &dpv1alpha1.ConnectionCredential{
				SecretName:  secretName,
				PasswordKey: passwordKey,
				UsernameKey: usernameKey,
			}
		}
	}

	buildWithBackupType := func(backupType string, isMysqlHScalePolicy bool) {
		policy, found, _ := unstructured.NestedMap(specMap, backupType)
		if found {
			var backupMethod *dpv1alpha1.BackupMethod
			if backupMethod, backupRepoName = u.buildBackupMethod(componentDefName, backupType, policy); backupMethod != nil {
				if isMysqlHScalePolicy {
					backupMethod.Env = []corev1.EnvVar{
						{Name: "SIGNAL_FILE", Value: ".restore"},
					}
				}
				backupMethods = append(backupMethods, *backupMethod)
			}
			buildBackupTarget(policy)
		}
	}
	// build backupMethod/backupTarget with datafile
	buildWithBackupType(backupTypeDatafile, isMysqlHScalePolicy)

	/// build backupMethod/backupTarget with snapshot
	buildWithBackupType(backupTypeSnapshot, isMysqlHScalePolicy)
	if backupRepoName != "" {
		newSpecData["backupRepoName"] = backupRepoName
	}
	newSpecData["pathPrefix"] = obj.GetAnnotations()["dataprotection.kubeblocks.io/path-prefix"]
	newSpecData["backupMethods"] = backupMethods
	newSpecData["target"] = backupTarget
	patchBytes, _ := json.Marshal(map[string]interface{}{"spec": newSpecData})
	if _, err := dynamic.Resource(types.BackupPolicyGVR()).Namespace(obj.GetNamespace()).Patch(context.TODO(), obj.GetName(), apitypes.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("update backupPolicy %s failed: %s", obj.GetName(), err.Error())
	}
	return nil
}

// buildBackupMethod builds backupMethod for backup policy.
func (u *upgradeHandlerTo7) buildBackupMethod(componentDefName, backupType string, source map[string]interface{}) (*dpv1alpha1.BackupMethod, string) {
	backupMethod := dpv1alpha1.BackupMethod{}
	buildBackupMethod := func(methodName, actionsSetName, mountPath string, useSnapshotVolumes bool) {
		backupMethod.Name = methodName
		backupMethod.ActionSetName = actionsSetName
		backupMethod.SnapshotVolumes = &useSnapshotVolumes
		targetVolumes := &dpv1alpha1.TargetVolumeInfo{}
		if useSnapshotVolumes {
			targetVolumes.Volumes = []string{dataVolumeName}
		}
		if mountPath != "" {
			targetVolumes.VolumeMounts = []corev1.VolumeMount{{Name: dataVolumeName, MountPath: mountPath}}
		}
		backupMethod.TargetVolumes = targetVolumes
	}
	switch backupType {
	case backupTypeDatafile:
		switch componentDefName {
		case componentPostgresql:
			buildBackupMethod(pgbasebackupMethodName, pgBasebackupActionSet, pgsqlMountPath, false)
		case componentMysql:
			buildBackupMethod(xtrabackupMethodName, xtrabackupActionSet, mysqlMountPath, false)
		case componentRedis:
			buildBackupMethod(datafileMethodName, redisDatafileActionSet, redisMountPath, false)
		case componentMongodb:
			buildBackupMethod(datafileMethodName, mongoDatafileActionSet, mongodbMountPath, false)
		case componentQdrant:
			buildBackupMethod(datafileMethodName, qdrantSnapshotActionSet, qdrantMountPath, false)
		}
	case backupTypeSnapshot:
		switch componentDefName {
		case componentMysql:
			buildBackupMethod(volumeSnapshotMethodName, volumeSnapshotForMysql, mysqlMountPath, true)
		case componentMongodb:
			buildBackupMethod(volumeSnapshotMethodName, volumeSnapshotForMongo, mongodbMountPath, true)
		default:
			buildBackupMethod(volumeSnapshotMethodName, "", "", true)
		}
	default:
		return nil, ""
	}
	backupRepoName, _, _ := unstructured.NestedString(source, "backupRepoName")
	return &backupMethod, backupRepoName
}

// createBackupSchedule creates the backup schedule by backup policy.
func (u *upgradeHandlerTo7) createBackupSchedule(dynamic dynamic.Interface, obj unstructured.Unstructured) error {
	_, found, _ := unstructured.NestedMap(obj.Object, "spec", "schedule")
	if !found {
		return nil
	}
	schedule := &dpv1alpha1.BackupSchedule{
		TypeMeta: metav1.TypeMeta{
			Kind:       types.KindBackupSchedule,
			APIVersion: types.BackupScheduleGVR().GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        strings.Replace(obj.GetName(), "backup-policy", "backup-schedule", 1),
			Namespace:   obj.GetNamespace(),
			Labels:      obj.GetLabels(),
			Annotations: obj.GetAnnotations(),
		},
		Spec: dpv1alpha1.BackupScheduleSpec{
			BackupPolicyName: obj.GetName(),
			Schedules:        u.buildBackupMethodSchedule(obj),
		},
	}

	startingDeadlineMinutes, found, _ := unstructured.NestedInt64(obj.Object, "spec", "schedule", "startingDeadlineMinutes")
	if found {
		schedule.Spec.StartingDeadlineMinutes = &startingDeadlineMinutes
	}
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&schedule)
	if err != nil {
		return err
	}
	_, err = dynamic.Resource(types.BackupScheduleGVR()).Namespace(obj.GetNamespace()).Create(context.TODO(),
		&unstructured.Unstructured{Object: unstructuredMap}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create backupSchedule %s failed: %s", schedule.Name, err.Error())
	}
	return nil
}

func (u *upgradeHandlerTo7) buildBackupMethodSchedule(obj unstructured.Unstructured) []dpv1alpha1.SchedulePolicy {
	var schedulePolicies []dpv1alpha1.SchedulePolicy
	sourceSchedule, _, _ := unstructured.NestedMap(obj.Object, "spec", "schedule")
	retentionPeriod, _, _ := unstructured.NestedString(obj.Object, "spec", "retention", "ttl")
	buildSchedulePolicy := func(backupMethod string, oldSchedule map[string]interface{}) dpv1alpha1.SchedulePolicy {
		cronExpression, _, _ := unstructured.NestedString(oldSchedule, "cronExpression")
		enabled, _, _ := unstructured.NestedBool(oldSchedule, "enable")
		return dpv1alpha1.SchedulePolicy{
			BackupMethod:    backupMethod,
			CronExpression:  cronExpression,
			RetentionPeriod: dpv1alpha1.RetentionPeriod(retentionPeriod),
			Enabled:         &enabled,
		}
	}
	datafile, _, _ := unstructured.NestedMap(sourceSchedule, "datafile")
	snapshot, _, _ := unstructured.NestedMap(sourceSchedule, "snapshot")
	componentDefName := obj.GetLabels()["apps.kubeblocks.io/component-def-ref"]
	switch componentDefName {
	case componentMysql:
		schedulePolicies = append(schedulePolicies, buildSchedulePolicy(xtrabackupMethodName, datafile))
	case componentMongodb:
		schedulePolicy := buildSchedulePolicy(datafileMethodName, datafile)
		// Note: will set dump by default, datafile tool may lead inconsistent backup data.
		schedulePolicies = append(schedulePolicies, dpv1alpha1.SchedulePolicy{
			BackupMethod:    "dump",
			CronExpression:  schedulePolicy.CronExpression,
			RetentionPeriod: schedulePolicy.RetentionPeriod,
			Enabled:         schedulePolicy.Enabled,
		})
		// close the datafile schedule
		var enable bool
		schedulePolicy.Enabled = &enable
		schedulePolicies = append(schedulePolicies, schedulePolicy)
	case componentPostgresql:
		schedulePolicies = append(schedulePolicies, buildSchedulePolicy(pgbasebackupMethodName, datafile))
	case componentRedis, componentQdrant:
		schedulePolicies = append(schedulePolicies, buildSchedulePolicy(datafileMethodName, datafile))
	}
	// set volume-snapshot
	schedulePolicies = append(schedulePolicies, buildSchedulePolicy(volumeSnapshotMethodName, snapshot))
	return schedulePolicies
}

func (u *upgradeHandlerTo7) transformBackup(dynamic dynamic.Interface, obj unstructured.Unstructured) error {
	var (
		newSpecData     = map[string]interface{}{}
		newStatusData   = map[string]interface{}{}
		specMap, _, _   = unstructured.NestedMap(obj.Object, "spec")
		statusMap, _, _ = unstructured.NestedMap(obj.Object, "status")
		compName        = obj.GetLabels()["apps.kubeblocks.io/component-name"]
		backupMethodKey = "backupMethod"
	)

	backupMethod, _, _ := unstructured.NestedString(specMap, "backupMethod")
	if backupMethod != "" {
		// if exist backupMethod, nothing to do.
		return nil
	}

	// covert spec of backup
	backupToolName, _, _ := unstructured.NestedString(statusMap, "backupToolName")
	backupType, _, _ := unstructured.NestedString(specMap, "backupType")
	switch backupType {
	case backupTypeSnapshot:
		newSpecData[backupMethodKey] = volumeSnapshotMethodName
	case backupTypeDatafile:
		switch {
		case strings.Contains(backupToolName, "basebackup"):
			newSpecData[backupMethodKey] = pgbasebackupMethodName
		case strings.Contains(backupToolName, "apecloud-mysql"):
			newSpecData[backupMethodKey] = xtrabackupMethodName
		default:
			newSpecData[backupMethodKey] = datafileMethodName
		}
	case backupTypeLogfile:
		// Note: set a non-existent method for required value.
		newSpecData[backupMethodKey] = backupTypeLogfile
	}
	newSpecData["deletionPolicy"] = dpv1alpha1.BackupDeletionPolicyDelete
	patchBytes, _ := json.Marshal(map[string]interface{}{"spec": newSpecData})
	if _, err := dynamic.Resource(types.BackupGVR()).Namespace(obj.GetNamespace()).Patch(context.TODO(), obj.GetName(), apitypes.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("update backup %s failed: %s", obj.GetName(), err.Error())
	}

	// covert status of backup
	newStatusData["persistentVolumeClaimName"] = statusMap["persistentVolumeClaimName"]
	newStatusData["completionTimestamp"] = statusMap["completionTimestamp"]
	newStatusData["startTimestamp"] = statusMap["startTimestamp"]
	newStatusData["backupRepoName"] = statusMap["backupRepoName"]
	newStatusData["expiration"] = statusMap["expiration"]
	newStatusData["totalSize"] = statusMap["totalSize"]
	newStatusData["duration"] = statusMap["duration"]
	newStatusData["phase"] = statusMap["phase"]
	if newStatusData["phase"] == "InProgress" {
		newStatusData["phase"] = dpv1alpha1.BackupPhaseRunning
	}
	// covert timeRange
	manifests, _, _ := unstructured.NestedMap(statusMap, "manifests")
	if manifests != nil {
		backupLog, _, _ := unstructured.NestedMap(manifests, "backupLog")
		newStatusData["timeRange"] = map[string]interface{}{
			"end":   backupLog["stopTime"],
			"start": backupLog["startTime"],
		}
		backupTool, _, _ := unstructured.NestedMap(manifests, "backupTool")
		newStatusData["path"] = backupTool["filePath"]
	}
	// covert backupMethod of status
	newObj, err := dynamic.Resource(types.BackupGVR()).Namespace(obj.GetNamespace()).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
	if err != nil {
		return err
	}
	var (
		useSnapshotVolumes bool
		volumes            []string
		volumeMounts       []corev1.VolumeMount
		actionSetName      string
	)
	switch backupType {
	case backupTypeSnapshot:
		useSnapshotVolumes = true
		volumes = append(volumes, dataVolumeName)
		if compName == componentMysql {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: dataVolumeName, MountPath: mysqlMountPath})
			actionSetName = volumeSnapshotForMysql
		} else if compName == componentMongodb {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: dataVolumeName, MountPath: mongodbMountPath})
			actionSetName = volumeSnapshotForMongo
		}
	case backupTypeDatafile:
		switch {
		case strings.Contains(backupToolName, "basebackup"):
			volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: dataVolumeName, MountPath: pgsqlMountPath})
			actionSetName = pgBasebackupActionSet
		case strings.Contains(backupToolName, "apecloud-mysql"):
			volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: dataVolumeName, MountPath: mysqlMountPath})
			actionSetName = xtrabackupActionSet
		case strings.Contains(backupToolName, "redis"):
			volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: dataVolumeName, MountPath: redisMountPath})
			actionSetName = redisDatafileActionSet
		case strings.Contains(backupToolName, "mongodb"):
			volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: dataVolumeName, MountPath: mongodbMountPath})
			actionSetName = mongoDatafileActionSet
		case strings.Contains(backupToolName, "qdrant"):
			volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: dataVolumeName, MountPath: qdrantMountPath})
			actionSetName = qdrantSnapshotActionSet
		}
	}
	newStatusData[backupMethodKey] = map[string]interface{}{
		"name":            newSpecData[backupMethodKey],
		"actionSetName":   actionSetName,
		"snapshotVolumes": useSnapshotVolumes,
		"targetVolumes": map[string]interface{}{
			"volumes":      volumes,
			"volumeMounts": volumeMounts,
		},
	}
	newObj.Object["status"] = newStatusData
	if _, err := dynamic.Resource(types.BackupGVR()).Namespace(newObj.GetNamespace()).UpdateStatus(context.TODO(), newObj, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update status of backup %s failed: %s", obj.GetName(), err.Error())
	}
	return nil
}

func (u *upgradeHandlerTo7) transformStatefulSet(dynamic dynamic.Interface, obj unstructured.Unstructured) error {
	// filter objects not managed by KB
	labels := obj.GetLabels()
	if labels == nil || labels[constant.AppManagedByLabelKey] != constant.AppName {
		return nil
	}
	// create a rsm
	serviceName, _, _ := unstructured.NestedString(obj.Object, "spec", "serviceName")
	matchLabels, _, _ := unstructured.NestedStringMap(obj.Object, "spec", "selector", "matchLabels")
	replicas, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	podManagementPolicy, _, _ := unstructured.NestedString(obj.Object, "spec", "podManagementPolicy")
	pvcsUnstructured, _, _ := unstructured.NestedSlice(obj.Object, "spec", "volumeClaimTemplates")
	var pvcs []corev1.PersistentVolumeClaim
	for _, pvcUnstructured := range pvcsUnstructured {
		pvc := &corev1.PersistentVolumeClaim{}
		pvcU, _ := pvcUnstructured.(map[string]interface{})
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(pvcU, pvc); err != nil {
			return err
		}
		pvcs = append(pvcs, *pvc)
	}
	template, _, _ := unstructured.NestedMap(obj.Object, "spec", "template")
	podTemplate := &corev1.PodTemplateSpec{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(template, podTemplate); err != nil {
		return err
	}
	updateStrategy, _, _ := unstructured.NestedString(obj.Object, "spec", "updateStrategy")
	rsm := builder.NewReplicatedStateMachineBuilder(obj.GetNamespace(), obj.GetName()).
		AddAnnotationsInMap(obj.GetAnnotations()).
		AddLabelsInMap(obj.GetLabels()).
		SetServiceName(serviceName).
		AddMatchLabelsInMap(matchLabels).
		SetReplicas(int32(replicas)).
		SetPodManagementPolicy(v1.PodManagementPolicyType(podManagementPolicy)).
		SetVolumeClaimTemplates(pvcs...).
		SetTemplate(*podTemplate).
		SetUpdateStrategyType(v1.StatefulSetUpdateStrategyType(updateStrategy)).
		SetPaused(true).
		GetObject()
	gvk := schema.GroupVersionKind{
		Group:   types.RSMGVR().Group,
		Version: types.RSMGVR().Version,
		Kind:    types.KindRSM,
	}
	rsm.SetGroupVersionKind(gvk)

	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rsm)
	if err != nil {
		return err
	}
	_, err = dynamic.Resource(types.RSMGVR()).Namespace(obj.GetNamespace()).Create(context.TODO(),
		&unstructured.Unstructured{Object: unstructuredMap}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create rsm %s failed: %s", rsm.Name, err.Error())
	}
	return nil
}

func (u *upgradeHandlerTo7) transformConfigMap(dynamic dynamic.Interface, obj unstructured.Unstructured) error {
	// transform pulsar broker env config map
	configData, _, _ := unstructured.NestedMap(obj.Object, "data")
	zkSVC, ok := configData["zookeeperSVC"]
	if !ok {
		return nil
	}
	zkServers := fmt.Sprintf("%s:2181", zkSVC)
	configData["zookeeperServers"] = zkServers
	configData["configurationStoreServers"] = zkServers
	patchBytes, _ := json.Marshal(map[string]interface{}{"data": configData})
	if _, err := dynamic.Resource(types.ConfigmapGVR()).Namespace(obj.GetNamespace()).Patch(context.TODO(), obj.GetName(), apitypes.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("update pulsar configmap %s failed: %s", obj.GetName(), err.Error())
	}
	return nil
}

func (u *upgradeHandlerTo7) transformQdrantCluster(dynamic dynamic.Interface, obj unstructured.Unstructured) error {
	labels := obj.GetLabels()
	if labels[constant.ClusterDefLabelKey] != "qdrant" {
		return nil
	}
	specMap, _, _ := unstructured.NestedMap(obj.Object, "spec")
	specMap["clusterVersionRef"] = "qdrant-1.5.0"
	patchBytes, _ := json.Marshal(map[string]interface{}{"spec": specMap})
	if _, err := dynamic.Resource(types.ClusterGVR()).Namespace(obj.GetNamespace()).Patch(context.TODO(), obj.GetName(), apitypes.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("update pulsar configmap %s failed: %s", obj.GetName(), err.Error())
	}

	return nil
}
