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

package trace

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	"github.com/apecloud/kubeblocks/pkg/testutil/k8s/mocks"
)

var _ = Describe("dry_run_handler test", func() {
	var (
		k8sMock    *mocks.MockClient
		controller *gomock.Controller
	)

	BeforeEach(func() {
		controller, k8sMock = testutil.SetupK8sMock()
	})

	AfterEach(func() {
		controller.Finish()
	})

	Context("Testing dry_run_handler", func() {
		It("should work well", func() {
			clusterDefinition := &kbappsv1.ClusterDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  namespace,
					Name:       name,
					Generation: int64(1),
				},
				Spec: kbappsv1.ClusterDefinitionSpec{
					Topologies: []kbappsv1.ClusterTopology{
						{
							Name:    name,
							Default: true,
							Components: []kbappsv1.ClusterTopologyComponent{
								{
									Name:    name,
									CompDef: name,
								},
							},
						},
					},
				},
				Status: kbappsv1.ClusterDefinitionStatus{
					ObservedGeneration: int64(1),
					Phase:              kbappsv1.AvailablePhase,
				},
			}
			serviceVersion := "1.0.0"
			componentDefinition := &kbappsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  namespace,
					Name:       name,
					Generation: int64(1),
				},
				Spec: kbappsv1.ComponentDefinitionSpec{
					ServiceVersion: serviceVersion,
					Runtime: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  name,
							Image: "busybox",
						}},
					},
				},
				Status: kbappsv1.ComponentDefinitionStatus{
					ObservedGeneration: int64(1),
					Phase:              kbappsv1.AvailablePhase,
				},
			}
			componentVersion := &kbappsv1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  namespace,
					Name:       name,
					Generation: int64(1),
					Annotations: map[string]string{
						"componentversion.kubeblocks.io/compatible-definitions": name,
					},
				},
				Spec: kbappsv1.ComponentVersionSpec{
					CompatibilityRules: []kbappsv1.ComponentVersionCompatibilityRule{{
						CompDefs: []string{name},
						Releases: []string{name},
					}},
					Releases: []kbappsv1.ComponentVersionRelease{{
						Name:           name,
						ServiceVersion: serviceVersion,
						Images: map[string]string{
							name: "busybox",
						},
					}},
				},
				Status: kbappsv1.ComponentVersionStatus{
					ObservedGeneration: int64(1),
					Phase:              kbappsv1.AvailablePhase,
				},
			}
			clusterTemplate := &kbappsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            name,
					UID:             uid,
					ResourceVersion: "1",
				},
				Spec: kbappsv1.ClusterSpec{
					ClusterDef:        name,
					TerminationPolicy: kbappsv1.WipeOut,
					ComponentSpecs: []kbappsv1.ClusterComponentSpec{{
						Name:     name,
						Replicas: 0,
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
						VolumeClaimTemplates: []kbappsv1.ClusterComponentVolumeClaimTemplate{{
							Name: name,
							Spec: kbappsv1.PersistentVolumeClaimSpec{
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: resource.MustParse("20Gi"),
									},
								},
							},
						}},
					}},
				},
			}
			primaryV1 := clusterTemplate.DeepCopy()
			primaryV1.Generation = 1
			primaryV1.ResourceVersion = "2"
			primaryV1.Status.Phase = kbappsv1.RunningClusterPhase
			primaryV2 := clusterTemplate.DeepCopy()
			primaryV2.Generation = 2
			primaryV2.ResourceVersion = "3"
			primaryV2.Spec.ComponentSpecs[0].Replicas = 1
			specJSON, err := json.Marshal(primaryV2.Spec)
			Expect(err).ToNot(HaveOccurred())
			var specMap map[string]interface{}
			Expect(json.Unmarshal(specJSON, &specMap)).To(Succeed())
			desiredSpec, err := yaml.Marshal(specMap)
			Expect(err).NotTo(HaveOccurred())

			trace := &tracev1.ReconciliationTrace{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
				Spec: tracev1.ReconciliationTraceSpec{
					TargetObject: &tracev1.ObjectReference{
						Namespace: primaryV1.Namespace,
						Name:      primaryV1.Name,
					},
					DryRun: &tracev1.DryRun{
						DesiredSpec: string(desiredSpec),
					},
				},
			}

			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(trace)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &kbappsv1.ClusterDefinition{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *kbappsv1.ClusterDefinition, _ ...client.GetOption) error {
					*obj = *clusterDefinition.DeepCopy()
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &kbappsv1.ComponentDefinition{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *kbappsv1.ComponentDefinition, _ ...client.GetOption) error {
					*obj = *componentDefinition.DeepCopy()
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				List(gomock.Any(), &kbappsv1.ComponentDefinitionList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *kbappsv1.ComponentDefinitionList, _ ...client.GetOption) error {
					list.Items = []kbappsv1.ComponentDefinition{*componentDefinition.DeepCopy()}
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				List(gomock.Any(), &kbappsv1.ComponentVersionList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *kbappsv1.ComponentVersionList, _ ...client.ListOption) error {
					list.Items = []kbappsv1.ComponentVersion{*componentVersion.DeepCopy()}
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				List(gomock.Any(), &kbappsv1.SidecarDefinitionList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *kbappsv1.SidecarDefinitionList, _ ...client.ListOption) error {
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				List(gomock.Any(), &dpv1alpha1.BackupPolicyTemplateList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *dpv1alpha1.BackupPolicyTemplateList, _ ...client.ListOption) error {
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &kbappsv1.Cluster{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *kbappsv1.Cluster, _ ...client.GetOption) error {
					*obj = *primaryV1.DeepCopy()
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &kbappsv1.ComponentList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *kbappsv1.ComponentList, _ ...client.ListOption) error {
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.ServiceList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.ServiceList, _ ...client.ListOption) error {
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.SecretList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &kbappsv1.Component{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *kbappsv1.Component, _ ...client.GetOption) error {
					return apierrors.NewNotFound(kbappsv1.Resource(kbappsv1.ComponentKind), objKey.Name)
				}).AnyTimes()
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &appsv1alpha1.Configuration{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *appsv1alpha1.Configuration, _ ...client.GetOption) error {
					return apierrors.NewNotFound(appsv1alpha1.Resource(constant.ConfigurationKind), objKey.Name)
				}).AnyTimes()
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.ConfigMap{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.ConfigMap, _ ...client.GetOption) error {
					return apierrors.NewNotFound(corev1.Resource(constant.ConfigMapKind), objKey.Name)
				}).AnyTimes()
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.PersistentVolumeClaim{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.PersistentVolumeClaim, _ ...client.GetOption) error {
					return apierrors.NewNotFound(corev1.Resource(constant.PersistentVolumeClaimKind), objKey.Name)
				}).AnyTimes()
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.PersistentVolume{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.PersistentVolume, _ ...client.GetOption) error {
					return apierrors.NewNotFound(corev1.Resource(constant.PersistentVolumeKind), objKey.Name)
				}).AnyTimes()
			k8sMock.EXPECT().Scheme().Return(scheme.Scheme).AnyTimes()

			reconciler := dryRun(ctx, k8sMock, scheme.Scheme)
			Expect(reconciler.PreCondition(tree)).To(Equal(kubebuilderx.ConditionSatisfied))
			res, err := reconciler.Reconcile(tree)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			Expect(trace.Status.DryRunResult).ShouldNot(BeNil())
			Expect(trace.Status.DryRunResult.Phase).Should(Equal(tracev1.DryRunSucceedPhase))
			Expect(trace.Status.DryRunResult.DesiredSpecRevision).ShouldNot(BeEmpty())
			Expect(trace.Status.DryRunResult.ObservedTargetGeneration).Should(Equal(primaryV1.Generation))
			Expect(trace.Status.DryRunResult.SpecDiff).ShouldNot(BeEmpty())
			Expect(trace.Status.DryRunResult.Plan.ObjectTree).ShouldNot(BeNil())
			Expect(len(trace.Status.DryRunResult.Plan.Changes)).Should(BeNumerically(">", 0))
			Expect(trace.Status.DryRunResult.Plan.Summary.ObjectSummaries).ShouldNot(BeNil())
		})
	})
})
