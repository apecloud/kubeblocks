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
	"context"
	"fmt"
	"reflect"

	"github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/testutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

const (
	testFinalizer = "test.kubeblocks.io/finalizer"
)

// NewFakeStatefulSet creates a fake StatefulSet workload object for testing.
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

// NewFakeStatefulSetPod creates a fake pod of the StatefulSet workload for testing.
func NewFakeStatefulSetPod(set *apps.StatefulSet, ordinal int) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name = fmt.Sprintf("%s-%d", set.Name, ordinal)
	return pod
}

// MockStatefulSetReady mocks the StatefulSet workload is ready.
func MockStatefulSetReady(sts *apps.StatefulSet) {
	sts.Status.AvailableReplicas = *sts.Spec.Replicas
	sts.Status.ObservedGeneration = sts.Generation
	sts.Status.Replicas = *sts.Spec.Replicas
	sts.Status.ReadyReplicas = *sts.Spec.Replicas
	sts.Status.CurrentRevision = sts.Status.UpdateRevision
}

// DeletePodLabelKey deletes the specified label of the pod.
func DeletePodLabelKey(ctx context.Context, testCtx testutil.TestContext, podName, labelKey string) {
	pod := &corev1.Pod{}
	gomega.Expect(testCtx.Cli.Get(ctx, client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, pod)).Should(gomega.Succeed())
	if pod.Labels == nil {
		return
	}
	patch := client.MergeFrom(pod.DeepCopy())
	delete(pod.Labels, labelKey)
	gomega.Expect(testCtx.Cli.Patch(ctx, pod, patch)).Should(gomega.Succeed())
	gomega.Eventually(func(g gomega.Gomega) {
		tmpPod := &corev1.Pod{}
		_ = testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, tmpPod)
		g.Expect(tmpPod.Labels == nil || tmpPod.Labels[labelKey] == "").Should(gomega.BeTrue())
	}).Should(gomega.Succeed())
}

// UpdatePodStatusNotReady updates the pod status to make it not ready.
func UpdatePodStatusNotReady(ctx context.Context, testCtx testutil.TestContext, podName string) {
	pod := &corev1.Pod{}
	gomega.Expect(testCtx.Cli.Get(ctx, client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, pod)).Should(gomega.Succeed())
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Status.Conditions = nil
	gomega.Expect(testCtx.Cli.Status().Patch(ctx, pod, patch)).Should(gomega.Succeed())
	gomega.Eventually(func(g gomega.Gomega) {
		tmpPod := &corev1.Pod{}
		_ = testCtx.Cli.Get(context.Background(),
			client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, tmpPod)
		g.Expect(tmpPod.Status.Conditions).Should(gomega.BeNil())
	}).Should(gomega.Succeed())
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

func ListAndCheckStatefulSet(testCtx *testutil.TestContext, key types.NamespacedName) *apps.StatefulSetList {
	stsList := &apps.StatefulSetList{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, stsList, client.MatchingLabels{
			constant.AppInstanceLabelKey: key.Name,
		}, client.InNamespace(key.Namespace))).Should(gomega.Succeed())
		g.Expect(stsList.Items).ShouldNot(gomega.BeNil())
		g.Expect(stsList.Items).ShouldNot(gomega.BeEmpty())
	}).Should(gomega.Succeed())
	return stsList
}

func ListAndCheckStatefulSetCount(testCtx *testutil.TestContext, key types.NamespacedName, cnt int) *apps.StatefulSetList {
	stsList := &apps.StatefulSetList{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, stsList, client.MatchingLabels{
			constant.AppInstanceLabelKey: key.Name,
		}, client.InNamespace(key.Namespace))).Should(gomega.Succeed())
		g.Expect(len(stsList.Items)).Should(gomega.Equal(cnt))
	}).Should(gomega.Succeed())
	return stsList
}

func ListAndCheckStatefulSetWithComponent(testCtx *testutil.TestContext, key types.NamespacedName, componentName string) *apps.StatefulSetList {
	stsList := &apps.StatefulSetList{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, stsList, client.MatchingLabels{
			constant.AppInstanceLabelKey:    key.Name,
			constant.KBAppComponentLabelKey: componentName,
		}, client.InNamespace(key.Namespace))).Should(gomega.Succeed())
		g.Expect(stsList.Items).ShouldNot(gomega.BeNil())
		g.Expect(stsList.Items).ShouldNot(gomega.BeEmpty())
	}).Should(gomega.Succeed())
	return stsList
}

func ListAndCheckPodCountWithComponent(testCtx *testutil.TestContext, key types.NamespacedName, componentName string, cnt int) *corev1.PodList {
	podList := &corev1.PodList{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, podList, client.MatchingLabels{
			constant.AppInstanceLabelKey:    key.Name,
			constant.KBAppComponentLabelKey: componentName,
		}, client.InNamespace(key.Namespace))).Should(gomega.Succeed())
		g.Expect(len(podList.Items)).Should(gomega.Equal(cnt))
	}).Should(gomega.Succeed())
	return podList
}

func PatchStatefulSetStatus(testCtx *testutil.TestContext, stsName string, status apps.StatefulSetStatus) {
	objectKey := client.ObjectKey{Name: stsName, Namespace: testCtx.DefaultNamespace}
	gomega.Expect(testapps.GetAndChangeObjStatus(testCtx, objectKey, func(newSts *apps.StatefulSet) {
		newSts.Status = status
	})()).Should(gomega.Succeed())
	gomega.Eventually(testapps.CheckObj(testCtx, objectKey, func(g gomega.Gomega, newSts *apps.StatefulSet) {
		g.Expect(reflect.DeepEqual(newSts.Status, status)).Should(gomega.BeTrue())
	})).Should(gomega.Succeed())
}

func InitStatefulSetStatus(testCtx testutil.TestContext, statefulset *apps.StatefulSet, controllerRevision string) {
	gomega.Expect(testapps.ChangeObjStatus(&testCtx, statefulset, func() {
		statefulset.Status.UpdateRevision = controllerRevision
		statefulset.Status.CurrentRevision = controllerRevision
		statefulset.Status.ObservedGeneration = statefulset.Generation
	})).Should(gomega.Succeed())
}
