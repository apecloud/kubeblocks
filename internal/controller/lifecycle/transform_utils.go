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
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func newRequeueError(after time.Duration, reason string) error {
	return &realRequeueError{
		reason:       reason,
		requeueAfter: after,
	}
}

func isClusterDeleting(cluster appsv1alpha1.Cluster) bool {
	return !cluster.GetDeletionTimestamp().IsZero()
}

func isClusterUpdating(cluster appsv1alpha1.Cluster) bool {
	return cluster.Status.ObservedGeneration != cluster.Generation
}

func isClusterStatusUpdating(cluster appsv1alpha1.Cluster) bool {
	return !isClusterDeleting(cluster) && !isClusterUpdating(cluster)
	// return cluster.Status.ObservedGeneration == cluster.Generation &&
	//	slices.Contains(appsv1alpha1.GetClusterTerminalPhases(), cluster.Status.Phase)
}

func getBackupObjects(reqCtx intctrlutil.RequestCtx,
	cli types2.ReadonlyClient,
	namespace string,
	backupName string) (*dataprotectionv1alpha1.Backup, *dataprotectionv1alpha1.BackupTool, error) {
	// get backup
	backup := &dataprotectionv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: backupName, Namespace: namespace}, backup); err != nil {
		return nil, nil, err
	}

	// get backup tool
	backupTool := &dataprotectionv1alpha1.BackupTool{}
	if backup.Spec.BackupType != dataprotectionv1alpha1.BackupTypeSnapshot {
		if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: backup.Status.BackupToolName}, backupTool); err != nil {
			return nil, nil, err
		}
	}
	return backup, backupTool, nil
}

func isTypeOf[T interface{}](obj client.Object) bool {
	_, ok := obj.(T)
	return ok
}

// getBackupPolicyFromTemplate gets backup policy from template policy template.
func getBackupPolicyFromTemplate(reqCtx intctrlutil.RequestCtx,
	cli types2.ReadonlyClient,
	cluster *appsv1alpha1.Cluster,
	componentDef,
	backupPolicyTemplateName string) (*dataprotectionv1alpha1.BackupPolicy, error) {
	backupPolicyList := &dataprotectionv1alpha1.BackupPolicyList{}
	if err := cli.List(reqCtx.Ctx, backupPolicyList,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			constant.AppInstanceLabelKey:          cluster.Name,
			constant.KBAppComponentDefRefLabelKey: componentDef,
		}); err != nil {
		return nil, err
	}
	for _, backupPolicy := range backupPolicyList.Items {
		if backupPolicy.Annotations[constant.BackupPolicyTemplateAnnotationKey] == backupPolicyTemplateName {
			return &backupPolicy, nil
		}
	}
	return nil, nil
}
