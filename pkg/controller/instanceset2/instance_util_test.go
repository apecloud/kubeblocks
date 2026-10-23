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

package instanceset2

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestParseParentNameAndOrdinal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantParent  string
		wantOrdinal int
	}{
		{name: "with ordinal", input: "mysql-leader-12", wantParent: "mysql-leader", wantOrdinal: 12},
		{name: "without dash", input: "mysql", wantParent: "mysql", wantOrdinal: -1},
		{name: "non numeric suffix", input: "mysql-leader", wantParent: "mysql-leader", wantOrdinal: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotParent, gotOrdinal := parseParentNameAndOrdinal(tt.input)
			if gotParent != tt.wantParent || gotOrdinal != tt.wantOrdinal {
				t.Fatalf("parseParentNameAndOrdinal() = %q, %d; want %q, %d",
					gotParent, gotOrdinal, tt.wantParent, tt.wantOrdinal)
			}
		})
	}
}

func TestSortObjects(t *testing.T) {
	pods := []*corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "mysql-0", Labels: map[string]string{constant.RoleLabelKey: "leader"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "mysql-2", Labels: map[string]string{constant.RoleLabelKey: "follower"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "mysql-1", Labels: map[string]string{constant.RoleLabelKey: "follower"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "mysql-unknown", Labels: map[string]string{}}},
	}
	rolePriorityMap := map[string]int{"": 0, "follower": 1, "leader": 2}

	sortObjects(pods, rolePriorityMap, false)
	got := []string{pods[0].Name, pods[1].Name, pods[2].Name, pods[3].Name}
	want := []string{"mysql-unknown", "mysql-2", "mysql-1", "mysql-0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted pods = %v, want %v", got, want)
	}

	sortObjects(pods, rolePriorityMap, true)
	got = []string{pods[0].Name, pods[1].Name, pods[2].Name, pods[3].Name}
	want = []string{"mysql-0", "mysql-1", "mysql-2", "mysql-unknown"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reverse sorted pods = %v, want %v", got, want)
	}
}

func TestSortInstancesNormalizesObservedRoleName(t *testing.T) {
	roles := []workloads.ReplicaRole{
		{Name: "Follower", UpdatePriority: 1},
		{Name: "Leader", UpdatePriority: 2},
	}
	priorityMap := composeRolePriorityMap(roles)
	instances := []workloads.Instance{
		{ObjectMeta: metav1.ObjectMeta{Name: "mysql-0"}, Status: workloads.InstanceStatus2{Role: "Follower"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "mysql-1"}, Status: workloads.InstanceStatus2{Role: "Leader"}},
	}

	sortInstances(instances, priorityMap, false)
	if got := instances[0].Name; got != "mysql-0" {
		t.Fatalf("sorted instances = %s, want mysql-0", got)
	}
}

func TestCopyAndMergeService(t *testing.T) {
	oldSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mysql",
			Namespace:   "default",
			Labels:      map[string]string{"old": "kept", "override": "old"},
			Annotations: map[string]string{"old": "kept"},
			Finalizers:  []string{"old-finalizer"},
			OwnerReferences: []metav1.OwnerReference{{
				UID:  types.UID("old-owner"),
				Name: "old",
			}},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"old": "selector"},
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    []corev1.ServicePort{{Name: "old", Port: 3306}},
		},
	}
	newSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mysql",
			Namespace:   "default",
			Labels:      map[string]string{"new": "added", "override": "new"},
			Annotations: map[string]string{"new": "added"},
			Finalizers:  []string{"new-finalizer"},
			OwnerReferences: []metav1.OwnerReference{{
				UID:  types.UID("new-owner"),
				Name: "new",
			}},
		},
		Spec: corev1.ServiceSpec{
			Selector:                 map[string]string{"app": "mysql"},
			Type:                     corev1.ServiceTypeNodePort,
			PublishNotReadyAddresses: true,
			Ports:                    []corev1.ServicePort{{Name: "new", Port: 3307}},
		},
	}

	got := copyAndMerge(oldSvc, newSvc).(*corev1.Service)

	if !reflect.DeepEqual(got.Finalizers, []string{"old-finalizer", "new-finalizer"}) {
		t.Fatalf("unexpected finalizers: %#v", got.Finalizers)
	}
	if len(got.OwnerReferences) != 2 {
		t.Fatalf("owner references = %d, want 2", len(got.OwnerReferences))
	}
	if !reflect.DeepEqual(got.Labels, map[string]string{"old": "kept", "new": "added", "override": "new"}) {
		t.Fatalf("unexpected labels: %#v", got.Labels)
	}
	if !reflect.DeepEqual(got.Annotations, map[string]string{"old": "kept", "new": "added"}) {
		t.Fatalf("unexpected annotations: %#v", got.Annotations)
	}
	if !reflect.DeepEqual(got.Spec.Selector, map[string]string{"app": "mysql"}) ||
		got.Spec.Type != corev1.ServiceTypeNodePort ||
		!got.Spec.PublishNotReadyAddresses ||
		!reflect.DeepEqual(got.Spec.Ports, []corev1.ServicePort{{Name: "new", Port: 3307}}) {
		t.Fatalf("unexpected service spec: %#v", got.Spec)
	}
	if oldSvc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Fatalf("copyAndMerge should not mutate old service, got type %s", oldSvc.Spec.Type)
	}
}

func TestCopyAndMerge(t *testing.T) {
	if got := copyAndMerge(&corev1.Service{}, &corev1.ConfigMap{}); got != nil {
		t.Fatalf("type mismatch should return nil, got %T", got)
	}

	newCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "new"}}
	if got := copyAndMerge(&corev1.ConfigMap{}, newCM); got != newCM {
		t.Fatalf("non-service object should return new object")
	}
}

func TestCopyAndMergeInstance(t *testing.T) {
	oldInst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mysql-0",
			Labels:      map[string]string{"old": "kept"},
			Annotations: map[string]string{"old": "kept"},
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
				},
			},
			InstanceSetName: "mysql",
		},
	}
	newInst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mysql-0",
			Labels:      map[string]string{"new": "added"},
			Annotations: map[string]string{"new": "added"},
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.4"}},
				},
			},
			Selector:             &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mysql"}},
			MinReadySeconds:      10,
			InstanceSetName:      "mysql",
			InstanceTemplateName: "az-a",
			Configs:              []workloads.ConfigTemplate{{Name: "mysql-conf"}},
		},
	}

	got := copyAndMergeInstance(oldInst, newInst)
	if got == nil {
		t.Fatal("expected merged instance")
		return
	}
	if got.Spec.Template.Spec.Containers[0].Image != "mysql:8.4" {
		t.Fatalf("unexpected merged image: %s", got.Spec.Template.Spec.Containers[0].Image)
	}
	if !reflect.DeepEqual(got.Labels, map[string]string{"old": "kept", "new": "added"}) {
		t.Fatalf("unexpected merged labels: %#v", got.Labels)
	}
	if !reflect.DeepEqual(got.Annotations, map[string]string{"old": "kept", "new": "added"}) {
		t.Fatalf("unexpected merged annotations: %#v", got.Annotations)
	}

	if got := copyAndMergeInstance(got, got.DeepCopy()); got != nil {
		t.Fatalf("unchanged instance should return nil, got %#v", got)
	}
}

func TestGetInstanceTemplateMap(t *testing.T) {
	got, err := getInstanceTemplateMap(nil)
	if err != nil || got != nil {
		t.Fatalf("nil annotations = %#v, %v; want nil, nil", got, err)
	}

	got, err = getInstanceTemplateMap(map[string]string{"other": "value"})
	if err != nil || got != nil {
		t.Fatalf("missing template annotation = %#v, %v; want nil, nil", got, err)
	}

	got, err = getInstanceTemplateMap(map[string]string{
		templateRefAnnotationKey: `{"mysql-0":"az-a","mysql-1":"az-b"}`,
	})
	if err != nil {
		t.Fatalf("getInstanceTemplateMap() error = %v", err)
	}
	if !reflect.DeepEqual(got, map[string]string{"mysql-0": "az-a", "mysql-1": "az-b"}) {
		t.Fatalf("unexpected template map: %#v", got)
	}

	_, err = getInstanceTemplateMap(map[string]string{templateRefAnnotationKey: "not-json"})
	if err == nil {
		t.Fatal("expected invalid json error")
	}
}

func TestGetHeadlessSvcName(t *testing.T) {
	if got := getHeadlessSvcName("mysql"); got != "mysql-headless" {
		t.Fatalf("getHeadlessSvcName() = %q, want mysql-headless", got)
	}
}

func TestLifecycleRuntimeDiscoversRetainedPVCIdentity(t *testing.T) {
	for _, loaded := range []bool{false, true} {
		for _, tc := range []struct {
			name          string
			mutate        func(*corev1.PersistentVolumeClaim)
			missing, fail bool
			want          bool
		}{
			{name: "ownerless retained", want: true},
			{name: "different namespace", mutate: func(p *corev1.PersistentVolumeClaim) { p.Namespace = "other" }},
			{name: "different identity", mutate: func(p *corev1.PersistentVolumeClaim) { p.Labels[constant.KBAppInstanceNameLabelKey] = "foreign" }},
			{name: "different pod", mutate: func(p *corev1.PersistentVolumeClaim) { p.Labels[constant.KBAppPodNameLabelKey] = "foreign" }},
			{name: "foreign owner", mutate: func(p *corev1.PersistentVolumeClaim) {
				p.OwnerReferences = []metav1.OwnerReference{{APIVersion: workloads.GroupVersion.String(), Kind: "Instance", Name: "foreign", UID: "foreign"}}
			}},
			{name: "missing claim", missing: true},
			{name: "lookup failure", fail: true},
		} {
			t.Run(fmt.Sprintf("loaded=%t/%s", loaded, tc.name), func(t *testing.T) {
				ctx := context.Background()
				name := "demo-named-1"
				its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "data-a,data-b"}}, Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](0), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}}, LifecycleActions: &workloads.LifecycleActions{DataDump: &kbappsv1.Action{}, DataLoad: &kbappsv1.Action{}}}, Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{PodName: name, TemplateName: ptr.To("removed"), DataLoaded: ptr.To(loaded)}}}}
				claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "old-template-claim", Namespace: its.Namespace, UID: "retained", Labels: map[string]string{constant.AppManagedByLabelKey: constant.AppName, constant.KBAppInstanceNameLabelKey: name, constant.KBAppPodNameLabelKey: name}}}
				if tc.mutate != nil {
					tc.mutate(claim)
				}
				builder := fake.NewClientBuilder().WithScheme(model.GetScheme())
				if !tc.missing {
					builder.WithObjects(claim)
				}
				failure := errors.New("claim lookup unavailable")
				queries := 0
				reader := builder.WithInterceptorFuncs(interceptor.Funcs{List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					if _, ok := list.(*corev1.PersistentVolumeClaimList); ok {
						queries++
						placement, err := multicluster.FromContext(ctx)
						if err != nil || placement != "data-b" {
							t.Fatalf("wrong retained data placement %q %v", placement, err)
						}
						options := (&client.ListOptions{}).ApplyOptions(opts)
						if options.Namespace != its.Namespace {
							t.Fatalf("wrong namespace %q", options.Namespace)
						}
						dataContext := false
						for _, opt := range opts {
							if reflect.DeepEqual(opt, multicluster.InDataContext()) {
								dataContext = true
							}
						}
						if !dataContext {
							t.Fatal("retained PVC lookup omitted data-context routing")
						}
						if tc.fail {
							return failure
						}
					}
					return c.List(ctx, list, opts...)
				}}).Build()
				tree := kubebuilderx.NewObjectTree()
				tree.SetRoot(its)
				tree.Context = ctx
				err := loadLifecycleRuntime(ctx, reader, tree)
				if queries != 1 {
					t.Fatalf("retained identity lookup count %d", queries)
				}
				if tc.fail {
					if !errors.Is(err, failure) {
						t.Fatalf("lookup error lost: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				claimsByInstance := tree.Context.Value(lifecyclePVCsKey{}).(map[string][]*corev1.PersistentVolumeClaim)
				if (len(claimsByInstance[name]) > 0) != tc.want {
					t.Fatalf("claims=%v want=%t", claimsByInstance, tc.want)
				}
				if len(tree.List(&corev1.PersistentVolumeClaim{})) != 0 {
					t.Fatal("parent took mutable ownership of observed claims")
				}
				if *its.FindInstanceStatus(name).DataLoaded != loaded {
					t.Fatal("resource presence changed recorded result")
				}
			})
		}
	}
}
