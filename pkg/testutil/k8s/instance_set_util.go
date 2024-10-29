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
	"strings"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/testutil"
)

// MockInstanceSetReady mocks the ITS workload to ready state.
func MockInstanceSetReady(its *workloads.InstanceSet, pods ...*corev1.Pod) {
	its.Status.InitReplicas = *its.Spec.Replicas
	its.Status.ReadyInitReplicas = *its.Spec.Replicas
	its.Status.AvailableReplicas = *its.Spec.Replicas
	its.Status.ObservedGeneration = its.Generation
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
