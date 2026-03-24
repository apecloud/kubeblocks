package instanceset2

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestAddHeadlessServiceIsIdempotent(t *testing.T) {
	r := &headlessServiceReconciler{}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-its",
			Annotations: map[string]string{
				constant.KBAppMultiClusterPlacementKey: "data",
			},
		},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-its-headless",
		},
	}

	r.addHeadlessService(its, svc)
	r.addHeadlessService(its, svc)

	if got := len(its.Spec.InstanceAssistantObjects); got != 1 {
		t.Fatalf("expected 1 assistant object, got %d", got)
	}
	ref := its.Spec.InstanceAssistantObjects[0]
	if ref.Kind != "Service" || ref.Namespace != svc.Namespace || ref.Name != svc.Name {
		t.Fatalf("unexpected assistant object ref: %#v", ref)
	}
}
