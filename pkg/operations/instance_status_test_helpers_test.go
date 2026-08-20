/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"encoding/json"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/workloads/instancestatus"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

// publishInstanceSetStatus uses the same allocation and status builders as the InstanceSet controller.
// Tests still provide the observed Pods, but do not handcraft InstanceStatus entries.
func publishInstanceSetStatus(cluster *appsv1.Cluster, compName string) {
	itsName := constant.GenerateClusterComponentName(cluster.Name, compName)
	compSpec := cluster.Spec.GetComponentByName(compName)
	Expect(compSpec).ShouldNot(BeNil())
	Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKey{Name: itsName, Namespace: cluster.Namespace}, func(its *workloads.InstanceSet) {
		replicas := compSpec.Replicas
		its.Spec.Replicas = &replicas
		its.Spec.Ordinals = compSpec.Ordinals
		its.Spec.FlatInstanceOrdinal = compSpec.FlatInstanceOrdinal
		its.Spec.OfflineInstances = append([]string(nil), compSpec.OfflineInstances...)
		its.Spec.Stop = compSpec.Stop
		its.Spec.Instances = make([]workloads.InstanceTemplate, len(compSpec.Instances))
		for i := range compSpec.Instances {
			its.Spec.Instances[i] = workloads.InstanceTemplate{
				Name:     compSpec.Instances[i].Name,
				Replicas: compSpec.Instances[i].Replicas,
				Ordinals: compSpec.Instances[i].Ordinals,
			}
		}
	})()).Should(Succeed())

	its := &workloads.InstanceSet{}
	Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKey{Name: itsName, Namespace: cluster.Namespace}, its)).Should(Succeed())
	desiredAssignments, templateNames, err := instancetemplate.BuildAssignments(nil, its)
	Expect(err).ShouldNot(HaveOccurred())
	desired := make([]instancestatus.TemplateAssignment, 0, len(desiredAssignments))
	desiredNames := make(map[string]struct{}, len(desiredAssignments))
	templateHints := make([]instancestatus.TemplateAssignment, 0, len(desiredAssignments))
	updateRevisions := map[string]string{}
	for _, assignment := range desiredAssignments {
		desired = append(desired, instancestatus.TemplateAssignment{
			InstanceName: assignment.InstanceName,
			TemplateName: assignment.TemplateName,
		})
		desiredNames[assignment.InstanceName] = struct{}{}
		updateRevisions[assignment.InstanceName] = "revision"
	}
	offlineNames := append([]string(nil), its.Spec.OfflineInstances...)
	if its.Spec.Stop != nil && *its.Spec.Stop {
		for _, assignment := range desired {
			offlineNames = append(offlineNames, assignment.InstanceName)
		}
		templateHints = append(templateHints, desired...)
		desired = nil
	}

	podList := &corev1.PodList{}
	Expect(testCtx.Cli.List(testCtx.Ctx, podList, client.MatchingLabels{
		constant.AppInstanceLabelKey:    cluster.Name,
		constant.KBAppComponentLabelKey: compName,
	})).Should(Succeed())
	currRevisions := map[string]string{}
	observations := make([]instancestatus.Observation, 0, len(podList.Items))
	notReadyPodNames := make([]string, 0)
	for i := range podList.Items {
		pod := &podList.Items[i]
		currRevisions[pod.Name] = "revision"
		ready := pod.DeletionTimestamp.IsZero() && intctrlutil.IsPodReady(pod)
		if !ready {
			notReadyPodNames = append(notReadyPodNames, pod.Name)
		}
		if templateName, ok := instancetemplate.TemplateNameFromLabels(pod.Labels); ok {
			templateHints = append(templateHints, instancestatus.TemplateAssignment{
				InstanceName: pod.Name,
				TemplateName: templateName,
			})
		}
		if _, isDesired := desiredNames[pod.Name]; !isDesired {
			addHistoricalTemplateHint(its, pod.Name, templateNames, &templateHints)
		}
		state := workloads.InstanceCurrentStatePresent
		if !pod.DeletionTimestamp.IsZero() {
			state = workloads.InstanceCurrentStateTerminating
		}
		observations = append(observations, instancestatus.Observation{
			InstanceName: pod.Name,
			State:        state,
			Revision:     "revision",
			UpToDate:     true,
			Ready:        ready,
			Available:    ready,
			Role:         pod.Labels[constant.RoleLabelKey],
		})
	}
	for _, name := range offlineNames {
		if _, isDesired := desiredNames[name]; !isDesired {
			addHistoricalTemplateHint(its, name, templateNames, &templateHints)
		}
	}
	instanceStatus, err := instancestatus.Build(instancestatus.Input{
		Previous:           its.Status.InstanceStatus,
		DesiredAssignments: desired,
		Offline:            offlineNames,
		Observations:       observations,
		TemplateHints:      templateHints,
		UpdateRevisions:    updateRevisions,
	})
	Expect(err).ShouldNot(HaveOccurred())

	Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKey{Name: itsName, Namespace: cluster.Namespace}, func(its *workloads.InstanceSet) {
		its.Status.CurrentRevisions = currRevisions
		its.Status.UpdateRevisions = updateRevisions
		its.Status.Replicas = compSpec.Replicas
		its.Status.CurrentReplicas = int32(len(podList.Items))
		its.Status.InstanceStatus = instanceStatus
		condition := metav1.Condition{
			Type:               string(workloads.InstanceReady),
			ObservedGeneration: its.Generation,
		}
		if len(notReadyPodNames) > 0 {
			message, marshalErr := json.Marshal(notReadyPodNames)
			Expect(marshalErr).ShouldNot(HaveOccurred())
			condition.Status = metav1.ConditionFalse
			condition.Reason = workloads.ReasonNotReady
			condition.Message = string(message)
		} else {
			condition.Status = metav1.ConditionTrue
			condition.Reason = workloads.ReasonReady
		}
		meta.SetStatusCondition(&its.Status.Conditions, condition)
	})()).Should(Succeed())
}

func addHistoricalTemplateHint(its *workloads.InstanceSet, name string, templateNames []string,
	templateHints *[]instancestatus.TemplateAssignment) {
	templateName, ok, err := instancetemplate.ResolveHistoricalTemplate(its, name, templateNames)
	Expect(err).ShouldNot(HaveOccurred())
	if ok {
		*templateHints = append(*templateHints, instancestatus.TemplateAssignment{
			InstanceName: name,
			TemplateName: templateName,
		})
	}
}
