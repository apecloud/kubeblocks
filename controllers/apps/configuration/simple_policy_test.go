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

package configuration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Reconfigure simplePolicy", func() {

	var (
		k8sMockClient *testutil.K8sClientMockHelper
		simplePolicy  = upgradePolicyMap[appsv1alpha1.NormalPolicy]
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
		pod.Annotations[cfgcore.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)] = configVersion
	}

	Context("simple reconfigure policy test", func() {
		It("Should success without error", func() {
			Expect(simplePolicy.GetPolicyName()).Should(BeEquivalentTo("simple"))

			mockParam := newMockReconfigureParams("simplePolicy", k8sMockClient.Client(),
				withMockStatefulSet(2, nil),
				withConfigTpl("for_test", map[string]string{
					"key": "value",
				}),
				withCDComponent(appsv1alpha1.Consensus, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					}}}))

			// mock client update caller
			updateErr := cfgcore.MakeError("update failed!")
			k8sMockClient.MockUpdateMethod(
				testutil.WithFailed(updateErr, testutil.WithTimes(1)),
				testutil.WithSucceed(testutil.WithAnyTimes()))
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListSequenceResult([][]runtime.Object{
					fromPodObjectList(newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 2)),
					fromPodObjectList(newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 2, withReadyPod(0, 2), func(pod *corev1.Pod, index int) {
						// mock pod-1 restart
						if index == 1 {
							updatePodCfgVersion(pod, mockParam.getConfigKey(), mockParam.getTargetVersionHash())
						}
					})),
					fromPodObjectList(newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 2, withReadyPod(0, 2), func(pod *corev1.Pod, index int) {
						// mock all pod restart
						updatePodCfgVersion(pod, mockParam.getConfigKey(), mockParam.getTargetVersionHash())
					})),
				}),
				testutil.WithTimes(3),
			))

			status, err := simplePolicy.Upgrade(mockParam)
			Expect(err).Should(BeEquivalentTo(updateErr))
			Expect(status.Status).Should(BeEquivalentTo(ESAndRetryFailed))

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
			mockParam := newMockReconfigureParams("simplePolicy", k8sMockClient.Client(),
				withMockStatefulSet(2, nil),
				withConfigTpl("for_test", map[string]string{
					"key": "value",
				}),
				withCDComponent(appsv1alpha1.Replication, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					}}}),
			)

			k8sMockClient.MockUpdateMethod(testutil.WithSucceed(testutil.WithAnyTimes()))
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListSequenceResult([][]runtime.Object{
					fromPodObjectList(newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 2)),
					fromPodObjectList(newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 2,
						withReadyPod(0, 2), func(pod *corev1.Pod, _ int) {
							updatePodCfgVersion(pod, mockParam.getConfigKey(), mockParam.getTargetVersionHash())
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

	Context("simple reconfigure policy test without not support component", func() {
		It("Should failed", func() {
			// not support type
			mockParam := newMockReconfigureParams("simplePolicy", nil,
				withMockStatefulSet(2, nil),
				withConfigTpl("for_test", map[string]string{
					"key": "value",
				}),
				withCDComponent(appsv1alpha1.Stateless, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					}}}))
			status, err := simplePolicy.Upgrade(mockParam)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("not support component workload type"))
			Expect(status.Status).Should(BeEquivalentTo(ESNotSupport))
		})
	})

	Context("simple reconfigure policy test without not configmap volume", func() {
		It("Should failed", func() {
			// mock not cc
			mockParam := newMockReconfigureParams("simplePolicy", nil,
				withMockStatefulSet(2, nil),
				withConfigTpl("not_tpl_name", map[string]string{
					"key": "value",
				}),
				withCDComponent(appsv1alpha1.Consensus, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					}}}))
			status, err := simplePolicy.Upgrade(mockParam)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to find config meta"))
			Expect(status.Status).Should(BeEquivalentTo(ESFailed))
		})
	})
})
