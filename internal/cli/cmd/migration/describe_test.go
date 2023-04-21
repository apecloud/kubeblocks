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

package migration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("describe", func() {

	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	It("command build", func() {
		cmd := NewMigrationDescribeCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("func test", func() {
		sts := appv1.StatefulSet{
			Status: appv1.StatefulSetStatus{
				Replicas: 1,
			},
		}
		pod := corev1.Pod{}

		sts.Status.AvailableReplicas = 0
		pod.Status.Phase = corev1.PodFailed
		Expect(getCdcStatus(&sts, &pod)).Should(Equal(corev1.PodFailed))

		sts.Status.AvailableReplicas = 1
		pod.Status.Phase = corev1.PodPending
		Expect(getCdcStatus(&sts, &pod)).Should(Equal(corev1.PodPending))

		sts.Status.AvailableReplicas = 1
		pod.Status.Phase = corev1.PodRunning
		Expect(getCdcStatus(&sts, &pod)).Should(Equal(corev1.PodRunning))

		sts.Status.AvailableReplicas = 0
		t1, _ := time.ParseDuration("-30m")
		sts.CreationTimestamp = v1.NewTime(time.Now().Add(t1))
		pod.Status.Phase = corev1.PodRunning
		Expect(getCdcStatus(&sts, &pod)).Should(Equal(corev1.PodFailed))

		sts.Status.AvailableReplicas = 0
		sts.CreationTimestamp = v1.NewTime(time.Now())
		pod.Status.Phase = corev1.PodRunning
		Expect(getCdcStatus(&sts, &pod)).Should(Equal(corev1.PodPending))
	})

})
