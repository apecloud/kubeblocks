/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type treeLoader struct{}

func (r *treeLoader) Load(ctx context.Context, reader client.Reader, req ctrl.Request, recorder record.EventRecorder, logger logr.Logger) (*kubebuilderx.ObjectTree, error) {
	keys := getMatchLabelKeys()
	kinds := ownedKinds()
	tree, err := kubebuilderx.ReadObjectTree[*workloads.InstanceSet](ctx, reader, req, keys, kinds...)
	if err != nil {
		return nil, err
	}

	// load compressed instance templates if present
	if err = loadCompressedInstanceTemplates(ctx, reader, tree); err != nil {
		return nil, err
	}

	tree.EventRecorder = recorder
	tree.Logger = logger

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

func getMatchLabelKeys() []string {
	if viper.GetBool(rsm.FeatureGateRSMCompatibilityMode) {
		return []string{
			constant.AppManagedByLabelKey,
			constant.AppNameLabelKey,
			constant.AppComponentLabelKey,
			constant.AppInstanceLabelKey,
			constant.KBAppComponentLabelKey,
		}
	}
	return []string{
		rsm.WorkloadsManagedByLabelKey,
		rsm.WorkloadsInstanceLabelKey,
	}
}

func ownedKinds() []client.ObjectList {
	return []client.ObjectList{
		&corev1.ServiceList{},
		&corev1.ConfigMapList{},
		&corev1.PodList{},
		&corev1.PersistentVolumeClaimList{},
		&batchv1.JobList{},
	}
}

func NewTreeLoader() kubebuilderx.TreeLoader {
	return &treeLoader{}
}

var _ kubebuilderx.TreeLoader = &treeLoader{}
