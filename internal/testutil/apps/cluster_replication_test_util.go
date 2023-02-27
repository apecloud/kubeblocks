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
	"fmt"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// MockReplicationComponentStsPod mocks to create pod of the replication StatefulSet, just using in envTest
func MockReplicationComponentStsPod(
	testCtx testutil.TestContext,
	sts *appsv1.StatefulSet,
	clusterName,
	compName,
	podName,
	roleName string) *corev1.Pod {
	pod := NewPodFactory(testCtx.DefaultNamespace, podName).
		SetOwnerReferences("apps/v1", intctrlutil.StatefulSetKind, sts).
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
	gomega.Expect(testCtx.Cli.Status().Patch(context.Background(), pod, patch)).Should(gomega.Succeed())
	return pod
}

// MockReplicationComponentPods mocks to create pods of the component, just using in envTest
func MockReplicationComponentPods(
	testCtx testutil.TestContext,
	sts *appsv1.StatefulSet,
	clusterName,
	compName string,
	podRole string) []*corev1.Pod {
	var pods []*corev1.Pod
	podName := fmt.Sprintf("%s-0", sts.Name)
	pods = append(pods, MockReplicationComponentStsPod(testCtx, sts, clusterName, compName, podName, podRole))
	return pods
}
