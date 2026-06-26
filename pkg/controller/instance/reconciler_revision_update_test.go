package instance

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestNewRevisionUpdateReconciler(t *testing.T) {
	r := NewRevisionUpdateReconciler()
	if r == nil {
		t.Fatal("NewRevisionUpdateReconciler() returned nil")
	}
	if _, ok := r.(*revisionUpdateReconciler); !ok {
		t.Fatalf("expected *revisionUpdateReconciler, got %T", r)
	}
}

func TestRevisionUpdatePreCondition(t *testing.T) {
	r := &revisionUpdateReconciler{}

	// nil root
	tree := kubebuilderx.NewObjectTree()
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for nil root")
	}

	// not updating (Generation == ObservedGeneration)
	inst := buildStatusTestInstance()
	inst.Status.ObservedGeneration = 1
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for non-updating root")
	}

	// updating (Generation != ObservedGeneration)
	inst = buildStatusTestInstance()
	inst.Status.ObservedGeneration = 0
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); !result.Satisfied {
		t.Fatal("expected satisfied for updating root")
	}
}

func TestRevisionUpdateReconcile(t *testing.T) {
	r := &revisionUpdateReconciler{}
	inst := buildStatusTestInstance()
	inst.Status.ObservedGeneration = 0

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)

	result, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Next != "Continue" {
		t.Fatalf("expected Continue, got %s", result.Next)
	}
	if inst.Status.UpdateRevision == "" {
		t.Fatal("expected UpdateRevision to be set")
	}
	if inst.Status.ObservedGeneration != inst.Generation {
		t.Fatalf("expected ObservedGeneration = %d, got %d", inst.Generation, inst.Status.ObservedGeneration)
	}
}

func TestRevisionUpdatePreConditionDeletingRoot(t *testing.T) {
	r := &revisionUpdateReconciler{}
	inst := buildStatusTestInstance()
	inst.Status.ObservedGeneration = 0
	inst.DeletionTimestamp = &metav1.Time{Time: time.Now()}

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); !result.Satisfied {
		t.Fatal("expected satisfied since revisionUpdateReconciler only checks IsObjectUsing, not deletion")
	}
}
