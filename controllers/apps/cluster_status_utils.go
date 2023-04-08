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

package apps

import (
	"context"
	"regexp"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	"github.com/apecloud/kubeblocks/internal/constant"
)

// EventTimeOut timeout of the event
const EventTimeOut = 30 * time.Second

// isTargetKindForEvent checks the event involve object is the target resources
func isTargetKindForEvent(event *corev1.Event) bool {
	return slices.Index([]string{constant.PodKind, constant.DeploymentKind, constant.StatefulSetKind}, event.InvolvedObject.Kind) != -1
}

// getEventInvolvedObject gets event involved object for StatefulSet/Deployment/Pod workload
func getEventInvolvedObject(ctx context.Context, cli client.Client, event *corev1.Event) (client.Object, error) {
	objectKey := client.ObjectKey{
		Name:      event.InvolvedObject.Name,
		Namespace: event.InvolvedObject.Namespace,
	}
	var err error
	// If client.object interface object is used as a parameter, it will not return an error when the object is not found.
	// so we should specify the object type to get the object.
	switch event.InvolvedObject.Kind {
	case constant.PodKind:
		pod := &corev1.Pod{}
		err = cli.Get(ctx, objectKey, pod)
		return pod, err
	case constant.StatefulSetKind:
		sts := &appsv1.StatefulSet{}
		err = cli.Get(ctx, objectKey, sts)
		return sts, err
	case constant.DeploymentKind:
		deployment := &appsv1.Deployment{}
		err = cli.Get(ctx, objectKey, deployment)
		return deployment, err
	}
	return nil, err
}

// handleClusterPhaseWhenCompsNotReady handles the Cluster.status.phase when some components are Abnormal or Failed.
// TODO: Clear definitions need to be added to determine whether components will affect cluster availability in ClusterDefinition.
func handleClusterPhaseWhenCompsNotReady(cluster *appsv1alpha1.Cluster,
	componentMap map[string]string,
	clusterAvailabilityEffectMap map[string]bool) {
	var (
		clusterIsFailed   bool
		failedCompCount   int
		isVolumeExpanding bool
	)
	opsRecords, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	if len(opsRecords) != 0 && opsRecords[0].Type == appsv1alpha1.VolumeExpansionType {
		isVolumeExpanding = true
	}
	for k, v := range cluster.Status.Components {
		// determine whether other components are still doing operation, i.e., create/restart/scaling.
		// waiting for operation to complete except for volumeExpansion operation.
		// because this operation will not affect cluster availability.
		if !slices.Contains(appsv1alpha1.GetComponentTerminalPhases(), v.Phase) && !isVolumeExpanding {
			return
		}
		if v.Phase == appsv1alpha1.FailedClusterCompPhase {
			failedCompCount += 1
			componentDefName := componentMap[k]
			// if the component can affect cluster availability, set Cluster.status.phase to Failed
			if clusterAvailabilityEffectMap[componentDefName] {
				clusterIsFailed = true
				break
			}
		}
	}
	// If all components fail or there are failed components that affect the availability of the cluster, set phase to Failed
	if failedCompCount == len(cluster.Status.Components) || clusterIsFailed {
		cluster.Status.Phase = appsv1alpha1.FailedClusterPhase
	} else {
		cluster.Status.Phase = appsv1alpha1.AbnormalClusterPhase
	}
}

// handleClusterStatusByEvent handles the cluster status when warning event happened
func handleClusterStatusByEvent(ctx context.Context, cli client.Client, recorder record.EventRecorder, event *corev1.Event) error {
	var (
		err error
	)
	object, err := getEventInvolvedObject(ctx, cli, event)
	if err != nil {
		return err
	}
	return notifyClusterStatusChange(ctx, cli, object)
}

// TODO: Unified cluster event processing
// handleEventForClusterStatus handles event for cluster Warning and Failed phase
func handleEventForClusterStatus(ctx context.Context, cli client.Client, recorder record.EventRecorder, event *corev1.Event) error {

	type predicateProcessor struct {
		pred      func() bool
		processor func() error
	}

	nilReturnHandler := func() error { return nil }

	pps := []predicateProcessor{
		{
			// handle cronjob complete or fail event
			pred: func() bool {
				return event.InvolvedObject.Kind == constant.CronJobKind &&
					event.Reason == "SawCompletedJob"
			},
			processor: func() error {
				return handleDeletePVCCronJobEvent(ctx, cli, recorder, event)
			},
		},
		{
			pred: func() bool {
				return event.Type != corev1.EventTypeWarning ||
					!isTargetKindForEvent(event)
			},
			processor: nilReturnHandler,
		},
		{
			pred: func() bool {
				// the error repeated several times, so we can sure it's a real error to the cluster.
				return !k8score.IsOvertimeEvent(event, EventTimeOut)
			},
			processor: nilReturnHandler,
		},
		{
			// handle cluster workload error events such as pod/statefulset/deployment errors
			// must be the last one
			pred: func() bool {
				return true
			},
			processor: func() error {
				return handleClusterStatusByEvent(ctx, cli, recorder, event)
			},
		},
	}

	for _, pp := range pps {
		if pp.pred() {
			return pp.processor()
		}
	}
	return nil
}

func handleDeletePVCCronJobEvent(ctx context.Context,
	cli client.Client,
	recorder record.EventRecorder,
	event *corev1.Event) error {
	re := regexp.MustCompile("status: Failed")
	var (
		err    error
		object client.Object
	)
	matches := re.FindStringSubmatch(event.Message)
	if len(matches) == 0 {
		// TODO(refactor): introduce a one-shot delayed job to delete the pvc object.
		// delete pvc success, then delete cronjob
		return checkedDeleteDeletePVCCronJob(ctx, cli, event.InvolvedObject.Name, event.InvolvedObject.Namespace)
	}
	// cronjob failed
	if object, err = getEventInvolvedObject(ctx, cli, event); err != nil {
		return err
	}
	// TODO(refactor): we will lost the event and event.Message
	return notifyClusterStatusChange(ctx, cli, object)
}

func checkedDeleteDeletePVCCronJob(ctx context.Context, cli client.Client, name string, namespace string) error {
	// label check
	cronJob := batchv1.CronJob{}
	if err := cli.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, &cronJob); err != nil {
		return client.IgnoreNotFound(err)
	}
	if cronJob.ObjectMeta.Labels[constant.AppManagedByLabelKey] != constant.AppName {
		return nil
	}
	// check the delete-pvc-cronjob annotation.
	// the reason for this is that the backup policy also creates cronjobs,
	// which need to be distinguished by the annotation.
	if cronJob.ObjectMeta.Annotations[lifecycleAnnotationKey] != lifecycleDeletePVCAnnotation {
		return nil
	}
	// if managed by kubeblocks, then it must be the cronjob used to delete pvc, delete it since it's completed
	if err := cli.Delete(ctx, &cronJob); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

// existsOperations checks if the cluster is doing operations
func existsOperations(cluster *appsv1alpha1.Cluster) bool {
	opsRequestMap, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	_, isRestoring := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	return len(opsRequestMap) > 0 || isRestoring
}
