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

package instanceset

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/klauspost/compress/zstd"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	"github.com/apecloud/kubeblocks/pkg/testutil/k8s/mocks"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.
const (
	namespace       = "foo"
	name            = "bar"
	oldRevisionStr  = "old-revision"
	newRevisionStr  = "new-revision"
	minReadySeconds = 10
)

var (
	its         *workloads.InstanceSet
	priorityMap map[string]int
	reconciler  kubebuilderx.Reconciler
	controller  *gomock.Controller
	k8sMock     *mocks.MockClient
	ctx         context.Context
	logger      logr.Logger

	uid = types.UID("its-mock-uid")

	selectors = map[string]string{
		constant.AppInstanceLabelKey: name,
		WorkloadsManagedByLabelKey:   workloads.InstanceSetKind,
	}
	roles = []workloads.ReplicaRole{
		{
			Name:                 "leader",
			ParticipatesInQuorum: true,
			UpdatePriority:       5,
		},
		{
			Name:                 "follower",
			ParticipatesInQuorum: true,
			UpdatePriority:       4,
		},
		{
			Name:                 "logger",
			ParticipatesInQuorum: false,
			UpdatePriority:       3,
		},
		{
			Name:                 "learner",
			ParticipatesInQuorum: false,
			UpdatePriority:       2,
		},
	}
	pod = builder.NewPodBuilder("", "").
		AddContainer(corev1.Container{
			Name:  "foo",
			Image: "bar",
			Ports: []corev1.ContainerPort{
				{
					Name:          "my-svc",
					Protocol:      corev1.ProtocolTCP,
					ContainerPort: 12345,
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("300m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
		}).GetObject()
	template = corev1.PodTemplateSpec{
		ObjectMeta: pod.ObjectMeta,
		Spec:       pod.Spec,
	}

	volumeClaimTemplates = []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "data",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse("2G"),
					},
				},
			},
		},
	}
)

func makePodUpdateReady(newRevision string, roleful bool, pods ...*corev1.Pod) {
	readyCondition := corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	}
	for _, pod := range pods {
		pod.Labels[apps.StatefulSetRevisionLabel] = newRevision
		if roleful {
			if pod.Labels[RoleLabelKey] == "" {
				pod.Labels[RoleLabelKey] = "learner"
			}
		} else {
			delete(pod.Labels, RoleLabelKey)
		}
		pod.Status.Conditions = append(pod.Status.Conditions, readyCondition)
	}
}

func mockCompressedInstanceTemplates(ns, name string) (*corev1.ConfigMap, string, error) {
	instances := []workloads.InstanceTemplate{
		{
			Name:     "foo",
			Replicas: func() *int32 { r := int32(2); return &r }(),
		},
		{
			Name:     "bar0",
			Replicas: func() *int32 { r := int32(1); return &r }(),
		},
	}
	templateByte, err := json.Marshal(instances)
	if err != nil {
		return nil, "", err
	}
	writer, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, "", err
	}
	templateData := writer.EncodeAll(templateByte, nil)
	templateName := fmt.Sprintf("template-ref-%s", name)
	templateObj := builder.NewConfigMapBuilder(ns, templateName).
		SetBinaryData(map[string][]byte{
			templateRefDataKey: templateData,
		}).GetObject()
	templateRef := map[string]string{name: templateName}
	templateRefByte, err := json.Marshal(templateRef)
	if err != nil {
		return nil, "", err
	}
	return templateObj, string(templateRefByte), nil
}

func buildRandomPod() *corev1.Pod {
	randStr := rand.String(8)
	deadline := rand.Int63nRange(0, 1024*1024)
	randInt1 := rand.Int()
	randInt2 := rand.Int()
	return builder.NewPodBuilder(namespace, name).
		AddLabels(randStr, randStr).
		AddAnnotations(randStr, randStr).
		SetActiveDeadlineSeconds(&deadline).
		AddTolerations(corev1.Toleration{
			Key:      randStr,
			Operator: corev1.TolerationOpEqual,
			Value:    randStr,
		}).
		AddInitContainer(corev1.Container{
			Name:  "init-container",
			Image: randStr,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", randInt1)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dm", randInt2)),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", randInt1)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dm", randInt2)),
				},
			}}).
		AddContainer(corev1.Container{
			Name:  "container",
			Image: randStr,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", randInt1)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dm", randInt2)),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", randInt1)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dm", randInt2)),
				},
			}}).
		GetObject()
}

func getPodName(parent string, ordinal int) string {
	return fmt.Sprintf("%s-%d", parent, ordinal)
}

func init() {
	model.AddScheme(workloads.AddToScheme)
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "InstanceSet Suite")
}

var _ = BeforeSuite(func() {
	controller, k8sMock = testutil.SetupK8sMock()
	ctx = context.Background()
	logger = logf.FromContext(ctx).WithValues("instance-set", "test")

	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
	controller.Finish()
})

func getPodUpdatePolicyForTest(its *workloads.InstanceSet, pod *corev1.Pod) (podUpdatePolicy, workloads.PodUpdatePolicyType, string, error) {
	templates, err := instancetemplate.BuildInstanceTemplatesWithConfigs(its, nil)
	if err != nil {
		return noOpsPolicy, "", "", err
	}
	return getPodUpdatePolicy(its, pod, templates[pod.Name])
}

func newDirectConfigFixture(t *testing.T, layout string) (*workloads.InstanceSet, *kubebuilderx.ObjectTree, map[string]*corev1.Pod, string) {
	t.Helper()
	originalResizeSupport := intctrlutil.SupportResizeSubResource
	intctrlutil.SupportResizeSubResource = func() (bool, error) { return false, nil }
	t.Cleanup(func() { intctrlutil.SupportResizeSubResource = originalResizeSupport })
	its := legacyInstanceStatusSet(2)
	its.Spec.FlatInstanceOrdinal = layout == "compressed flat"
	its.Spec.PodManagementPolicy = apps.ParallelPodManagement
	its.Spec.PodUpdatePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
	its.Spec.PodUpgradePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
	action := &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"reload"}}}
	its.Spec.LifecycleActions = &workloads.LifecycleActions{Switchover: action}
	its.Spec.Configs = []workloads.ConfigTemplate{
		{Name: "main", ConfigHash: ptr.To("global"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "main"}, Reconfigure: action, Parameters: map[string]string{"id": "global-main"}},
		{Name: "aux", ConfigHash: ptr.To("global-aux"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "aux", ConfigMapName: "aux"}, Reconfigure: action, Parameters: map[string]string{"id": "global-aux"}},
	}
	its.Spec.Template.Spec.Volumes = []corev1.Volume{
		{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "main"}}}},
		{Name: "aux", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "aux"}}}},
	}
	tpl := workloads.InstanceTemplate{Name: "a", Replicas: ptr.To[int32](1), Configs: []workloads.ConfigTemplate{
		{Name: "main", ConfigHash: ptr.To("main-old"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "main-a"}, Reconfigure: action, Parameters: map[string]string{"id": "main"}},
		{Name: "aux", ConfigHash: ptr.To("aux-old"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "aux", ConfigMapName: "aux-a"}, Reconfigure: action, Parameters: map[string]string{"id": "aux"}},
	}}
	member := "demo-a-0"
	if its.Spec.FlatInstanceOrdinal {
		tpl.Ordinals = workloads.Ordinals{Discrete: []int32{7}}
		its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{0}}
		member = "demo-7"
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if layout == "named" {
		its.Spec.Instances = []workloads.InstanceTemplate{tpl}
	} else {
		its.Spec.Instances = nil
		setDirectCompressedTemplate(t, its, tree, tpl)
	}
	runDirectReconciler(t, NewValidationReconciler(), tree)
	runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
	runDirectReconciler(t, NewReplicasAlignmentReconciler(), tree)
	pods := map[string]*corev1.Pod{}
	for _, obj := range tree.List(&corev1.Pod{}) {
		pod := obj.(*corev1.Pod)
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		pods[pod.Name] = pod
	}
	if len(pods) != 2 || pods[member] == nil || pods["demo-0"] == nil {
		t.Fatalf("unexpected members: %v", pods)
	}
	runDirectReconciler(t, NewStatusReconciler(), tree)
	return its, tree, pods, member
}

func runDirectReconciler(t *testing.T, r kubebuilderx.Reconciler, tree *kubebuilderx.ObjectTree) {
	t.Helper()
	if _, err := r.Reconcile(tree); err != nil {
		t.Fatal(err)
	}
}

func setDirectCompressedTemplate(t *testing.T, its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree, tpl workloads.InstanceTemplate) {
	t.Helper()
	data, err := json.Marshal([]workloads.InstanceTemplate{tpl})
	if err != nil {
		t.Fatal(err)
	}
	writer, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "templates", Namespace: its.Namespace}, BinaryData: map[string][]byte{"instances": writer.EncodeAll(data, nil)}}
	its.Annotations = map[string]string{templateRefAnnotationKey: `{"demo":"templates"}`}
	if err := tree.Add(cm); err != nil {
		t.Fatal(err)
	}
}

func mutateDirectTemplate(t *testing.T, its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree, mutate func(*workloads.InstanceTemplate)) {
	t.Helper()
	if len(its.Spec.Instances) > 0 {
		mutate(&its.Spec.Instances[0])
		return
	}
	cmObj, err := tree.Get(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "templates", Namespace: its.Namespace}})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := reader.DecodeAll(cmObj.(*corev1.ConfigMap).BinaryData["instances"], nil)
	if err != nil {
		t.Fatal(err)
	}
	var templates []workloads.InstanceTemplate
	if err := json.Unmarshal(data, &templates); err != nil {
		t.Fatal(err)
	}
	mutate(&templates[0])
	setDirectCompressedTemplate(t, its, tree, templates[0])
}
