/*
Copyright ApeCloud Inc.

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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

const (
	// annotation keys

	RestartAnnotationKey = "kubeblocks.io/restart"
)

type OpsBehaviour struct {
	FromClusterPhases []dbaasv1alpha1.Phase
	ToClusterPhase    dbaasv1alpha1.Phase
	// Action The action running time should be short. if it fails, it will be reconciled by the OpsRequest controller.
	// if you do not want to be reconciled when the operation fails,
	// you need to call PatchOpsStatus function in ops_util.go and set OpsRequest.status.phase to Failed
	Action func(opsResource *OpsResource) error
	// ReconcileAction loop until the operation is completed.
	// if true, the operation is execution completed. otherwise, the operation is running.
	ReconcileAction func(opsResource *OpsResource) (bool, error)
	// ActionStartedCondition append to OpsRequest.status.conditions when start performing Action function
	ActionStartedCondition func(opsRequest *dbaasv1alpha1.OpsRequest) *metav1.Condition
}

type OpsResource struct {
	Ctx        context.Context
	Client     client.Client
	OpsRequest *dbaasv1alpha1.OpsRequest
	Cluster    *dbaasv1alpha1.Cluster
	Recorder   record.EventRecorder
}

type OpsManager struct {
	OpsMap map[dbaasv1alpha1.OpsType]*OpsBehaviour
}
