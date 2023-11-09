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

package util

import (
	"context"
	"encoding/json"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/constant"
)

func SetOpsRequestToCluster(cluster *appsv1alpha1.Cluster, opsRequestSlice []appsv1alpha1.OpsRecorder) {
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	if len(opsRequestSlice) > 0 {
		result, _ := json.Marshal(opsRequestSlice)
		cluster.Annotations[intctrlutil.OpsRequestAnnotationKey] = string(result)
	} else {
		delete(cluster.Annotations, intctrlutil.OpsRequestAnnotationKey)
	}
}

// PatchClusterOpsAnnotations patches OpsRequest annotation in Cluster.annotations
func PatchClusterOpsAnnotations(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	opsRequestSlice []appsv1alpha1.OpsRecorder) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	SetOpsRequestToCluster(cluster, opsRequestSlice)
	return cli.Patch(ctx, cluster, patch)
}

// UpdateClusterOpsAnnotations updates OpsRequest annotation in Cluster.annotations
func UpdateClusterOpsAnnotations(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	opsRequestSlice []appsv1alpha1.OpsRecorder) error {
	SetOpsRequestToCluster(cluster, opsRequestSlice)
	return cli.Update(ctx, cluster)
}

// PatchOpsRequestReconcileAnnotation patches the reconcile annotation to OpsRequest
func PatchOpsRequestReconcileAnnotation(ctx context.Context, cli client.Client, namespace string, opsRequestName string) error {
	opsRequest := &appsv1alpha1.OpsRequest{}
	if err := cli.Get(ctx, client.ObjectKey{Name: opsRequestName, Namespace: namespace}, opsRequest); err != nil {
		return err
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Annotations == nil {
		opsRequest.Annotations = map[string]string{}
	}
	// because many changes may be triggered within one second, if the accuracy is only in seconds, the event may be lost.
	// so use nanoseconds to record the time.
	opsRequest.Annotations[intctrlutil.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
	return cli.Patch(ctx, opsRequest, patch)
}

// GetOpsRequestSliceFromCluster gets OpsRequest slice from cluster annotations.
// this records what OpsRequests are running in cluster
func GetOpsRequestSliceFromCluster(cluster *appsv1alpha1.Cluster) ([]appsv1alpha1.OpsRecorder, error) {
	var (
		opsRequestValue string
		opsRequestSlice []appsv1alpha1.OpsRecorder
		ok              bool
	)
	if cluster == nil || cluster.Annotations == nil {
		return nil, nil
	}
	if opsRequestValue, ok = cluster.Annotations[intctrlutil.OpsRequestAnnotationKey]; !ok {
		return nil, nil
	}
	// opsRequest annotation value in cluster to slice
	if err := json.Unmarshal([]byte(opsRequestValue), &opsRequestSlice); err != nil {
		return nil, err
	}
	return opsRequestSlice, nil
}

// GetOpsRequestFromBackup gets OpsRequest slice from cluster annotations.
func GetOpsRequestFromBackup(backup *dpv1alpha1.Backup) *appsv1alpha1.OpsRecorder {
	var (
		opsRequestName string
		opsRequestType string
		ok             bool
	)
	if backup == nil || backup.Labels == nil {
		return nil
	}
	if opsRequestName, ok = backup.Labels[intctrlutil.OpsRequestNameLabelKey]; !ok {
		return nil
	}
	if opsRequestType, ok = backup.Labels[intctrlutil.OpsRequestTypeLabelKey]; !ok {
		return nil
	}
	return &appsv1alpha1.OpsRecorder{
		Name: opsRequestName,
		Type: appsv1alpha1.OpsType(opsRequestType),
	}
}
