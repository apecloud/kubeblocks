/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

SPDX-License-Identifier: AGPL-3.0-or-later
*/

package dataprotection

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

// Auxiliary volumes may be provisioned without data only when the same target
// instance has a volume that can restore data from this Backup. Check the owner
// templates rather than sibling PVCs, which may not have been created yet.
func (r *VolumePopulatorReconciler) validateReplicaRestoreDataVolume(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim, backup *dpv1alpha1.Backup) error {
	ref := metav1.GetControllerOf(pvc)
	if ref == nil || ref.APIVersion != workloads.GroupVersion.String() {
		return intctrlutil.NewFatalError(fmt.Sprintf("replica restore PVC %s/%s requires a workload owner", pvc.Namespace, pvc.Name))
	}
	key := client.ObjectKey{Namespace: pvc.Namespace, Name: ref.Name}
	var owner client.Object
	var volumeNames []string
	switch ref.Kind {
	case workloads.InstanceSetKind:
		its := &workloads.InstanceSet{}
		if err := r.Client.Get(reqCtx.Ctx, key, its); err != nil {
			return err
		}
		owner = its
		// ReplicaRestore supports default instances only. Instance templates and
		// custom allocation are rejected by the Cluster owner API.
		for _, template := range its.Spec.VolumeClaimTemplates {
			volumeNames = append(volumeNames, template.Name)
		}
	case "Instance":
		inst := &workloads.Instance{}
		if err := r.Client.Get(reqCtx.Ctx, key, inst); err != nil {
			return err
		}
		owner = inst
		for _, template := range inst.Spec.VolumeClaimTemplates {
			volumeNames = append(volumeNames, template.Name)
		}
	default:
		return intctrlutil.NewFatalError(fmt.Sprintf("replica restore PVC %s/%s has unsupported owner kind %s", pvc.Namespace, pvc.Name, ref.Kind))
	}
	if err := validatePVCControllerRef(pvc, ref, owner); err != nil {
		return intctrlutil.NewFatalError(err.Error())
	}
	if backup.Status.BackupMethod.TargetVolumes != nil {
		for _, name := range volumeNames {
			if utils.ExistTargetVolume(backup.Status.BackupMethod.TargetVolumes, name) {
				return nil
			}
		}
	}
	return intctrlutil.NewFatalError(fmt.Sprintf("replica restore PVC %s/%s has no workload volume matching Backup %s/%s target volumes",
		pvc.Namespace, pvc.Name, backup.Namespace, backup.Name))
}
