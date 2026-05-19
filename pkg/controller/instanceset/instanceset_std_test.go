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

package instanceset

import (
	"crypto/sha256"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

// --- ValidationReconciler ---

func TestNewValidationReconciler(t *testing.T) {
	r := NewValidationReconciler()
	require.NotNil(t, r)
}

func TestValidationReconciler_PreCondition_NilRoot(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	r := NewValidationReconciler()
	result := r.PreCondition(tree)
	assert.False(t, result.Satisfied)
}

func TestValidationReconciler_PreCondition_WithRoot(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(&workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-its", Namespace: "ns"},
	})
	r := NewValidationReconciler()
	result := r.PreCondition(tree)
	assert.True(t, result.Satisfied)
}

// --- GetMatchLabels ---

func TestGetMatchLabels(t *testing.T) {
	labels := GetMatchLabels("my-its")
	assert.NotEmpty(t, labels)
}

// --- GetPodNameSetFromInstanceSetCondition ---

func TestGetPodNameSetFromInstanceSetCondition_NoCondition(t *testing.T) {
	its := &workloads.InstanceSet{}
	result := GetPodNameSetFromInstanceSetCondition(its, workloads.InstanceReady)
	assert.Empty(t, result)
}

func TestGetPodNameSetFromInstanceSetCondition_WithCondition(t *testing.T) {
	podNames := []string{"pod-0", "pod-1"}
	msg, _ := json.Marshal(podNames)
	its := &workloads.InstanceSet{
		Status: workloads.InstanceSetStatus{
			Conditions: []metav1.Condition{
				{
					Type:    string(workloads.InstanceReady),
					Status:  metav1.ConditionFalse,
					Message: string(msg),
				},
			},
		},
	}
	result := GetPodNameSetFromInstanceSetCondition(its, workloads.InstanceReady)
	assert.Len(t, result, 2)
}

func TestGetPodNameSetFromInstanceSetCondition_TrueCondition(t *testing.T) {
	its := &workloads.InstanceSet{
		Status: workloads.InstanceSetStatus{
			Conditions: []metav1.Condition{
				{
					Type:   string(workloads.InstanceReady),
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	result := GetPodNameSetFromInstanceSetCondition(its, workloads.InstanceReady)
	assert.Empty(t, result)
}

// --- GenerateAllInstanceNames ---

func TestGenerateAllInstanceNames_Basic(t *testing.T) {
	names, err := GenerateAllInstanceNames("parent", 3, nil, nil, kbappsv1.Ordinals{})
	require.NoError(t, err)
	assert.Len(t, names, 3)
	assert.Equal(t, "parent-0", names[0])
	assert.Equal(t, "parent-1", names[1])
	assert.Equal(t, "parent-2", names[2])
}

func TestGenerateAllInstanceNames_WithTemplate(t *testing.T) {
	tmpl := &workloads.InstanceTemplate{
		Name:     "special",
		Replicas: func() *int32 { v := int32(1); return &v }(),
	}
	names, err := GenerateAllInstanceNames("parent", 3, []InstanceTemplate{tmpl}, nil, kbappsv1.Ordinals{})
	require.NoError(t, err)
	assert.Len(t, names, 3)
}

// --- NewUpdatePlan ---

func TestNewUpdatePlan(t *testing.T) {
	its := workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "its", Namespace: "ns"},
		Spec: workloads.InstanceSetSpec{
			Replicas: func() *int32 { r := int32(1); return &r }(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}
	pods := []*corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "its-0"}}}
	plan := NewUpdatePlan(its, pods, func(i *workloads.InstanceSet, p *corev1.Pod) (bool, error) {
		return true, nil
	})
	assert.NotNil(t, plan)
}

// --- deepHashObject ---

func TestDeepHashObject(t *testing.T) {
	h := sha256.New()
	deepHashObject(h, "test-value")
	sum := h.Sum(nil)
	assert.NotEmpty(t, sum)

	// Calling again should produce same result
	deepHashObject(h, "test-value")
	sum2 := h.Sum(nil)
	assert.Equal(t, sum, sum2)
}

// --- Reconciler PreCondition nil root tests ---

func TestDeletionReconciler_PreCondition_NilRoot(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	r := NewDeletionReconciler()
	result := r.PreCondition(tree)
	assert.False(t, result.Satisfied)
}

func TestFixMetaReconciler_PreCondition_NilRoot(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	r := NewFixMetaReconciler()
	result := r.PreCondition(tree)
	assert.False(t, result.Satisfied)
}

func TestReplicasAlignmentReconciler_PreCondition_NilRoot(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	r := NewReplicasAlignmentReconciler()
	result := r.PreCondition(tree)
	assert.False(t, result.Satisfied)
}

func TestUpdateReconciler_PreCondition_NilRoot(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	r := NewUpdateReconciler()
	result := r.PreCondition(tree)
	assert.False(t, result.Satisfied)
}

func TestStatusReconciler_PreCondition_NilRoot(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	r := NewStatusReconciler()
	result := r.PreCondition(tree)
	assert.False(t, result.Satisfied)
}

func TestRevisionUpdateReconciler_PreCondition_NilRoot(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	r := NewRevisionUpdateReconciler()
	result := r.PreCondition(tree)
	assert.False(t, result.Satisfied)
}

func TestAPIVersionReconciler_PreCondition_NilRoot(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	r := NewAPIVersionReconciler()
	result := r.PreCondition(tree)
	assert.False(t, result.Satisfied)
}

func TestAssistantObjectReconciler_PreCondition_NilRoot(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	r := NewAssistantObjectReconciler()
	result := r.PreCondition(tree)
	assert.False(t, result.Satisfied)
}

// --- imageSplit edge case ---

func TestImageSplit_NoTag(t *testing.T) {
	name, tag, digest := imageSplit("myregistry/myrepo")
	assert.Equal(t, "myregistry/myrepo", name)
	assert.Empty(t, tag)
	assert.Empty(t, digest)
}

func TestImageSplit_WithDigest(t *testing.T) {
	name, tag, digest := imageSplit("myregistry/myrepo@sha256:abc123")
	assert.Equal(t, "myregistry/myrepo", name)
	assert.Empty(t, tag)
	assert.Equal(t, "sha256:abc123", digest)
}

func TestImageSplit_SimpleImage(t *testing.T) {
	name, tag, digest := imageSplit("nginx:latest")
	assert.Equal(t, "nginx", name)
	assert.Equal(t, "latest", tag)
	assert.Empty(t, digest)
}

// --- hasConfigRestart ---

// --- newLifecycleAction ---

func TestNewLifecycleAction_NilLifecycleActions(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "its",
			Namespace: "ns",
			Labels: map[string]string{
				"app.kubernetes.io/instance":        "cluster",
				"apps.kubeblocks.io/component-name": "mysql",
			},
		},
		Spec: workloads.InstanceSetSpec{
			Replicas: func() *int32 { r := int32(1); return &r }(),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
			LifecycleActions: &workloads.LifecycleActions{},
		},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "its-0", Namespace: "ns"}}
	require.NoError(t, tree.Add(pod))
	la, err := newLifecycleAction(its, tree, pod)
	require.NoError(t, err)
	assert.NotNil(t, la)
}

// --- parseParentNameAndOrdinal edge case ---

func TestParseParentNameAndOrdinal_WithOrdinal(t *testing.T) {
	parent, ordinal := parseParentNameAndOrdinal("my-its-2")
	assert.Equal(t, "my-its", parent)
	assert.Equal(t, 2, ordinal)
}

// --- controllerRevisionName ---

func TestControllerRevisionName_Short(t *testing.T) {
	name := controllerRevisionName("my-its", "abc123")
	assert.Equal(t, "my-its-abc123", name)
}

func TestControllerRevisionName_LongPrefix(t *testing.T) {
	longPrefix := ""
	for i := 0; i < 230; i++ {
		longPrefix += "a"
	}
	name := controllerRevisionName(longPrefix, "hash")
	assert.True(t, len(name) <= 253+1+4) // prefix truncated to 223 + "-" + hash
	assert.Equal(t, longPrefix[:223]+"-hash", name)
}

// --- ConvertOrdinalsToSortedList ---

func TestConvertOrdinalsToSortedList_InvalidRange(t *testing.T) {
	ordinals := kbappsv1.Ordinals{
		Ranges: []kbappsv1.Range{{Start: 5, End: 2}},
	}
	_, err := ConvertOrdinalsToSortedList(ordinals)
	assert.Error(t, err)
}

func TestConvertOrdinalsToSortedList_DiscreteAndRanges(t *testing.T) {
	ordinals := kbappsv1.Ordinals{
		Discrete: []int32{10, 20},
		Ranges:   []kbappsv1.Range{{Start: 0, End: 2}},
	}
	result, err := ConvertOrdinalsToSortedList(ordinals)
	require.NoError(t, err)
	assert.Equal(t, []int32{0, 1, 2, 10, 20}, result)
}

// --- ParseNodeSelectorOnceAnnotation ---

func TestParseNodeSelectorOnceAnnotation_NoAnnotation(t *testing.T) {
	its := &workloads.InstanceSet{}
	result, err := ParseNodeSelectorOnceAnnotation(its)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParseNodeSelectorOnceAnnotation_InvalidJSON(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.NodeSelectorOnceAnnotationKey: "not-json",
			},
		},
	}
	_, err := ParseNodeSelectorOnceAnnotation(its)
	assert.Error(t, err)
}

func TestParseNodeSelectorOnceAnnotation_Valid(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.NodeSelectorOnceAnnotationKey: `{"pod-0":"node-1","pod-1":"node-2"}`,
			},
		},
	}
	result, err := ParseNodeSelectorOnceAnnotation(its)
	require.NoError(t, err)
	assert.Equal(t, "node-1", result["pod-0"])
	assert.Equal(t, "node-2", result["pod-1"])
}

// --- configsToPod / configsFromPod ---

func TestConfigsToPod_Empty(t *testing.T) {
	pod := &corev1.Pod{}
	err := configsToPod(nil, pod)
	require.NoError(t, err)
	assert.Empty(t, pod.Annotations)
}

func TestConfigsToPod_WithConfigs(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-0"}}
	configs := []workloads.ConfigTemplate{
		{Name: "cfg1", ConfigHash: ptr.To("hash1")},
	}
	err := configsToPod(configs, pod)
	require.NoError(t, err)
	assert.Contains(t, pod.Annotations[constant.CMInsConfigurationHashLabelKey], "cfg1")
}

func TestConfigsFromPod_Empty(t *testing.T) {
	pod := &corev1.Pod{}
	configs, err := configsFromPod(pod)
	require.NoError(t, err)
	assert.Nil(t, configs)
}

func TestConfigsFromPod_WithData(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.CMInsConfigurationHashLabelKey: `{"cfg1":"hash1","cfg2":"hash2"}`,
			},
		},
	}
	configs, err := configsFromPod(pod)
	require.NoError(t, err)
	assert.Len(t, configs, 2)
}

func TestConfigsFromPod_InvalidJSON(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.CMInsConfigurationHashLabelKey: "not-json",
			},
		},
	}
	_, err := configsFromPod(pod)
	assert.Error(t, err)
}

// --- configsToUpdate ---

func TestConfigsToUpdate_NoConfigs(t *testing.T) {
	its := &workloads.InstanceSet{}
	pod := &corev1.Pod{}
	configs, err := configsToUpdate(its, pod)
	require.NoError(t, err)
	assert.Empty(t, configs)
}

func TestConfigsToUpdate_NewConfig(t *testing.T) {
	its := &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{
			Configs: []workloads.ConfigTemplate{
				{Name: "cfg1", ConfigHash: ptr.To("newhash")},
			},
		},
	}
	pod := &corev1.Pod{} // no annotations = no existing configs
	configs, err := configsToUpdate(its, pod)
	require.NoError(t, err)
	assert.Len(t, configs, 1)
}

func TestConfigsToUpdate_SameHash(t *testing.T) {
	its := &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{
			Configs: []workloads.ConfigTemplate{
				{Name: "cfg1", ConfigHash: ptr.To("hash1")},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.CMInsConfigurationHashLabelKey: `{"cfg1":"hash1"}`,
			},
		},
	}
	configs, err := configsToUpdate(its, pod)
	require.NoError(t, err)
	assert.Empty(t, configs)
}

// --- hasConfigRestart with configs ---

func TestHasConfigRestart_WithRestartConfig(t *testing.T) {
	its := &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{
			Configs: []workloads.ConfigTemplate{
				{Name: "cfg1", ConfigHash: ptr.To("newhash"), Restart: ptr.To(true)},
			},
		},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "its-0", Namespace: "ns"}}
	restart, names, err := hasConfigRestart(its, pod)
	require.NoError(t, err)
	assert.True(t, restart)
	assert.Contains(t, names, "cfg1")
}

func TestHasConfigRestart_NoConfigs(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "its", Namespace: "ns"},
		Spec: workloads.InstanceSetSpec{
			Replicas: func() *int32 { r := int32(1); return &r }(),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "its-0", Namespace: "ns"}}
	restart, _, err := hasConfigRestart(its, pod)
	require.NoError(t, err)
	assert.False(t, restart)
}
