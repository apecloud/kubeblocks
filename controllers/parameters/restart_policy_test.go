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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("Reconfigure restartPolicy", func() {

	var (
		k8sMockClient *testutil.K8sClientMockHelper
		simplePolicy  = upgradePolicyMap[parametersv1alpha1.RestartPolicy]
	)

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	updatePodCfgVersion := func(pod *corev1.Pod, configKey, configVersion string) {
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)] = configVersion
	}

	Context("simple reconfigure policy test", func() {
		It("Should success without error", func() {
			Expect(simplePolicy.GetPolicyName()).Should(BeEquivalentTo("restart"))

			mockParam := newMockReconfigureParams("restartPolicy", k8sMockClient.Client(),
				withMockInstanceSet(2, nil),
				withConfigSpec("for_test", map[string]string{
					"key": "value",
				}),
				withClusterComponent(2))

			// mock client update caller
			updateErr := core.MakeError("update failed!")
			k8sMockClient.MockPatchMethod(
				testutil.WithFailed(updateErr, testutil.WithTimes(1)),
				testutil.WithSucceed(testutil.WithAnyTimes()))
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListSequenceResult([][]runtime.Object{
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 2)),
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 2, withReadyPod(0, 2), func(pod *corev1.Pod, index int) {
						// mock pod-1 restart
						if index == 1 {
							updatePodCfgVersion(pod, mockParam.generateConfigIdentifier(), mockParam.getTargetVersionHash())
						}
					})),
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 2, withReadyPod(0, 2), func(pod *corev1.Pod, index int) {
						// mock all pod restart
						updatePodCfgVersion(pod, mockParam.generateConfigIdentifier(), mockParam.getTargetVersionHash())
					})),
				}),
				testutil.WithTimes(3),
			))

			status, err := simplePolicy.Upgrade(mockParam)
			Expect(err).Should(BeEquivalentTo(updateErr))
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))

			// first upgrade, not pod is ready
			status, err = simplePolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(0)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))

			// only one pod ready
			status, err = simplePolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(1)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))

			// succeed update pod
			status, err = simplePolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(2)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))
		})
	})

	Context("simple reconfigure policy test with Replication", func() {
		It("Should success", func() {
			mockParam := newMockReconfigureParams("restartPolicy", k8sMockClient.Client(),
				withMockInstanceSet(2, nil),
				withConfigSpec("for_test", map[string]string{
					"key": "value",
				}),
				withClusterComponent(2))

			k8sMockClient.MockPatchMethod(testutil.WithSucceed(testutil.WithAnyTimes()))
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListSequenceResult([][]runtime.Object{
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 2)),
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 2,
						withReadyPod(0, 2), func(pod *corev1.Pod, _ int) {
							updatePodCfgVersion(pod, mockParam.generateConfigIdentifier(), mockParam.getTargetVersionHash())
						})),
				}),
				testutil.WithAnyTimes(),
			))

			status, err := simplePolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(0)))

			status, err = simplePolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(2)))
		})
	})

	// TODO(component)
	Context("simple reconfigure policy test for not supported component", func() {
		It("Should failed", func() {
			// not support type
			mockParam := newMockReconfigureParams("restartPolicy", k8sMockClient.Client(),
				withMockInstanceSet(2, nil),
				withConfigSpec("for_test", map[string]string{
					"key": "value",
				}),
				withClusterComponent(2))

			updateErr := core.MakeError("update failed!")
			k8sMockClient.MockPatchMethod(
				testutil.WithFailed(updateErr, testutil.WithTimes(1)),
				testutil.WithSucceed(testutil.WithAnyTimes()))
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListSequenceResult([][]runtime.Object{
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 2)),
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 2, withReadyPod(0, 2), func(pod *corev1.Pod, index int) {
						// mock pod-1 restart
						if index == 1 {
							updatePodCfgVersion(pod, mockParam.generateConfigIdentifier(), mockParam.getTargetVersionHash())
						}
					})),
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 2, withReadyPod(0, 2), func(pod *corev1.Pod, index int) {
						// mock all pod restart
						updatePodCfgVersion(pod, mockParam.generateConfigIdentifier(), mockParam.getTargetVersionHash())
					})),
				}),
				testutil.WithTimes(3),
			))

			status, err := simplePolicy.Upgrade(mockParam)
			Expect(err).Should(BeEquivalentTo(updateErr))
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))

			// first upgrade, not pod is ready
			status, err = simplePolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(0)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))

			// only one pod ready
			status, err = simplePolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(1)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))

			// succeed update pod
			status, err = simplePolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(2)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))

		})
	})

	// Context("simple reconfigure policy test without not configmap volume", func() {
	//	It("Should failed", func() {
	//		// mock not cc
	//		mockParam := newMockReconfigureParams("restartPolicy", nil,
	//			withMockInstanceSet(2, nil),
	//			withConfigSpec("not_tpl_name", map[string]string{
	//				"key": "value",
	//			}),
	//			withCDComponent(appsv1alpha1.Consensus, []appsv1alpha1.ComponentConfigSpec{{
	//				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
	//					Name:       "for_test",
	//					VolumeName: "test_volume",
	//				}}}))
	//		status, err := restartPolicy.Upgrade(mockParam)
	//		Expect(err).ShouldNot(Succeed())
	//		Expect(err.Error()).Should(ContainSubstring("failed to find config meta"))
	//		Expect(status.Status).Should(BeEquivalentTo(ESFailed))
	//	})
	// })
})
