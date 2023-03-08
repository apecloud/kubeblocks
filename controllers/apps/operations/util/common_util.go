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

// PatchClusterOpsAnnotations patches OpsRequest annotation in Cluster.annotations
func PatchClusterOpsAnnotations(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	opsRequestSlice []appsv1alpha1.OpsRecorder) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	if len(opsRequestSlice) > 0 {
		result, _ := json.Marshal(opsRequestSlice)
		cluster.Annotations[intctrlutil.OpsRequestAnnotationKey] = string(result)
	} else {
		delete(cluster.Annotations, intctrlutil.OpsRequestAnnotationKey)
	}
	return cli.Patch(ctx, cluster, patch)
}

// PatchOpsRequestReconcileAnnotation patches the reconcile annotation to OpsRequest
func PatchOpsRequestReconcileAnnotation(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, opsRequestName string) error {
	opsRequest := &appsv1alpha1.OpsRequest{}
	if err := cli.Get(ctx, client.ObjectKey{Name: opsRequestName, Namespace: cluster.Namespace}, opsRequest); err != nil {
		return err
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Annotations == nil {
		opsRequest.Annotations = map[string]string{}
	}
	// because many changes may be triggered within one second, if the accuracy is only seconds, the event may be lost.
	// so we used RFC3339Nano format.
	opsRequest.Annotations[intctrlutil.OpsRequestReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
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

// MarkRunningOpsRequestAnnotation marks reconcile annotation to the OpsRequest which is running in the cluster.
// then the related OpsRequest can reconcile.
// Note: if the client-go fetches the Cluster resources from cache,
// it should record the Cluster.ResourceVersion to check if the Cluster object from client-go is the latest in OpsRequest controller.
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
		if err = PatchOpsRequestReconcileAnnotation(ctx, cli, cluster, v.Name); err != nil && !apierrors.IsNotFound(err) {
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
