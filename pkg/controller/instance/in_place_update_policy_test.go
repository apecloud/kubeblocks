/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package instance

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// buildTestInstanceWithPod creates an Instance with a simple PodTemplate
// and returns the Instance along with a matching Pod built from it.
func buildTestInstanceWithPod(t *testing.T) (*workloads.Instance, *corev1.Pod) {
	t.Helper()
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("uid-test-1234")).
		SetPodTemplate(corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "mysql"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "mysql",
					Image: "mysql:8.0",
				}},
			},
		}).
		SetSelectorMatchLabels(map[string]string{"app": "mysql"}).
		SetInstanceSetName("mysql").
		GetObject()

	pod, err := buildInstancePod(inst, "")
	if err != nil {
		t.Fatalf("buildInstancePod() error = %v", err)
	}
	revision := getPodRevision(pod)
	inst.Status.UpdateRevision = revision
	return inst, pod
}

func TestSupportPodVerticalScaling(t *testing.T) {
	orig := viper.GetBool(constant.FeatureGateInPlacePodVerticalScaling)
	defer viper.Set(constant.FeatureGateInPlacePodVerticalScaling, orig)

	viper.Set(constant.FeatureGateInPlacePodVerticalScaling, true)
	if !supportPodVerticalScaling() {
		t.Fatal("expected supportPodVerticalScaling to return true when feature gate is enabled")
	}

	viper.Set(constant.FeatureGateInPlacePodVerticalScaling, false)
	if supportPodVerticalScaling() {
		t.Fatal("expected supportPodVerticalScaling to return false when feature gate is disabled")
	}
}

func TestFilterInPlaceFields(t *testing.T) {
	template := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.RestartAnnotationKey: "2026-01-01T00:00:00Z",
				"other-annotation":            "value",
			},
			Labels: map[string]string{"app": "mysql"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "mysql",
				Image: "mysql:8.0",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
			}},
			InitContainers: []corev1.Container{{
				Name:  "init",
				Image: "init:1.0",
			}},
			ActiveDeadlineSeconds: ptr.To[int64](100),
			Tolerations: []corev1.Toleration{{
				Key:      "node.kubernetes.io/not-ready",
				Operator: corev1.TolerationOpExists,
			}},
		},
	}

	result := filterInPlaceFields(template)

	// annotations: only Restart annotation should be kept
	if result.Annotations[constant.RestartAnnotationKey] != "2026-01-01T00:00:00Z" {
		t.Fatalf("expected restart annotation to be kept, got %#v", result.Annotations)
	}
	if _, ok := result.Annotations["other-annotation"]; ok {
		t.Fatal("expected other-annotation to be filtered out")
	}

	// labels should be nil
	if result.Labels != nil {
		t.Fatalf("expected labels to be nil, got %#v", result.Labels)
	}

	// container images should be cleared
	if result.Spec.Containers[0].Image != "" {
		t.Fatalf("expected container image to be cleared, got %s", result.Spec.Containers[0].Image)
	}
	if result.Spec.InitContainers[0].Image != "" {
		t.Fatalf("expected init container image to be cleared, got %s", result.Spec.InitContainers[0].Image)
	}

	// ActiveDeadlineSeconds should be nil
	if result.Spec.ActiveDeadlineSeconds != nil {
		t.Fatal("expected ActiveDeadlineSeconds to be nil")
	}

	// Tolerations should be nil
	if result.Spec.Tolerations != nil {
		t.Fatal("expected Tolerations to be nil")
	}

	// CPU and Memory resources should be removed
	if _, ok := result.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]; ok {
		t.Fatal("expected CPU request to be removed")
	}
	if _, ok := result.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory]; ok {
		t.Fatal("expected Memory request to be removed")
	}
	if _, ok := result.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU]; ok {
		t.Fatal("expected CPU limit to be removed")
	}
	if _, ok := result.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory]; ok {
		t.Fatal("expected Memory limit to be removed")
	}
}

func TestFilterInPlaceFieldsEmpty(t *testing.T) {
	template := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
		},
	}
	result := filterInPlaceFields(template)
	if result.Annotations != nil {
		t.Fatalf("expected nil annotations, got %#v", result.Annotations)
	}
	if result.Labels != nil {
		t.Fatalf("expected nil labels, got %#v", result.Labels)
	}
}

func TestGetPodUpdatePolicyNoOps(t *testing.T) {
	origIgnore := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	defer viper.Set(FeatureGateIgnorePodVerticalScaling, origIgnore)
	viper.Set(FeatureGateIgnorePodVerticalScaling, false)

	inst, pod := buildTestInstanceWithPod(t)
	policy, specPolicy, err := getPodUpdatePolicy(inst, pod)
	if err != nil {
		t.Fatalf("getPodUpdatePolicy() error = %v", err)
	}
	if policy != noOpsPolicy {
		t.Fatalf("expected noOpsPolicy, got %s", policy)
	}
	if specPolicy != "" {
		t.Fatalf("expected empty specPolicy for noOps, got %s", specPolicy)
	}
}

func TestGetPodUpdatePolicyRecreateByConfigRestart(t *testing.T) {
	inst, pod := buildTestInstanceWithPod(t)
	// set up config that requires restart
	inst.Spec.Configs = []workloads.ConfigTemplate{{
		Name:       "mysql-conf",
		ConfigHash: ptr.To("new-hash"),
		Restart:    ptr.To(true),
	}}
	// set pod's config annotation to old hash
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[constant.CMInsConfigurationHashLabelKey] = `{"mysql-conf":"old-hash"}`

	policy, specPolicy, err := getPodUpdatePolicy(inst, pod)
	if err != nil {
		t.Fatalf("getPodUpdatePolicy() error = %v", err)
	}
	if policy != recreatePolicy {
		t.Fatalf("expected recreatePolicy for config restart, got %s", policy)
	}
	if specPolicy != inst.Spec.PodUpdatePolicy {
		t.Fatalf("expected specPolicy to be PodUpdatePolicy, got %s", specPolicy)
	}
}

func TestGetPodUpdatePolicyRecreateByRevisionMismatch(t *testing.T) {
	inst, pod := buildTestInstanceWithPod(t)
	// set UpdateRevision to something different from pod's revision
	inst.Status.UpdateRevision = "different-revision"

	policy, _, err := getPodUpdatePolicy(inst, pod)
	if err != nil {
		t.Fatalf("getPodUpdatePolicy() error = %v", err)
	}
	if policy != recreatePolicy {
		t.Fatalf("expected recreatePolicy for revision mismatch, got %s", policy)
	}
}

func TestGetPodUpdatePolicyNoOpsWhenUpdateRevisionEmpty(t *testing.T) {
	inst, pod := buildTestInstanceWithPod(t)
	// clear the pod's revision label so getPodRevision(pod) != inst.Status.UpdateRevision
	pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "some-other-rev"
	inst.Status.UpdateRevision = ""

	policy, _, err := getPodUpdatePolicy(inst, pod)
	if err != nil {
		t.Fatalf("getPodUpdatePolicy() error = %v", err)
	}
	if policy != noOpsPolicy {
		t.Fatalf("expected noOpsPolicy when UpdateRevision is empty, got %s", policy)
	}
}

func TestGetPodUpdatePolicyInPlaceForImageChange(t *testing.T) {
	origIgnore := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	defer viper.Set(FeatureGateIgnorePodVerticalScaling, origIgnore)
	viper.Set(FeatureGateIgnorePodVerticalScaling, false)

	inst, pod := buildTestInstanceWithPod(t)
	// change the pod's container image to trigger basic update
	// we need the pod's status to have the old image and spec to have new image
	// Actually, equalBasicInPlaceFields compares the pod spec with the newly built pod
	// The newly built pod will have the instance's template image
	// So if we change the pod's container image, it should differ from the template
	pod.Spec.Containers[0].Image = "mysql:8.4"

	policy, _, err := getPodUpdatePolicy(inst, pod)
	if err != nil {
		t.Fatalf("getPodUpdatePolicy() error = %v", err)
	}
	if policy != inPlaceUpdatePolicy {
		t.Fatalf("expected inPlaceUpdatePolicy for image change, got %s", policy)
	}
}

func TestGetPodUpdatePolicyRecreateForResourceUpdate(t *testing.T) {
	origIgnore := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	defer viper.Set(FeatureGateIgnorePodVerticalScaling, origIgnore)
	origVScale := viper.GetBool(constant.FeatureGateInPlacePodVerticalScaling)
	defer viper.Set(constant.FeatureGateInPlacePodVerticalScaling, origVScale)
	viper.Set(FeatureGateIgnorePodVerticalScaling, false)
	viper.Set(constant.FeatureGateInPlacePodVerticalScaling, false)

	inst, pod := buildTestInstanceWithPod(t)
	// change pod's container resources to trigger resource update
	// The new pod (built from instance template) won't have resources set
	// So if the old pod has resources, it should be a resource update
	pod.Spec.Containers[0].Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
	}

	policy, _, err := getPodUpdatePolicy(inst, pod)
	if err != nil {
		t.Fatalf("getPodUpdatePolicy() error = %v", err)
	}
	if policy != recreatePolicy {
		t.Fatalf("expected recreatePolicy for resource update without vertical scaling, got %s", policy)
	}
}

func TestGetPodUpdatePolicyInPlaceForResourceUpdate(t *testing.T) {
	origIgnore := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	defer viper.Set(FeatureGateIgnorePodVerticalScaling, origIgnore)
	origVScale := viper.GetBool(constant.FeatureGateInPlacePodVerticalScaling)
	defer viper.Set(constant.FeatureGateInPlacePodVerticalScaling, origVScale)
	viper.Set(FeatureGateIgnorePodVerticalScaling, false)
	viper.Set(constant.FeatureGateInPlacePodVerticalScaling, true)

	inst, pod := buildTestInstanceWithPod(t)
	// change pod's container resources
	pod.Spec.Containers[0].Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
	}

	policy, _, err := getPodUpdatePolicy(inst, pod)
	if err != nil {
		t.Fatalf("getPodUpdatePolicy() error = %v", err)
	}
	if policy != inPlaceUpdatePolicy {
		t.Fatalf("expected inPlaceUpdatePolicy for resource update with vertical scaling, got %s", policy)
	}
}

func TestGetPodUpdatePolicyInPlaceWithIgnorePodVerticalScaling(t *testing.T) {
	origIgnore := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	defer viper.Set(FeatureGateIgnorePodVerticalScaling, origIgnore)
	viper.Set(FeatureGateIgnorePodVerticalScaling, true)

	inst, pod := buildTestInstanceWithPod(t)
	// change pod's container image to trigger basic update
	pod.Spec.Containers[0].Image = "mysql:8.4"

	policy, _, err := getPodUpdatePolicy(inst, pod)
	if err != nil {
		t.Fatalf("getPodUpdatePolicy() error = %v", err)
	}
	if policy != inPlaceUpdatePolicy {
		t.Fatalf("expected inPlaceUpdatePolicy with ignorePodVerticalScaling, got %s", policy)
	}
}

func TestGetPodUpdatePolicyNoOpsWithIgnorePodVerticalScaling(t *testing.T) {
	origIgnore := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	defer viper.Set(FeatureGateIgnorePodVerticalScaling, origIgnore)
	viper.Set(FeatureGateIgnorePodVerticalScaling, true)

	inst, pod := buildTestInstanceWithPod(t)
	// pod matches template exactly, should be noOps
	policy, _, err := getPodUpdatePolicy(inst, pod)
	if err != nil {
		t.Fatalf("getPodUpdatePolicy() error = %v", err)
	}
	if policy != noOpsPolicy {
		t.Fatalf("expected noOpsPolicy, got %s", policy)
	}
}

func TestGetPodUpdatePolicyInSpec(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetPodUpdatePolicy(kbappsv1.PreferInPlacePodUpdatePolicyType).
		SetPodUpgradePolicy(kbappsv1.ReCreatePodUpdatePolicyType).
		SetPodTemplate(corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
			},
		}).
		GetObject()

	// same containers -> returns PodUpdatePolicy
	oldPod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}}}}
	newPod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}}}}
	policy := getPodUpdatePolicyInSpec(inst, oldPod, newPod)
	if policy != kbappsv1.PreferInPlacePodUpdatePolicyType {
		t.Fatalf("expected PreferInPlacePodUpdatePolicyType, got %s", policy)
	}

	// different containers -> returns PodUpgradePolicy
	newPod.Spec.Containers[0].Image = "mysql:8.4"
	policy = getPodUpdatePolicyInSpec(inst, oldPod, newPod)
	if policy != kbappsv1.ReCreatePodUpdatePolicyType {
		t.Fatalf("expected ReCreatePodUpdatePolicyType, got %s", policy)
	}

	// different init containers -> returns PodUpgradePolicy
	oldPod.Spec.InitContainers = []corev1.Container{{Name: "init", Image: "init:1.0"}}
	newPod.Spec.InitContainers = []corev1.Container{{Name: "init", Image: "init:2.0"}}
	newPod.Spec.Containers[0].Image = "mysql:8.0" // reset containers
	policy = getPodUpdatePolicyInSpec(inst, oldPod, newPod)
	if policy != kbappsv1.ReCreatePodUpdatePolicyType {
		t.Fatalf("expected ReCreatePodUpdatePolicyType for init container change, got %s", policy)
	}
}

func TestIsPodUpdated(t *testing.T) {
	origIgnore := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	defer viper.Set(FeatureGateIgnorePodVerticalScaling, origIgnore)
	viper.Set(FeatureGateIgnorePodVerticalScaling, false)

	inst, pod := buildTestInstanceWithPod(t)

	// pod matches template -> updated
	updated, err := isPodUpdated(inst, pod)
	if err != nil {
		t.Fatalf("isPodUpdated() error = %v", err)
	}
	if !updated {
		t.Fatal("expected pod to be updated when it matches template")
	}

	// pod has different image -> not updated
	pod.Spec.Containers[0].Image = "mysql:8.4"
	updated, err = isPodUpdated(inst, pod)
	if err != nil {
		t.Fatalf("isPodUpdated() error = %v", err)
	}
	if updated {
		t.Fatal("expected pod to not be updated when image differs")
	}
}
