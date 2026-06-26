package instance

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestNewAlignmentReconciler(t *testing.T) {
	r := NewAlignmentReconciler()
	if r == nil {
		t.Fatal("NewAlignmentReconciler() returned nil")
	}
	if _, ok := r.(*alignmentReconciler); !ok {
		t.Fatalf("expected *alignmentReconciler, got %T", r)
	}
}

func TestAlignmentPreCondition(t *testing.T) {
	r := &alignmentReconciler{}

	tree := kubebuilderx.NewObjectTree()
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for nil root")
	}

	inst := buildStatusTestInstance()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); !result.Satisfied {
		t.Fatal("expected satisfied for normal root")
	}

	inst.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for deleting root")
	}
}

func TestAlignmentReconcileCreatesPod(t *testing.T) {
	r := &alignmentReconciler{}
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

	pods := tree.List(&corev1.Pod{})
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	pod := pods[0].(*corev1.Pod)
	if pod.Name != inst.Name {
		t.Fatalf("pod name = %s, want %s", pod.Name, inst.Name)
	}
}

func TestAlignmentReconcileCreatesPVCs(t *testing.T) {
	r := &alignmentReconciler{}
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("uid-align")).
		SetPodTemplate(corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
			},
		}).
		SetSelectorMatchLabels(map[string]string{"app": "mysql"}).
		SetInstanceSetName("mysql").
		AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}).
		GetObject()
	inst.Generation = 1

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()

	_, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	pvcs := tree.List(&corev1.PersistentVolumeClaim{})
	if len(pvcs) != 1 {
		t.Fatalf("expected 1 PVC, got %d", len(pvcs))
	}
}

func TestAlignmentReconcileDeletesOldPVCs(t *testing.T) {
	r := &alignmentReconciler{}
	inst := buildStatusTestInstance()

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()

	oldPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-pvc",
			Namespace: inst.Namespace,
		},
	}
	if err := tree.Add(oldPVC); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	_, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	pvcs := tree.List(&corev1.PersistentVolumeClaim{})
	if len(pvcs) != 0 {
		t.Fatalf("expected 0 PVCs after delete, got %d", len(pvcs))
	}
}

var _ = constant.AppManagedByLabelKey
