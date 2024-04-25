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
	"context"
	"fmt"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/testutil"
)

const (
	testFinalizer = "test.kubeblocks.io/finalizer"
)

// NewFakePod creates a fake pod of the StatefulSet workload for testing.
func NewFakePod(parentName string, ordinal int) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name = fmt.Sprintf("%s-%d", parentName, ordinal)
	return pod
}

// MockPodAvailable mocks pod is available
func MockPodAvailable(pod *corev1.Pod, lastTransitionTime metav1.Time) {
	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:               corev1.PodReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: lastTransitionTime,
		},
	}
}

// MockPodIsTerminating mocks pod is terminating.
func MockPodIsTerminating(ctx context.Context, testCtx testutil.TestContext, pod *corev1.Pod) {
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Finalizers = []string{testFinalizer}
	gomega.Expect(testCtx.Cli.Patch(ctx, pod, patch)).Should(gomega.Succeed())
	gomega.Expect(testCtx.Cli.Delete(ctx, pod)).Should(gomega.Succeed())
	gomega.Eventually(func(g gomega.Gomega) {
		tmpPod := &corev1.Pod{}
		_ = testCtx.Cli.Get(context.Background(),
			client.ObjectKey{Name: pod.Name, Namespace: testCtx.DefaultNamespace}, tmpPod)
		g.Expect(!tmpPod.DeletionTimestamp.IsZero()).Should(gomega.BeTrue())
	}).Should(gomega.Succeed())
}

// RemovePodFinalizer removes the pod finalizer to delete the pod finally.
func RemovePodFinalizer(ctx context.Context, testCtx testutil.TestContext, pod *corev1.Pod) {
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Finalizers = []string{}
	gomega.Expect(testCtx.Cli.Patch(ctx, pod, patch)).Should(gomega.Succeed())
	gomega.Eventually(func() error {
		return testCtx.Cli.Get(context.Background(),
			client.ObjectKey{Name: pod.Name, Namespace: testCtx.DefaultNamespace}, &corev1.Pod{})
	}).Should(gomega.Satisfy(apierrors.IsNotFound))
}
