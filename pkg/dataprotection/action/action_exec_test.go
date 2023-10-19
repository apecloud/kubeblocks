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

package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/action"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("ExecAction Test", func() {
	const (
		actionName         = "test-exec-action"
		podName            = "pod"
		container          = "container"
		serviceAccountName = "service-account"
	)

	var (
		command = []string{"ls"}
	)

	cleanEnv := func() {
		By("clean resources")
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS)
	}

	BeforeEach(func() {
		cleanEnv()
		viper.Set(constant.KBToolsImage, testdp.KBToolImage)
	})

	AfterEach(func() {
		cleanEnv()
		viper.Set(constant.KBToolsImage, "")
	})

	Context("create exec action", func() {
		It("should return error when pod name is empty", func() {
			act := &action.ExecAction{}
			status, err := act.Execute(buildActionCtx())
			Expect(err).To(HaveOccurred())
			Expect(status).Should(BeNil())
		})

		It("should build pod spec but job action validate failed", func() {
			act := &action.ExecAction{
				JobAction: action.JobAction{
					Name: actionName,
				},
				PodName:   podName,
				Namespace: testCtx.DefaultNamespace,
				Command:   command,
			}
			status, err := act.Execute(buildActionCtx())
			Expect(err).To(HaveOccurred())
			Expect(status).ShouldNot(BeNil())
			Expect(status.Phase).Should(Equal(dpv1alpha1.ActionPhaseFailed))
			Expect(act.JobAction.PodSpec).ShouldNot(BeNil())
		})

		It("should success to build exec action", func() {
			labels := map[string]string{
				"dp-test-action": actionName,
			}

			act := &action.ExecAction{
				JobAction: action.JobAction{
					Name: actionName,
					ObjectMeta: metav1.ObjectMeta{
						Name:      actionName,
						Namespace: testCtx.DefaultNamespace,
						Labels:    labels,
					},
					Owner: testdp.NewFakeBackup(&testCtx, nil),
				},
				PodName:            podName,
				Namespace:          testCtx.DefaultNamespace,
				Command:            command,
				Container:          container,
				ServiceAccountName: serviceAccountName,
			}

			By("should success to execute")
			status, err := act.Execute(buildActionCtx())
			Expect(err).Should(Succeed())
			Expect(status).ShouldNot(BeNil())
			Expect(status.Phase).Should(Equal(dpv1alpha1.ActionPhaseRunning))

			By("check the job was created")
			job := &batchv1.Job{}
			key := client.ObjectKey{Name: actionName, Namespace: testCtx.DefaultNamespace}
			Eventually(testapps.CheckObjExists(&testCtx, key, job, true)).Should(Succeed())
		})
	})
})
