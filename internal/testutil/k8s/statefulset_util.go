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

package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/testutil"
)

var (
	ctx           = context.Background()
	timeout       = 10 * time.Second
	interval      = time.Second
	testFinalizer = "test.kubeblocks.io/finalizer"
)

func NewFakeStatefulSet(name string, replicas int) *apps.StatefulSet {
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
	statefulSetReplicas := int32(replicas)
	Revision := name + "-d5df5b8d6"
	return &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: apps.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			},
			Replicas:    &statefulSetReplicas,
			Template:    template,
			ServiceName: "governingsvc",
		},
		Status: apps.StatefulSetStatus{
			AvailableReplicas:  statefulSetReplicas,
			ObservedGeneration: 0,
			ReadyReplicas:      statefulSetReplicas,
			UpdatedReplicas:    statefulSetReplicas,
			CurrentRevision:    Revision,
			UpdateRevision:     Revision,
		},
	}
}

func NewFakeStatefulSetPod(set *apps.StatefulSet, ordinal int) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name = fmt.Sprintf("%s-%d", set.Name, ordinal)
	return pod
}

func MockStatefulSetReady(sts *apps.StatefulSet) {
	sts.Status.AvailableReplicas = *sts.Spec.Replicas
	sts.Status.ObservedGeneration = sts.Generation
	sts.Status.Replicas = *sts.Spec.Replicas
	sts.Status.ReadyReplicas = *sts.Spec.Replicas
	sts.Status.CurrentRevision = sts.Status.UpdateRevision
}

func DeletePodLabelKey(testCtx testutil.TestContext, podName, labelKey string) {
	pod := &corev1.Pod{}
	gomega.Expect(testCtx.Cli.Get(ctx, client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, pod)).Should(gomega.Succeed())
	if pod.Labels == nil {
		return
	}
	patch := client.MergeFrom(pod.DeepCopy())
	delete(pod.Labels, labelKey)
	gomega.Expect(testCtx.Cli.Patch(ctx, pod, patch)).Should(gomega.Succeed())
	gomega.Eventually(func() bool {
		tmpPod := &corev1.Pod{}
		_ = testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, tmpPod)
		return tmpPod.Labels == nil || tmpPod.Labels[labelKey] == ""
	}, timeout, interval).Should(gomega.BeTrue())
}

func UpdatePodStatusNotReady(testCtx testutil.TestContext, podName string) {
	pod := &corev1.Pod{}
	gomega.Expect(testCtx.Cli.Get(ctx, client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, pod)).Should(gomega.Succeed())
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Status.Conditions = nil
	gomega.Expect(testCtx.Cli.Status().Patch(ctx, pod, patch)).Should(gomega.Succeed())
	gomega.Eventually(func() bool {
		tmpPod := &corev1.Pod{}
		_ = testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, tmpPod)
		return tmpPod.Status.Conditions == nil
	}, timeout, interval).Should(gomega.BeTrue())
}

// MockPodIsTerminating mock pod is terminating.
func MockPodIsTerminating(testCtx testutil.TestContext, pod *corev1.Pod) {
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Finalizers = []string{testFinalizer}
	gomega.Expect(testCtx.Cli.Patch(ctx, pod, patch)).Should(gomega.Succeed())
	gomega.Expect(testCtx.Cli.Delete(ctx, pod)).Should(gomega.Succeed())
	gomega.Eventually(func() bool {
		tmpPod := &corev1.Pod{}
		_ = testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: pod.Name, Namespace: testCtx.DefaultNamespace}, tmpPod)
		return !tmpPod.DeletionTimestamp.IsZero()
	}, timeout, interval).Should(gomega.BeTrue())
}

// RemovePodFinalizer remove the pod finalizer to delete the pod finally.
func RemovePodFinalizer(testCtx testutil.TestContext, pod *corev1.Pod) {
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Finalizers = []string{}
	gomega.Expect(testCtx.Cli.Patch(ctx, pod, patch)).Should(gomega.Succeed())
	gomega.Eventually(func() error {
		return testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: pod.Name, Namespace: testCtx.DefaultNamespace}, &corev1.Pod{})
	}, timeout, interval).Should(gomega.Satisfy(apierrors.IsNotFound))
}
