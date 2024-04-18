/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

// NewFakeInstanceSet creates a fake ITS workload object for testing.
func NewFakeInstanceSet(name string, replicas int) *workloads.InstanceSet {
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
	itsReplicas := int32(replicas)
	Revision := name + "-d5df5b8d6"
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: workloads.InstanceSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			},
			Replicas:    &itsReplicas,
			Template:    template,
			ServiceName: "governingsvc",
		},
		Status: workloads.InstanceSetStatus{
			InitReplicas: itsReplicas,
			StatefulSetStatus: appsv1.StatefulSetStatus{
				AvailableReplicas:  itsReplicas,
				ObservedGeneration: 0,
				ReadyReplicas:      itsReplicas,
				UpdatedReplicas:    itsReplicas,
				CurrentRevision:    Revision,
				UpdateRevision:     Revision,
			},
		},
	}
}

// NewFakeInstanceSetPod creates a fake pod of the ITS workload for testing.
func NewFakeInstanceSetPod(its *workloads.InstanceSet, ordinal int) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name = fmt.Sprintf("%s-%d", its.Name, ordinal)
	return pod
}

// MockInstanceSetReady mocks the ITS workload to ready state.
func MockInstanceSetReady(its *workloads.InstanceSet, pods ...*corev1.Pod) {
	its.Status.InitReplicas = *its.Spec.Replicas
	its.Status.ReadyInitReplicas = *its.Spec.Replicas
	its.Status.AvailableReplicas = *its.Spec.Replicas
	its.Status.ObservedGeneration = its.Generation
	its.Status.CurrentGeneration = its.Generation
	its.Status.Replicas = *its.Spec.Replicas
	its.Status.ReadyReplicas = *its.Spec.Replicas
	its.Status.CurrentRevision = its.Status.UpdateRevision
	its.Status.UpdatedReplicas = its.Status.Replicas

	composeRoleMap := func(its workloads.InstanceSet) map[string]workloads.ReplicaRole {
		roleMap := make(map[string]workloads.ReplicaRole, 0)
		for _, role := range its.Spec.Roles {
			roleMap[strings.ToLower(role.Name)] = role
		}
		return roleMap
	}
	var membersStatus []workloads.MemberStatus
	roleMap := composeRoleMap(*its)
	for _, pod := range pods {
		roleName := strings.ToLower(pod.Labels[constant.RoleLabelKey])
		role, ok := roleMap[roleName]
		if !ok {
			continue
		}
		memberStatus := workloads.MemberStatus{
			PodName:     pod.Name,
			ReplicaRole: &role,
		}
		membersStatus = append(membersStatus, memberStatus)
	}
	its.Status.MembersStatus = membersStatus
}

func ListAndCheckInstanceSet(testCtx *testutil.TestContext, key types.NamespacedName) *workloads.InstanceSetList {
	itsList := &workloads.InstanceSetList{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, itsList, client.MatchingLabels{
			constant.AppInstanceLabelKey: key.Name,
		}, client.InNamespace(key.Namespace))).Should(gomega.Succeed())
		g.Expect(itsList.Items).ShouldNot(gomega.BeNil())
		g.Expect(itsList.Items).ShouldNot(gomega.BeEmpty())
	}).Should(gomega.Succeed())
	return itsList
}

func ListAndCheckInstanceSetItemsCount(testCtx *testutil.TestContext, key types.NamespacedName, cnt int) *workloads.InstanceSetList {
	itsList := &workloads.InstanceSetList{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, itsList, client.MatchingLabels{
			constant.AppInstanceLabelKey: key.Name,
		}, client.InNamespace(key.Namespace))).Should(gomega.Succeed())
		g.Expect(len(itsList.Items)).Should(gomega.Equal(cnt))
	}).Should(gomega.Succeed())
	return itsList
}

func ListAndCheckInstanceSetWithComponent(testCtx *testutil.TestContext, key types.NamespacedName, componentName string) *workloads.InstanceSetList {
	itsList := &workloads.InstanceSetList{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, itsList, client.MatchingLabels{
			constant.AppInstanceLabelKey:    key.Name,
			constant.KBAppComponentLabelKey: componentName,
		}, client.InNamespace(key.Namespace))).Should(gomega.Succeed())
		g.Expect(itsList.Items).ShouldNot(gomega.BeNil())
		g.Expect(itsList.Items).ShouldNot(gomega.BeEmpty())
	}).Should(gomega.Succeed())
	return itsList
}

func PatchInstanceSetStatus(testCtx *testutil.TestContext, stsName string, status workloads.InstanceSetStatus) {
	objectKey := client.ObjectKey{Name: stsName, Namespace: testCtx.DefaultNamespace}
	gomega.Expect(testapps.GetAndChangeObjStatus(testCtx, objectKey, func(newITS *workloads.InstanceSet) {
		newITS.Status = status
	})()).Should(gomega.Succeed())
	gomega.Eventually(testapps.CheckObj(testCtx, objectKey, func(g gomega.Gomega, newITS *workloads.InstanceSet) {
		g.Expect(reflect.DeepEqual(newITS.Status, status)).Should(gomega.BeTrue())
	})).Should(gomega.Succeed())
}

func InitInstanceSetStatus(testCtx testutil.TestContext, its *workloads.InstanceSet, controllerRevision string) {
	gomega.Expect(testapps.ChangeObjStatus(&testCtx, its, func() {
		its.Status.InitReplicas = *its.Spec.Replicas
		its.Status.Replicas = *its.Spec.Replicas
		its.Status.UpdateRevision = controllerRevision
		its.Status.CurrentRevision = controllerRevision
		its.Status.ObservedGeneration = its.Generation
		its.Status.CurrentGeneration = its.Generation
	})).Should(gomega.Succeed())
}
