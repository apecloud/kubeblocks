package instance

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestNewFixMetaReconciler(t *testing.T) {
	r := NewFixMetaReconciler()
	if r == nil {
		t.Fatal("NewFixMetaReconciler() returned nil")
	}
	if _, ok := r.(*fixMetaReconciler); !ok {
		t.Fatalf("expected *fixMetaReconciler, got %T", r)
	}
}

func TestFixMetaPreCondition(t *testing.T) {
	r := &fixMetaReconciler{}

	// nil root
	tree := kubebuilderx.NewObjectTree()
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for nil root")
	}

	// deleting root
	inst := buildStatusTestInstance()
	inst.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for deleting root")
	}

	// has finalizer
	inst = buildStatusTestInstance()
	controllerutil.AddFinalizer(inst, finalizer)
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for root with finalizer")
	}

	// no finalizer, not deleting
	inst = buildStatusTestInstance()
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); !result.Satisfied {
		t.Fatal("expected satisfied for root without finalizer")
	}
}

func TestFixMetaReconcile(t *testing.T) {
	r := &fixMetaReconciler{}
	inst := buildStatusTestInstance()

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)

	result, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Next != "Commit" {
		t.Fatalf("expected Commit result, got %s", result.Next)
	}
	if !controllerutil.ContainsFinalizer(inst, finalizer) {
		t.Fatal("expected finalizer to be added")
	}
}
