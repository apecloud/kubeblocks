/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
	mockproto "github.com/apecloud/kubeblocks/pkg/configuration/proto/mocks"
	"github.com/apecloud/kubeblocks/pkg/constant"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("Reconfigure RollingPolicy", func() {

	var (
		k8sMockClient     *testutil.K8sClientMockHelper
		mockParam         reconfigureContext
		reconfigureClient *mockproto.MockReconfigureClient

		defaultReplica = 3
		rollingPolicy  = upgradePolicyMap[parametersv1alpha1.RollingPolicy]
	)

	updateLabelPatch := func(pods []corev1.Pod, patch *corev1.Pod) {
		patchKey := client.ObjectKeyFromObject(patch)
		for i := range pods {
			orgPod := &pods[i]
			if client.ObjectKeyFromObject(orgPod) == patchKey {
				orgPod.Labels = patch.Labels
				break
			}
		}
	}

	createReconfigureParam := func(replicas int) reconfigureContext {
		return newMockReconfigureParams("rollingPolicy", k8sMockClient.Client(),
			withMockInstanceSet(replicas, nil),
			withConfigSpec("for_test", map[string]string{
				"key": "value",
			}),
			withConfigDescription(&parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Properties}),
			withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
				return reconfigureClient, nil
			}),
			withClusterComponent(replicas),
			// withCDComponent(compType, []appsv1alpha1.ComponentConfigSpec{{
			// 	ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
			// 		Name:       "for_test",
			// 		VolumeName: "test_volume",
			// 	}}})
		)
	}

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
		reconfigureClient = mockproto.NewMockReconfigureClient(k8sMockClient.Controller())
		mockParam = createReconfigureParam(defaultReplica)
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		k8sMockClient.Finish()
	})

	// TODO(component)
	Context("consensus rolling reconfigure policy test", func() {
		It("Should success without error", func() {
			Expect(rollingPolicy.GetPolicyName()).Should(BeEquivalentTo("rolling"))

			mockLeaderLabel := func(pod *corev1.Pod, i int) {
				if pod.Labels == nil {
					pod.Labels = make(map[string]string, 1)
				}
				if i == 1 {
					pod.Labels[constant.RoleLabelKey] = "leader"
				} else {
					pod.Labels[constant.RoleLabelKey] = "follower"
				}
			}

			acc := 0
			mockPods := [][]corev1.Pod{
				newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 2),
				newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 5,
					mockLeaderLabel),
				newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 3,
					withReadyPod(0, 0),
					withAvailablePod(0, 3),
					mockLeaderLabel),
			}

			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListSequenceResult([][]runtime.Object{
					fromPodObjectList(mockPods[0]),
					fromPodObjectList(mockPods[1]),
					fromPodObjectList(mockPods[2]),
				}, func(sequence int, r []runtime.Object) { acc = sequence }), testutil.WithAnyTimes()))

			k8sMockClient.MockPatchMethod(testutil.WithPatchReturned(func(obj client.Object, patch client.Patch) error {
				pod, _ := obj.(*corev1.Pod)
				// mock patch
				updateLabelPatch(mockPods[acc], pod)
				return nil
			}, testutil.WithAnyTimes()))

			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).
				Return(&cfgproto.StopContainerResponse{}, nil).
				AnyTimes()

			// mock wait the number of pods to target replicas
			status, err := rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))

			// mock wait the number of pods to ready status
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))

			// upgrade pod-0
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(mockPods[acc][2].Labels[mockParam.getConfigKey()]).Should(BeEquivalentTo(mockParam.getTargetVersionHash()))
			Expect(mockPods[acc][1].Labels[mockParam.getConfigKey()]).ShouldNot(BeEquivalentTo(mockParam.getTargetVersionHash()))
			Expect(mockPods[acc][0].Labels[mockParam.getConfigKey()]).ShouldNot(BeEquivalentTo(mockParam.getTargetVersionHash()))

			// upgrade pod-2
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(mockPods[acc][0].Labels[mockParam.getConfigKey()]).Should(BeEquivalentTo(mockParam.getTargetVersionHash()))
			Expect(mockPods[acc][1].Labels[mockParam.getConfigKey()]).ShouldNot(BeEquivalentTo(mockParam.getTargetVersionHash()))

			// upgrade pod-1
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(mockPods[acc][1].Labels[mockParam.getConfigKey()]).Should(BeEquivalentTo(mockParam.getTargetVersionHash()))

			// finish check, not upgrade
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
		})
	})
})
