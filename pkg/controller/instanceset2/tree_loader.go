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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
)

func NewTreeLoader() kubebuilderx.TreeLoader {
	return &treeLoader{}
}

type treeLoader struct{}

var _ kubebuilderx.TreeLoader = &treeLoader{}

func (r *treeLoader) Load(ctx context.Context, reader client.Reader, req ctrl.Request, recorder record.EventRecorder, logger logr.Logger) (*kubebuilderx.ObjectTree, error) {
	ml := getMatchLabels(req.Name)
	kinds := ownedKinds()
	tree, err := kubebuilderx.ReadObjectTree[*workloads.InstanceSet](ctx, reader, req, ml, kinds...)
	if err != nil {
		return nil, err
	}

	// load compressed instance templates if present
	if err = loadCompressedInstanceTemplates(ctx, reader, tree); err != nil {
		return nil, err
	}

	// load assistant objects
	if err = loadInstanceAssistantObjects(ctx, reader, tree); err != nil {
		return nil, err
	}

	tree.Context = context.WithValue(ctx, lifecycleReaderKey{}, reader)
	if err = loadLifecycleRuntime(ctx, reader, tree); err != nil {
		return nil, err
	}
	tree.EventRecorder = recorder
	tree.Logger = logger
	tree.SetFinalizer(finalizer)

	return tree, err
}

func ownedKinds() []client.ObjectList {
	return []client.ObjectList{
		&workloads.InstanceList{},
		&corev1.ServiceList{},
	}
}

func loadCompressedInstanceTemplates(ctx context.Context, reader client.Reader, tree *kubebuilderx.ObjectTree) error {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return nil
	}
	templateMap, err := getInstanceTemplateMap(tree.GetRoot().GetAnnotations())
	if err != nil {
		return err
	}
	ns := tree.GetRoot().GetNamespace()
	for _, templateName := range templateMap {
		template := &corev1.ConfigMap{}
		if err := reader.Get(ctx, types.NamespacedName{Namespace: ns, Name: templateName}, template); err != nil {
			return err
		}
		if err := tree.Add(template); err != nil {
			return err
		}
	}
	return nil
}

// Runtime resources belong to Instance. They are observations only in the parent tree.
func loadLifecycleRuntime(ctx context.Context, reader client.Reader, tree *kubebuilderx.ObjectTree) error {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return nil
	}
	its := tree.GetRoot().(*workloads.InstanceSet)
	if !workloadlifecycle.Enabled(its) {
		return nil
	}
	ctx = multicluster.IntoContext(ctx, its.Annotations[constant.KBAppMultiClusterPlacementKey])
	desired, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		if instancetemplate.IsAssignmentIncomplete(err) {
			return nil
		}
		return err
	}
	instances := map[string]*workloads.Instance{}
	for _, obj := range tree.List(&workloads.Instance{}) {
		inst := obj.(*workloads.Instance)
		instances[inst.Name] = inst
	}
	names := sets.New[string]()
	for name := range desired {
		names.Insert(name)
	}
	for name := range instances {
		names.Insert(name)
	}
	for _, status := range its.Status.InstanceStatus {
		names.Insert(status.PodName)
	}
	for _, name := range sets.List(names) {
		inst := instances[name]
		if inst == nil {
			inst = desired[name]
		}
		if inst == nil {
			inst = &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: its.Namespace}}
		}
		_, ordinal := parseParentNameAndOrdinal(name)
		inst = multicluster.Assign(ctx, inst.DeepCopy(), func() int { return max(0, ordinal) }).(*workloads.Instance)
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: its.Namespace, Annotations: inst.Annotations}}
		if err := reader.Get(ctx, client.ObjectKeyFromObject(pod), pod, multicluster.InDataContext()); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
			continue
		} else if err := tree.AddWithOption(pod, kubebuilderx.SkipToReconcile(true)); err != nil {
			return err
		}
	}
	return nil
}
