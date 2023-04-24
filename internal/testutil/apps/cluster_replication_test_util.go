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

package apps

import (
	"context"
	"fmt"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// MockReplicationComponentStsPod mocks to create pod of the replication StatefulSet, just using in envTest
func MockReplicationComponentStsPod(
	g gomega.Gomega,
	testCtx testutil.TestContext,
	sts *appsv1.StatefulSet,
	clusterName,
	compName,
	podName,
	roleName string) *corev1.Pod {
	pod := NewPodFactory(testCtx.DefaultNamespace, podName).
		SetOwnerReferences("apps/v1", constant.StatefulSetKind, sts).
		AddAppInstanceLabel(clusterName).
		AddAppComponentLabel(compName).
		AddAppManangedByLabel().
		AddRoleLabel(roleName).
		AddControllerRevisionHashLabel(sts.Status.UpdateRevision).
		AddContainer(corev1.Container{Name: DefaultRedisContainerName, Image: DefaultRedisImageName}).
		Create(&testCtx).GetObject()
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		},
	}
	if g != nil {
		g.Expect(testCtx.Cli.Status().Patch(context.Background(), pod, patch)).Should(gomega.Succeed())
	} else {
		gomega.Expect(testCtx.Cli.Status().Patch(context.Background(), pod, patch)).Should(gomega.Succeed())
	}
	return pod
}

// MockReplicationComponentPods mocks to create pods of the component, just using in envTest. If roleByIdx is empty,
// will have implicit pod-0 being "primary" role and others to "secondary" role.
func MockReplicationComponentPods(
	g gomega.Gomega,
	testCtx testutil.TestContext,
	sts *appsv1.StatefulSet,
	clusterName,
	compName string,
	roleByIdx map[int32]string) []*corev1.Pod {

	var pods []*corev1.Pod
	for i := int32(0); i < *sts.Spec.Replicas; i++ {
		podName := fmt.Sprintf("%s-%d", sts.Name, i)
		role := "secondary"
		if podRole, ok := roleByIdx[i]; ok && podRole != "" {
			role = podRole
		} else if i == 0 {
			role = "primary"
		}
		pods = append(pods, MockReplicationComponentStsPod(g, testCtx, sts, clusterName, compName, podName, role))
	}
	return pods
}

// UpdateClusterCompSpecPrimaryIndex updates cluster component spec primaryIndex.
func UpdateClusterCompSpecPrimaryIndex(testCtx *testutil.TestContext,
	cluster *appsv1alpha1.Cluster,
	compName string,
	primaryIndex *int32) {
	objectKey := client.ObjectKey{Name: cluster.Name, Namespace: testCtx.DefaultNamespace}
	gomega.Expect(GetAndChangeObj(testCtx, objectKey, func(newCluster *appsv1alpha1.Cluster) {
		var index int
		comps := newCluster.Spec.ComponentSpecs
		if len(comps) > 0 {
			for i, compSpec := range newCluster.Spec.ComponentSpecs {
				if compSpec.Name == compName {
					index = i
				}
			}
			comps[index].PrimaryIndex = primaryIndex
		}
		newCluster.Spec.ComponentSpecs = comps
	})()).Should(gomega.Succeed())
	gomega.Eventually(CheckObj(testCtx, objectKey, func(g gomega.Gomega, newCluster *appsv1alpha1.Cluster) {
		for index, compSpec := range newCluster.Spec.ComponentSpecs {
			if compSpec.Name == compName {
				g.Expect(newCluster.Spec.ComponentSpecs[index].PrimaryIndex).Should(gomega.Equal(primaryIndex))
			}
		}
	})).Should(gomega.Succeed())
}
