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
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
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
			mockParam := newMockReconfigureParams("restartPolicy", k8sMockClient.Client(),
				withMockInstanceSet(2, nil),
				withConfigSpec("for_test", map[string]string{
					"key": "value",
				}),
				withClusterComponent(2))

			// mock client update caller
			updateErr := core.MakeError("update failed!")
			k8sMockClient.MockUpdateMethod(
				testutil.WithFailed(updateErr, testutil.WithTimes(1)),
				testutil.WithSucceed(testutil.WithAnyTimes()))

			componentFullName := constant.GenerateClusterComponentName(mockParam.Cluster.Name, mockParam.ClusterComponent.Name)
			k8sMockClient.MockGetMethod(
				testutil.WithGetReturned(
					testutil.WithConstructSequenceResult(map[client.ObjectKey][]testutil.MockGetReturned{
						{Namespace: mockParam.Cluster.Namespace, Name: componentFullName}: {
							{
								Object: newMockRunningComponent(),
							},
						},
					}),
				),
				testutil.WithAnyTimes(),
			)

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

			k8sMockClient.MockUpdateMethod(testutil.WithSucceed(testutil.WithAnyTimes()))
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

			componentFullName := constant.GenerateClusterComponentName(mockParam.Cluster.Name, mockParam.ClusterComponent.Name)
			k8sMockClient.MockGetMethod(
				testutil.WithGetReturned(
					testutil.WithConstructSequenceResult(map[client.ObjectKey][]testutil.MockGetReturned{
						{Namespace: mockParam.Cluster.Namespace, Name: componentFullName}: {
							{
								Object: newMockRunningComponent(),
							},
						},
					}),
				),
				testutil.WithAnyTimes(),
			)

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
			k8sMockClient.MockUpdateMethod(
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

			componentFullName := constant.GenerateClusterComponentName(mockParam.Cluster.Name, mockParam.ClusterComponent.Name)
			k8sMockClient.MockGetMethod(
				testutil.WithGetReturned(
					testutil.WithConstructSequenceResult(map[client.ObjectKey][]testutil.MockGetReturned{
						{Namespace: mockParam.Cluster.Namespace, Name: componentFullName}: {
							{
								Object: newMockRunningComponent(),
							},
						},
					}),
				),
				testutil.WithAnyTimes(),
			)

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

// Mock helper functions for testing
type paramsOps func(*reconfigureContext)

func withMockInstanceSet(replicas int, labels map[string]string) paramsOps {
	return func(rc *reconfigureContext) {
		// Create a simple InstanceSet for testing
		if rc.InstanceSetUnits == nil {
			rc.InstanceSetUnits = make([]workloads.InstanceSet, 0)
		}
		// Add minimal InstanceSet structure
		rc.InstanceSetUnits = append(rc.InstanceSetUnits, workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock-instanceset",
				Namespace: "default",
			},
		})
	}
}

func withConfigSpec(configSpecName string, data map[string]string) paramsOps {
	return func(rc *reconfigureContext) {
		// Create minimal ConfigTemplate
		rc.ConfigTemplate = appsv1.ComponentFileTemplate{
			Name: configSpecName,
		}
		// Create ConfigMap with data
		if rc.ConfigMap == nil {
			rc.ConfigMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mock-configmap",
					Namespace: "default",
				},
				Data: data,
			}
		} else {
			rc.ConfigMap.Data = data
		}
	}
}

func withClusterComponent(replicas int) paramsOps {
	return func(rc *reconfigureContext) {
		if rc.ClusterComponent == nil {
			rc.ClusterComponent = &appsv1.ClusterComponentSpec{
				Name:     "mock-component",
				Replicas: int32(replicas),
			}
		}
	}
}

func newMockReconfigureParams(testName string, cli client.Client, paramOps ...paramsOps) reconfigureContext {
	rc := reconfigureContext{
		RequestCtx: intctrlutil.RequestCtx{
			Ctx: context.Background(),
			Log: log.FromContext(context.Background()),
		},
		Client: cli,
		Cluster: &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock-cluster",
				Namespace: "default",
			},
			Spec: appsv1.ClusterSpec{},
		},
	}

	// Apply all parameter operations
	for _, op := range paramOps {
		op(&rc)
	}

	return rc
}

func newMockRunningComponent() client.Object {
	// Return a minimal Component object
	return &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock-component",
			Namespace: "default",
		},
		Status: appsv1.ComponentStatus{
			Phase: appsv1.RunningComponentPhase,
		},
	}
}

func fromPodObjectList(pods []runtime.Object) []runtime.Object {
	// Simple conversion function
	return pods
}

func newMockPodsWithInstanceSet(its *workloads.InstanceSet, count int, opts ...func(*corev1.Pod, int)) []runtime.Object {
	pods := make([]runtime.Object, count)
	for i := 0; i < count; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "default",
				Name:            fmt.Sprintf("mock-pod-%d", i),
				OwnerReferences: []metav1.OwnerReference{newControllerRef(its, workloads.GroupVersion.WithKind(workloads.InstanceSetKind))},
			},
			Spec: *its.Spec.Template.Spec.DeepCopy(),
			Status: corev1.PodStatus{
				PodIP: "1.1.1.1",
			},
		}
		// Apply options
		for _, opt := range opts {
			opt(pod, i)
		}
		pods[i] = pod
	}
	return pods
}

func withReadyPod(start, end int) func(*corev1.Pod, int) {
	return func(pod *corev1.Pod, index int) {
		// Mark pod as ready if within range
		if index >= start && index < end {
			pod.Status.Conditions = []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			}
			pod.Status.Phase = corev1.PodRunning
		}
	}
}

func newControllerRef(owner client.Object, gvk schema.GroupVersionKind) metav1.OwnerReference {
	bRefFn := func(b bool) *bool { return &b }
	return metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		Controller:         bRefFn(true),
		BlockOwnerDeletion: bRefFn(false),
	}
}
