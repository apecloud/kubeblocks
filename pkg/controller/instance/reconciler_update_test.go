package instance

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestNewUpdateReconciler(t *testing.T) {
	r := NewUpdateReconciler()
	if r == nil {
		t.Fatal("NewUpdateReconciler() returned nil")
	}
	if _, ok := r.(*updateReconciler); !ok {
		t.Fatalf("expected *updateReconciler, got %T", r)
	}
}

func TestUpdatePreCondition(t *testing.T) {
	r := &updateReconciler{}

	tree := kubebuilderx.NewObjectTree()
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for nil root")
	}

	inst := buildStatusTestInstance()
	inst.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for deleting root")
	}

	inst = buildStatusTestInstance()
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); !result.Satisfied {
		t.Fatal("expected satisfied for normal root")
	}
}

func TestBuildBlockedCondition(t *testing.T) {
	inst := buildStatusTestInstance()
	message := "update blocked due to strict in-place policy"
	cond := buildBlockedCondition(inst, message)

	if cond.Type != string(workloads.InstanceUpdateRestricted) {
		t.Fatalf("expected type %s, got %s", workloads.InstanceUpdateRestricted, cond.Type)
	}
	if cond.Status != metav1.ConditionTrue {
		t.Fatalf("expected ConditionTrue, got %s", cond.Status)
	}
	if cond.Message != message {
		t.Fatalf("expected message %s, got %s", message, cond.Message)
	}
}

func TestUpdateReconcileNoPods(t *testing.T) {
	r := &updateReconciler{}
	inst := buildStatusTestInstance()

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()

	result, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Next != "Continue" {
		t.Fatalf("expected Continue, got %s", result.Next)
	}
}

func TestUpdateReconcileWithOnDeleteStrategy(t *testing.T) {
	r := &updateReconciler{}
	inst := buildStatusTestInstance()
	strategy := kbappsv1.OnDeleteStrategyType
	inst.Spec.InstanceUpdateStrategyType = &strategy

	pod := buildReadyPod(inst)
	pod.Labels[constant.KBAppInstanceNameLabelKey] = inst.Name

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()
	tree.EventRecorder = record.NewFakeRecorder(100)
	if err := tree.Add(pod); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	result, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Next != "Continue" {
		t.Fatalf("expected Continue, got %s", result.Next)
	}
}

func TestUpdateReconcileWithUnalignedPods(t *testing.T) {
	r := &updateReconciler{}
	inst := buildStatusTestInstance()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "different-name",
			Namespace: inst.Namespace,
		},
	}

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()
	if err := tree.Add(pod); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	result, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Next != "Continue" {
		t.Fatalf("expected Continue, got %s", result.Next)
	}
}

func TestSwitchoverNilLifecycle(t *testing.T) {
	r := &updateReconciler{}
	inst := buildStatusTestInstance()
	pod := buildReadyPod(inst)

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()

	err := r.switchover(tree, inst, pod)
	if err != nil {
		t.Fatalf("switchover() error = %v", err)
	}
}

func TestReconfigureNoConfigs(t *testing.T) {
	r := &updateReconciler{}
	inst := buildStatusTestInstance()
	pod := buildReadyPod(inst)

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()

	allUpdated, err := r.reconfigure(tree, inst, pod)
	if err != nil {
		t.Fatalf("reconfigure() error = %v", err)
	}
	if !allUpdated {
		t.Fatal("expected allUpdated=true when no configs to update")
	}
}

func TestReconfigureInstNilReconfigure(t *testing.T) {
	r := &updateReconciler{}
	inst := buildStatusTestInstance()
	inst.Spec.Configs = []workloads.ConfigTemplate{{
		Name:       "conf",
		ConfigHash: ptr.To("hash"),
	}}
	pod := buildReadyPod(inst)
	pod.Annotations = map[string]string{}
	pod.Annotations[constant.CMInsConfigurationHashLabelKey] = `{"conf":"old-hash"}`

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()

	_, err := r.reconfigure(tree, inst, pod)
	if err != nil {
		t.Fatalf("reconfigure() error = %v", err)
	}
}

func TestUpdateReconcileWithReadyPod(t *testing.T) {
	origIgnore := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	defer viper.Set(FeatureGateIgnorePodVerticalScaling, origIgnore)
	viper.Set(FeatureGateIgnorePodVerticalScaling, false)

	r := &updateReconciler{}
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("uid-update-test")).
		SetPodTemplate(corev1.PodTemplateSpec{
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
	inst.Generation = 1
	inst.Status.ObservedGeneration = 1

	desiredPod, err := buildInstancePod(inst, "")
	if err != nil {
		t.Fatalf("buildInstancePod() error = %v", err)
	}
	revision := getPodRevision(desiredPod)
	inst.Status.UpdateRevision = revision

	pod := buildReadyPod(inst)
	pod.Labels = desiredPod.Labels
	pod.Labels[constant.KBAppInstanceNameLabelKey] = inst.Name
	pod.Spec = desiredPod.Spec
	pod.Status.ContainerStatuses[0].Image = "mysql:8.0"

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()
	tree.EventRecorder = record.NewFakeRecorder(100)
	if err := tree.Add(pod); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	_, err = r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
}

func TestUpdateReconcilePendingPod(t *testing.T) {
	origIgnore := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	defer viper.Set(FeatureGateIgnorePodVerticalScaling, origIgnore)
	viper.Set(FeatureGateIgnorePodVerticalScaling, false)

	r := &updateReconciler{}
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("uid-pending-test")).
		SetPodTemplate(corev1.PodTemplateSpec{
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
	inst.Generation = 1
	inst.Status.ObservedGeneration = 1

	desiredPod, err := buildInstancePod(inst, "")
	if err != nil {
		t.Fatalf("buildInstancePod() error = %v", err)
	}
	revision := getPodRevision(desiredPod)
	inst.Status.UpdateRevision = revision

	pendingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      inst.Name,
			Namespace: inst.Namespace,
			Labels:    map[string]string{},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodPending},
	}
	pendingPod.Labels[constant.KBAppInstanceNameLabelKey] = inst.Name
	pendingPod.Labels["controller.kubernetes.io/revisionhash"] = "old-revision"

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()
	tree.EventRecorder = record.NewFakeRecorder(100)
	if err := tree.Add(pendingPod); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	_, err = r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	pods := tree.List(&corev1.Pod{})
	if len(pods) != 0 {
		t.Fatalf("expected 0 pods after pending pod deletion, got %d", len(pods))
	}
}

func TestNewLifecycleActionVar(t *testing.T) {
	orig := newLifecycleAction
	defer func() { newLifecycleAction = orig }()

	called := false
	newLifecycleAction = func(inst *workloads.Instance, pods []*corev1.Pod, pod *corev1.Pod) (lifecycle.Lifecycle, error) {
		called = true
		return nil, lifecycle.ErrActionNotDefined
	}

	inst := buildStatusTestInstance()
	inst.Spec.LifecycleActions = &workloads.LifecycleActions{
		Switchover: &workloads.Action{},
	}
	pod := buildReadyPod(inst)

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()

	r := &updateReconciler{}
	_ = r.switchover(tree, inst, pod)
	if !called {
		t.Fatal("expected newLifecycleAction to be called")
	}
}
