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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/constant"
)

func setOpsRequestToCluster(cluster *appsv1alpha1.Cluster, opsRequestSlice []appsv1alpha1.OpsRecorder) {
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
	setOpsRequestToCluster(cluster, opsRequestSlice)
	return cli.Patch(ctx, cluster, patch)
}

// UpdateClusterOpsAnnotations updates OpsRequest annotation in Cluster.annotations
func UpdateClusterOpsAnnotations(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	opsRequestSlice []appsv1alpha1.OpsRecorder) error {
	setOpsRequestToCluster(cluster, opsRequestSlice)
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
	// because many changes may be triggered within one second, if the accuracy is only seconds, the event may be lost.
	// so use nanoseconds to record the time.
	opsRequest.Annotations[intctrlutil.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
	return cli.Patch(ctx, opsRequest, patch)
}

//// PatchOpsRequestReconcileAnnotation2 patches the reconcile annotation to OpsRequest
// func PatchOpsRequestReconcileAnnotation2(ctx context.Context, cli client.Client, namespace string, opsRequestName string, dag *graph.DAG) error {
//	opsRequest := &appsv1alpha1.OpsRequest{}
//	if err := cli.Get(ctx, client.ObjectKey{Name: opsRequestName, Namespace: namespace}, opsRequest); err != nil {
//		return err
//	}
//
//	opsRequestDeepCopy := opsRequest.DeepCopy()
//	if opsRequest.Annotations == nil {
//		opsRequest.Annotations = map[string]string{}
//	}
//	// because many changes may be triggered within one second, if the accuracy is only seconds, the event may be lost.
//	// so use nanoseconds to record the time.
//	opsRequest.Annotations[intctrlutil.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
//
//	types.AddVertex4Patch(dag, opsRequest, opsRequestDeepCopy)
//	return nil
// }

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

// MarkRunningOpsRequestAnnotation marks reconcile annotation to the OpsRequest which is running in the cluster.
// then the related OpsRequest can reconcile.
// Note: if the client-go fetches the Cluster resources from cache,
// it should record the Cluster.ResourceVersion to check if the Cluster object from client-go is the latest in OpsRequest controller.
// @return could return ErrNoOps
func MarkRunningOpsRequestAnnotation(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster) error {
	var (
		opsRequestSlice []appsv1alpha1.OpsRecorder
		err             error
	)
	if opsRequestSlice, err = GetOpsRequestSliceFromCluster(cluster); err != nil {
		return err
	}
	// mark annotation for operations
	var notExistOps = map[string]struct{}{}
	for _, v := range opsRequestSlice {
		if err = PatchOpsRequestReconcileAnnotation(ctx, cli, cluster.Namespace, v.Name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if apierrors.IsNotFound(err) {
			notExistOps[v.Name] = struct{}{}
		}
	}
	if len(notExistOps) != 0 {
		return RemoveClusterInvalidOpsRequestAnnotation(ctx, cli, cluster, opsRequestSlice, notExistOps)
	}
	return nil
}

//// MarkRunningOpsRequestAnnotation2 marks reconcile annotation to the OpsRequest which is running in the cluster.
//// then the related OpsRequest can reconcile.
//// Note: if the client-go fetches the Cluster resources from cache,
//// it should record the Cluster.ResourceVersion to check if the Cluster object from client-go is the latest in OpsRequest controller.
//// @return could return ErrNoOps
// func MarkRunningOpsRequestAnnotation2(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, dag *graph.DAG) error {
//	var (
//		opsRequestSlice []appsv1alpha1.OpsRecorder
//		err             error
//	)
//	if opsRequestSlice, err = GetOpsRequestSliceFromCluster(cluster); err != nil {
//		return err
//	}
//	// mark annotation for operations
//	var notExistOps = map[string]struct{}{}
//	for _, v := range opsRequestSlice {
//		if err = PatchOpsRequestReconcileAnnotation2(ctx, cli, cluster.Namespace, v.Name, dag); err != nil && !apierrors.IsNotFound(err) {
//			return err
//		}
//		if apierrors.IsNotFound(err) {
//			notExistOps[v.Name] = struct{}{}
//		}
//	}
//	if len(notExistOps) != 0 {
//		return RemoveClusterInvalidOpsRequestAnnotation2(ctx, cli, cluster, opsRequestSlice, notExistOps)
//	}
//	return nil
// }

// RemoveClusterInvalidOpsRequestAnnotation deletes the OpsRequest annotation in cluster when the OpsRequest not existing.
func RemoveClusterInvalidOpsRequestAnnotation(
	ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	opsRequestSlice []appsv1alpha1.OpsRecorder,
	notExistOps map[string]struct{}) error {
	// delete the OpsRequest annotation in cluster when the OpsRequest not existing.
	newOpsRequestSlice := make([]appsv1alpha1.OpsRecorder, 0, len(opsRequestSlice))
	for _, v := range opsRequestSlice {
		if _, ok := notExistOps[v.Name]; ok {
			continue
		}
		newOpsRequestSlice = append(newOpsRequestSlice, v)
	}
	return PatchClusterOpsAnnotations(ctx, cli, cluster, newOpsRequestSlice)
}

//// RemoveClusterInvalidOpsRequestAnnotation2 deletes the OpsRequest annotation in cluster when the OpsRequest not existing.
// func RemoveClusterInvalidOpsRequestAnnotation2(ctx context.Context,
//	cli client.Client,
//	cluster *appsv1alpha1.Cluster,
//	opsRequestSlice []appsv1alpha1.OpsRecorder,
//	notExistOps map[string]struct{}) error {
//	// delete the OpsRequest annotation in cluster when the OpsRequest not existing.
//	newOpsRequestSlice := make([]appsv1alpha1.OpsRecorder, 0, len(opsRequestSlice))
//	for _, v := range opsRequestSlice {
//		if _, ok := notExistOps[v.Name]; ok {
//			continue
//		}
//		newOpsRequestSlice = append(newOpsRequestSlice, v)
//	}
//	setOpsRequestToCluster(cluster, opsRequestSlice)
//	return nil
// }
