package instance

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"k8s.io/client-go/tools/record"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestNewTreeLoader(t *testing.T) {
	tl := NewTreeLoader()
	if tl == nil {
		t.Fatal("NewTreeLoader() returned nil")
	}
	if _, ok := tl.(*treeLoader); !ok {
		t.Fatalf("expected *treeLoader, got %T", tl)
	}
}

func TestOwnedKinds(t *testing.T) {
	kinds := ownedKinds()
	if len(kinds) != 7 {
		t.Fatalf("expected 7 owned kinds, got %d", len(kinds))
	}
	if _, ok := kinds[0].(*corev1.PodList); !ok {
		t.Fatalf("expected kinds[0] to be *corev1.PodList, got %T", kinds[0])
	}
	if _, ok := kinds[1].(*corev1.PersistentVolumeClaimList); !ok {
		t.Fatalf("expected kinds[1] to be *corev1.PersistentVolumeClaimList, got %T", kinds[1])
	}
	if _, ok := kinds[2].(*corev1.ConfigMapList); !ok {
		t.Fatalf("expected kinds[2] to be *corev1.ConfigMapList, got %T", kinds[2])
	}
	if _, ok := kinds[3].(*corev1.SecretList); !ok {
		t.Fatalf("expected kinds[3] to be *corev1.SecretList, got %T", kinds[3])
	}
	if _, ok := kinds[4].(*corev1.ServiceAccountList); !ok {
		t.Fatalf("expected kinds[4] to be *corev1.ServiceAccountList, got %T", kinds[4])
	}
	if _, ok := kinds[5].(*rbacv1.RoleList); !ok {
		t.Fatalf("expected kinds[5] to be *rbacv1.RoleList, got %T", kinds[5])
	}
	if _, ok := kinds[6].(*rbacv1.RoleBindingList); !ok {
		t.Fatalf("expected kinds[6] to be *rbacv1.RoleBindingList, got %T", kinds[6])
	}
}

func TestTreeLoaderLoad(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("uid-tree-test")).
		SetPodTemplate(corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
			},
		}).
		SetSelectorMatchLabels(map[string]string{"app": "mysql"}).
		SetInstanceSetName("mysql").
		GetObject()
	inst.Labels = map[string]string{
		constant.AppManagedByLabelKey:      constant.AppName,
		constant.KBAppInstanceNameLabelKey: "mysql-0",
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-0",
			Namespace: "default",
			Labels: map[string]string{
				constant.AppManagedByLabelKey:      constant.AppName,
				constant.KBAppInstanceNameLabelKey: "mysql-0",
			},
		},
	}

	cli := buildTreeLoaderFakeClient(t, inst, pod)
	tl := NewTreeLoader()

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "mysql-0"}}
	tree, err := tl.Load(context.Background(), cli, req, record.NewFakeRecorder(100), testLogger())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if tree == nil {
		t.Fatal("expected non-nil tree")
	}

	root := tree.GetRoot()
	if root == nil {
		t.Fatal("expected non-nil root")
	}
	rootInst, ok := root.(*workloads.Instance)
	if !ok {
		t.Fatalf("expected *workloads.Instance, got %T", root)
	}
	if rootInst.Name != "mysql-0" {
		t.Fatalf("root name = %s, want mysql-0", rootInst.Name)
	}

	pods := tree.List(&corev1.Pod{})
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod in tree, got %d", len(pods))
	}
}

func TestTreeLoaderLoadWithAssistantObjects(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("uid-tree-cm")).
		SetPodTemplate(corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
			},
		}).
		SetSelectorMatchLabels(map[string]string{"app": "mysql"}).
		SetInstanceSetName("mysql").
		SetInstanceAssistantObjects([]workloads.InstanceAssistantObject{{ConfigMap: cm}}).
		GetObject()
	inst.Labels = map[string]string{
		constant.AppManagedByLabelKey:      constant.AppName,
		constant.KBAppInstanceNameLabelKey: "mysql-0",
	}

	cli := buildTreeLoaderFakeClient(t, inst, cm)
	tl := NewTreeLoader()

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "mysql-0"}}
	tree, err := tl.Load(context.Background(), cli, req, record.NewFakeRecorder(100), testLogger())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if tree == nil {
		t.Fatal("expected non-nil tree")
	}

	cms := tree.List(&corev1.ConfigMap{})
	if len(cms) != 1 {
		t.Fatalf("expected 1 ConfigMap in tree, got %d", len(cms))
	}
}

func TestLoadAssistantObjectsNilRoot(t *testing.T) {
	tree := kubebuilderxNewObjectTree()
	if err := loadAssistantObjects(context.Background(), nil, tree); err != nil {
		t.Fatalf("loadAssistantObjects() with nil root error = %v", err)
	}
}

func buildTreeLoaderFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add rbac scheme: %v", err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatalf("add workloads scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func kubebuilderxNewObjectTree() *kubebuilderx.ObjectTree {
	return kubebuilderx.NewObjectTree()
}
