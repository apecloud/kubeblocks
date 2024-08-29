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

package component

import (
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Component", func() {

	Context("has the BuildComponent function", func() {
		const (
			compDefName   = "test-compdef"
			clusterName   = "test-cluster"
			mysqlCompName = "mysql"
		)

		var (
			compDef *appsv1alpha1.ComponentDefinition
			cluster *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			compDef = testapps.NewComponentDefinitionFactory(compDefName).
				SetDefaultSpec().
				GetObject()
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				AddComponent(mysqlCompName, compDef.GetName()).
				AddVolumeClaimTemplate(testapps.DataVolumeName, testapps.NewPVCSpec("1Gi")).
				GetObject()

		})

		compObj := func() *appsv1alpha1.Component {
			comp, err := BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
			Expect(err).Should(Succeed())
			return comp
		}

		PIt("build serviceReference correctly", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			const (
				name    = "nginx"
				ns      = "default"
				kind    = "mock-kind"
				version = "mock-version"
			)
			By("generate serviceReference")
			serviceDescriptor := &appsv1.ServiceDescriptor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
				},
				Spec: appsv1.ServiceDescriptorSpec{
					ServiceKind:    kind,
					ServiceVersion: version,
				},
			}
			serviceReferenceMap := map[string]*appsv1.ServiceDescriptor{
				testapps.NginxImage: serviceDescriptor,
			}
			By("call build")
			synthesizeComp, err := BuildSynthesizedComponent(reqCtx, testCtx.Cli, cluster, compDef, compObj())
			Expect(err).Should(Succeed())
			Expect(synthesizeComp).ShouldNot(BeNil())
			Expect(synthesizeComp.ServiceReferences).ShouldNot(BeNil())
			Expect(synthesizeComp.ServiceReferences[testapps.NginxImage].Name).Should(Equal(name))
			Expect(synthesizeComp.ServiceReferences[testapps.NginxImage].Spec.ServiceKind).Should(Equal(kind))
			Expect(synthesizeComp.ServiceReferences[testapps.NginxImage].Spec.ServiceVersion).Should(Equal(version))
			Expect(serviceReferenceMap).Should(BeEmpty()) // for test failed
		})

		It("limit the shared memory volume size correctly", func() {
			var (
				_128m  = resource.MustParse("128Mi")
				_512m  = resource.MustParse("512Mi")
				_1024m = resource.MustParse("1Gi")
				_2048m = resource.MustParse("2Gi")
				reqCtx = intctrlutil.RequestCtx{Ctx: ctx, Log: logger}
			)
			compDef.Spec.Runtime.Volumes = append(compDef.Spec.Runtime.Volumes, []corev1.Volume{
				{
					Name: "shmd-ok",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
						},
					},
				},
				{
					Name: "shmd-medium",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumDefault,
						},
					},
				},
				{
					Name: "shmd-size-small",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium:    corev1.StorageMediumMemory,
							SizeLimit: &_128m,
						},
					},
				},
				{
					Name: "shmd-size-large",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium:    corev1.StorageMediumMemory,
							SizeLimit: &_2048m,
						},
					},
				},
			}...)

			By("with memory resource set")
			if cluster.Spec.ComponentSpecs[0].Resources.Requests == nil {
				cluster.Spec.ComponentSpecs[0].Resources.Requests = corev1.ResourceList{}
			}
			if cluster.Spec.ComponentSpecs[0].Resources.Limits == nil {
				cluster.Spec.ComponentSpecs[0].Resources.Limits = corev1.ResourceList{}
			}
			cluster.Spec.ComponentSpecs[0].Resources.Requests[corev1.ResourceMemory] = _512m
			cluster.Spec.ComponentSpecs[0].Resources.Limits[corev1.ResourceMemory] = _1024m
			comp, err := BuildSynthesizedComponent(reqCtx, testCtx.Cli, cluster, compDef.DeepCopy(), compObj())
			Expect(err).Should(Succeed())
			Expect(comp).ShouldNot(BeNil())
			for _, vol := range comp.PodSpec.Volumes {
				if vol.Name == "shmd-ok" {
					Expect(*vol.EmptyDir.SizeLimit).Should(BeEquivalentTo(_1024m))
				}
				if vol.Name == "shmd-medium" {
					Expect(vol.EmptyDir.SizeLimit).Should(BeNil())
				}
				if vol.Name == "shmd-size-small" {
					Expect(*vol.EmptyDir.SizeLimit).Should(BeEquivalentTo(_128m))
				}
				if vol.Name == "shmd-size-large" {
					Expect(*vol.EmptyDir.SizeLimit).Should(BeEquivalentTo(_2048m))
				}
			}

			By("without memory resource set")
			delete(cluster.Spec.ComponentSpecs[0].Resources.Requests, corev1.ResourceMemory)
			delete(cluster.Spec.ComponentSpecs[0].Resources.Limits, corev1.ResourceMemory)
			comp, err = BuildSynthesizedComponent(reqCtx, testCtx.Cli, cluster, compDef.DeepCopy(), compObj())
			Expect(err).Should(Succeed())
			Expect(comp).ShouldNot(BeNil())
			for _, vol := range comp.PodSpec.Volumes {
				if vol.Name == "shmd-ok" {
					Expect(*vol.EmptyDir.SizeLimit).Should(BeEquivalentTo(defaultShmQuantity))
				}
				if vol.Name == "shmd-medium" {
					Expect(vol.EmptyDir.SizeLimit).Should(BeNil())
				}
				if vol.Name == "shmd-size-small" {
					Expect(*vol.EmptyDir.SizeLimit).Should(BeEquivalentTo(_128m))
				}
				if vol.Name == "shmd-size-large" {
					Expect(*vol.EmptyDir.SizeLimit).Should(BeEquivalentTo(_2048m))
				}
			}
		})
	})
})

func TestGetConfigSpecByName(t *testing.T) {
	type args struct {
		component  *SynthesizedComponent
		configSpec string
	}
	tests := []struct {
		name string
		args args
		want *appsv1alpha1.ComponentConfigSpec
	}{{
		name: "test",
		args: args{
			component:  &SynthesizedComponent{},
			configSpec: "for_test",
		},
		want: nil,
	}, {
		name: "test",
		args: args{
			component: &SynthesizedComponent{
				ConfigTemplates: []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name: "test",
					}}},
			},
			configSpec: "for-test",
		},
		want: nil,
	}, {
		name: "test",
		args: args{
			component: &SynthesizedComponent{
				ConfigTemplates: []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name: "for-test",
					}}},
			},
			configSpec: "for-test",
		},
		want: &appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name: "for-test",
			}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetConfigSpecByName(tt.args.component, tt.args.configSpec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetConfigSpecByName() = %v, want %v", got, tt.want)
			}
		})
	}
}
