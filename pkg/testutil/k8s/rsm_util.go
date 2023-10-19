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

package testutil

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/testutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

// NewFakeRSM creates a fake RSM workload object for testing.
func NewFakeRSM(name string, replicas int) *workloads.ReplicatedStateMachine {
	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx",
				},
			},
		},
	}

	template.Labels = map[string]string{"foo": "bar"}
	rsmReplicas := int32(replicas)
	Revision := name + "-d5df5b8d6"
	return &workloads.ReplicatedStateMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: workloads.ReplicatedStateMachineSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			},
			Replicas:    &rsmReplicas,
			Template:    template,
			ServiceName: "governingsvc",
		},
		Status: workloads.ReplicatedStateMachineStatus{
			InitReplicas: rsmReplicas,
			StatefulSetStatus: appsv1.StatefulSetStatus{
				AvailableReplicas:  rsmReplicas,
				ObservedGeneration: 0,
				ReadyReplicas:      rsmReplicas,
				UpdatedReplicas:    rsmReplicas,
				CurrentRevision:    Revision,
				UpdateRevision:     Revision,
			},
		},
	}
}

// NewFakeRSMPod creates a fake pod of the RSM workload for testing.
func NewFakeRSMPod(rsm *workloads.ReplicatedStateMachine, ordinal int) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name = fmt.Sprintf("%s-%d", rsm.Name, ordinal)
	return pod
}

// MockRSMReady mocks the RSM workload to ready state.
func MockRSMReady(rsm *workloads.ReplicatedStateMachine, pods ...*corev1.Pod) {
	rsm.Status.InitReplicas = *rsm.Spec.Replicas
	rsm.Status.ReadyInitReplicas = *rsm.Spec.Replicas
	rsm.Status.AvailableReplicas = *rsm.Spec.Replicas
	rsm.Status.ObservedGeneration = rsm.Generation
	rsm.Status.CurrentGeneration = rsm.Generation
	rsm.Status.Replicas = *rsm.Spec.Replicas
	rsm.Status.ReadyReplicas = *rsm.Spec.Replicas
	rsm.Status.CurrentRevision = rsm.Status.UpdateRevision
	rsm.Status.UpdatedReplicas = rsm.Status.Replicas

	composeRoleMap := func(rsm workloads.ReplicatedStateMachine) map[string]workloads.ReplicaRole {
		roleMap := make(map[string]workloads.ReplicaRole, 0)
		for _, role := range rsm.Spec.Roles {
			roleMap[strings.ToLower(role.Name)] = role
		}
		return roleMap
	}
	var membersStatus []workloads.MemberStatus
	roleMap := composeRoleMap(*rsm)
	for _, pod := range pods {
		roleName := strings.ToLower(pod.Labels[constant.RoleLabelKey])
		role, ok := roleMap[roleName]
		if !ok {
			continue
		}
		memberStatus := workloads.MemberStatus{
			PodName:     pod.Name,
			ReplicaRole: role,
		}
		membersStatus = append(membersStatus, memberStatus)
	}
	rsm.Status.MembersStatus = membersStatus
}

func ListAndCheckRSM(testCtx *testutil.TestContext, key types.NamespacedName) *workloads.ReplicatedStateMachineList {
	rsmList := &workloads.ReplicatedStateMachineList{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, rsmList, client.MatchingLabels{
			constant.AppInstanceLabelKey: key.Name,
		}, client.InNamespace(key.Namespace))).Should(gomega.Succeed())
		g.Expect(rsmList.Items).ShouldNot(gomega.BeNil())
		g.Expect(rsmList.Items).ShouldNot(gomega.BeEmpty())
	}).Should(gomega.Succeed())
	return rsmList
}

func ListAndCheckRSMItemsCount(testCtx *testutil.TestContext, key types.NamespacedName, cnt int) *workloads.ReplicatedStateMachineList {
	rsmList := &workloads.ReplicatedStateMachineList{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, rsmList, client.MatchingLabels{
			constant.AppInstanceLabelKey: key.Name,
		}, client.InNamespace(key.Namespace))).Should(gomega.Succeed())
		g.Expect(len(rsmList.Items)).Should(gomega.Equal(cnt))
	}).Should(gomega.Succeed())
	return rsmList
}

func ListAndCheckRSMWithComponent(testCtx *testutil.TestContext, key types.NamespacedName, componentName string) *workloads.ReplicatedStateMachineList {
	rsmList := &workloads.ReplicatedStateMachineList{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, rsmList, client.MatchingLabels{
			constant.AppInstanceLabelKey:    key.Name,
			constant.KBAppComponentLabelKey: componentName,
		}, client.InNamespace(key.Namespace))).Should(gomega.Succeed())
		g.Expect(rsmList.Items).ShouldNot(gomega.BeNil())
		g.Expect(rsmList.Items).ShouldNot(gomega.BeEmpty())
	}).Should(gomega.Succeed())
	return rsmList
}

func PatchRSMStatus(testCtx *testutil.TestContext, stsName string, status workloads.ReplicatedStateMachineStatus) {
	objectKey := client.ObjectKey{Name: stsName, Namespace: testCtx.DefaultNamespace}
	gomega.Expect(testapps.GetAndChangeObjStatus(testCtx, objectKey, func(newRSM *workloads.ReplicatedStateMachine) {
		newRSM.Status = status
	})()).Should(gomega.Succeed())
	gomega.Eventually(testapps.CheckObj(testCtx, objectKey, func(g gomega.Gomega, newRSM *workloads.ReplicatedStateMachine) {
		g.Expect(reflect.DeepEqual(newRSM.Status, status)).Should(gomega.BeTrue())
	})).Should(gomega.Succeed())
}

func InitRSMStatus(testCtx testutil.TestContext, rsm *workloads.ReplicatedStateMachine, controllerRevision string) {
	gomega.Expect(testapps.ChangeObjStatus(&testCtx, rsm, func() {
		rsm.Status.InitReplicas = *rsm.Spec.Replicas
		rsm.Status.Replicas = *rsm.Spec.Replicas
		rsm.Status.UpdateRevision = controllerRevision
		rsm.Status.CurrentRevision = controllerRevision
		rsm.Status.ObservedGeneration = rsm.Generation
		rsm.Status.CurrentGeneration = rsm.Generation
	})).Should(gomega.Succeed())
}
