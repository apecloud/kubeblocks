/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
