package instance

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestDeletionPreCondition(t *testing.T) {
	r := &deletionReconciler{}

	// nil root
	tree := kubebuilderx.NewObjectTree()
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for nil root")
	}

	// not deleting
	inst := buildStatusTestInstance()
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for non-deleting root")
	}

	// deleting
	inst = buildStatusTestInstance()
	inst.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); !result.Satisfied {
		t.Fatal("expected satisfied for deleting root")
	}
}

func TestDeletionReconcileDeletesRoot(t *testing.T) {
	r := &deletionReconciler{reader: newFakeClient(t)}
	inst := buildStatusTestInstance()
	inst.DeletionTimestamp = &metav1.Time{Time: time.Now()}

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()

	_, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if tree.GetRoot() != nil {
		t.Fatal("expected root to be deleted")
	}
}

func TestDeletionReconcileDeletesSecondaryObjects(t *testing.T) {
	r := &deletionReconciler{reader: newFakeClient(t)}
	inst := buildStatusTestInstance()
	inst.DeletionTimestamp = &metav1.Time{Time: time.Now()}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      inst.Name,
			Namespace: inst.Namespace,
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
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
	if err := tree.Add(cm); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	_, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	pods := tree.List(&corev1.Pod{})
	if len(pods) != 0 {
		t.Fatalf("expected 0 pods after deletion, got %d", len(pods))
	}
}

func TestDeletionReconcileRetainsPVC(t *testing.T) {
	r := &deletionReconciler{reader: newFakeClient(t)}
	inst := buildStatusTestInstance()
	inst.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	inst.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{
		WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-mysql-0",
			Namespace: inst.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				UID: inst.UID,
			}},
		},
	}

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()
	if err := tree.Add(pvc); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	_, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	pvcs := tree.List(&corev1.PersistentVolumeClaim{})
	if len(pvcs) != 1 {
		t.Fatalf("expected 1 PVC retained in tree, got %d", len(pvcs))
	}
	retainedPVC := pvcs[0].(*corev1.PersistentVolumeClaim)
	if len(retainedPVC.OwnerReferences) != 0 {
		t.Fatalf("expected owner references to be cleared, got %d", len(retainedPVC.OwnerReferences))
	}
}

func TestDeletionReconcileScaledDown(t *testing.T) {
	r := &deletionReconciler{reader: newFakeClient(t)}
	inst := buildStatusTestInstance()
	inst.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	inst.Spec.ScaledDown = ptr.To(true)
	inst.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{
		WhenScaled: appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
	}

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()

	_, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
}

func TestAssistantObjectKey(t *testing.T) {
	// nil assistant object
	obj, name, err := assistantObjectKey(workloads.InstanceAssistantObject{})
	if err != nil {
		t.Fatalf("assistantObjectKey() error = %v", err)
	}
	if obj != nil || name != nil {
		t.Fatalf("expected nil for empty assistant object")
	}

	// with ConfigMap
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}
	obj, name, err = assistantObjectKey(workloads.InstanceAssistantObject{ConfigMap: cm})
	if err != nil {
		t.Fatalf("assistantObjectKey() error = %v", err)
	}
	if obj == nil || name == nil {
		t.Fatal("expected non-nil for ConfigMap assistant object")
	}
}

func TestSkipAssistantObjectSecondaryDeletion(t *testing.T) {
	inst := buildStatusTestInstance()

	// non-ordinal assistant object -> should skip deletion (return true)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-cm"},
	}
	if !skipAssistantObjectSecondaryDeletion(inst, cm) {
		t.Fatal("expected skip for non-ordinal assistant object")
	}

	// ordinal assistant object for current instance -> should not skip
	ordinalCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mysql-0",
			Annotations: map[string]string{
				constant.KBAppMultiClusterObjectProvisionPolicyKey: constant.KBAppMultiClusterObjectProvisionOrdinal,
			},
		},
	}
	inst.Name = "mysql-0"
	if skipAssistantObjectSecondaryDeletion(inst, ordinalCM) {
		t.Fatal("expected no skip for current ordinal assistant object")
	}

	// ordinal assistant object for different instance -> should skip
	otherOrdinalCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mysql-1",
			Annotations: map[string]string{
				constant.KBAppMultiClusterObjectProvisionPolicyKey: constant.KBAppMultiClusterObjectProvisionOrdinal,
			},
		},
	}
	if !skipAssistantObjectSecondaryDeletion(inst, otherOrdinalCM) {
		t.Fatal("expected skip for other ordinal assistant object")
	}
}

func TestSharedAssistantObjectReferencedByOthers(t *testing.T) {
	inst1 := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql-0", Namespace: "default"},
		Spec: workloads.InstanceSpec{
			InstanceAssistantObjects: []workloads.InstanceAssistantObject{{
				ConfigMap: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "shared-cm", Namespace: "default"},
				},
			}},
		},
	}
	inst2 := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql-1", Namespace: "default"},
		Spec: workloads.InstanceSpec{
			InstanceAssistantObjects: []workloads.InstanceAssistantObject{{
				ConfigMap: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "shared-cm", Namespace: "default"},
				},
			}},
		},
	}

	_, objKey, err := assistantObjectKey(inst1.Spec.InstanceAssistantObjects[0])
	if err != nil {
		t.Fatalf("assistantObjectKey() error = %v", err)
	}

	// referenced by another instance
	referenced, err := sharedAssistantObjectReferencedByOthers(inst1, *objKey, []workloads.Instance{*inst2})
	if err != nil {
		t.Fatalf("sharedAssistantObjectReferencedByOthers() error = %v", err)
	}
	if !referenced {
		t.Fatal("expected shared object to be referenced by inst2")
	}

	// not referenced by any other instance
	referenced, err = sharedAssistantObjectReferencedByOthers(inst1, *objKey, []workloads.Instance{})
	if err != nil {
		t.Fatalf("sharedAssistantObjectReferencedByOthers() error = %v", err)
	}
	if referenced {
		t.Fatal("expected shared object to not be referenced")
	}

	// referenced by a deleting instance
	inst2.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	referenced, err = sharedAssistantObjectReferencedByOthers(inst1, *objKey, []workloads.Instance{*inst2})
	if err != nil {
		t.Fatalf("sharedAssistantObjectReferencedByOthers() error = %v", err)
	}
	if referenced {
		t.Fatal("expected shared object to not be referenced by deleting instance")
	}
}
