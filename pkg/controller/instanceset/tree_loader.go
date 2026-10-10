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

package instanceset

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
)

type treeLoader struct{}

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

	if err = loadDataPVCs(ctx, reader, tree); err != nil {
		return nil, err
	}

	tree.Context = ctx
	tree.EventRecorder = recorder
	tree.Logger = logger
	tree.SetFinalizer(finalizer)

	return tree, err
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

func ownedKinds() []client.ObjectList {
	return []client.ObjectList{
		&corev1.ServiceList{},
		&corev1.ConfigMapList{},
		&corev1.PodList{},
		&corev1.PersistentVolumeClaimList{},
	}
}

func NewTreeLoader() kubebuilderx.TreeLoader {
	return &treeLoader{}
}

var _ kubebuilderx.TreeLoader = &treeLoader{}

// Direct PVC references are observations, not resources owned by this controller.
func loadDataPVCs(ctx context.Context, reader client.Reader, tree *kubebuilderx.ObjectTree) error {
	if tree.GetRoot() == nil {
		return nil
	}
	its := tree.GetRoot().(*workloads.InstanceSet)
	if !workloadlifecycle.HasData(its) {
		return nil
	}
	names := sets.New[string]()
	collect := func(volumes []corev1.Volume) {
		for _, volume := range volumes {
			if volume.Name == its.Spec.LifecycleActions.DataVolume && volume.PersistentVolumeClaim != nil {
				names.Insert(volume.PersistentVolumeClaim.ClaimName)
			}
		}
	}
	collect(its.Spec.Template.Spec.Volumes)
	for _, obj := range tree.List(&corev1.Pod{}) {
		collect(obj.(*corev1.Pod).Spec.Volumes)
	}
	for name := range names {
		pvc := &corev1.PersistentVolumeClaim{}
		pvc.Namespace = its.Namespace
		pvc.Name = name
		old, err := tree.Get(pvc)
		if err != nil {
			return err
		}
		if old != nil {
			continue
		}
		if err := reader.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if err := tree.AddWithOption(pvc, kubebuilderx.SkipToReconcile(true)); err != nil {
			return err
		}
	}
	return nil
}
