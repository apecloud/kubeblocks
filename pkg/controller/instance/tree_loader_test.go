package instance

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func TestTreeLoaderSetsReader(t *testing.T) {
	reader := fake.NewClientBuilder().
		WithScheme(model.GetScheme()).
		WithObjects(&workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-instance",
			},
		}).
		Build()

	tree, err := NewTreeLoader().Load(
		context.Background(),
		reader,
		ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "test-instance"}},
		record.NewFakeRecorder(1),
		ctrl.Log.WithName("test"),
	)
	if err != nil {
		t.Fatalf("load tree: %v", err)
	}
	if tree.Reader == nil {
		t.Fatal("expected tree reader to be set")
	}
	if tree.Reader != reader {
		t.Fatal("expected tree reader to match loader reader")
	}
}
