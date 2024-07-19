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

package apps

import (
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentOwnershipTransformer adds finalizer to all none component objects
type componentOwnershipTransformer struct{}

var _ graph.Transformer = &componentOwnershipTransformer{}

func (f *componentOwnershipTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp := transCtx.Component
	return setCompOwnership(comp, dag, graphCli)
}

func setCompOwnership(comp *appsv1alpha1.Component, dag *graph.DAG, graphCli model.GraphClient) error {
	// find all objects that are not component and set ownership to the component
	objects := graphCli.FindAll(dag, &appsv1alpha1.Component{}, &model.HaveDifferentTypeWithOption{})
	for _, object := range objects {
		if skipSetCompOwnership(object) {
			continue
		}
		// add finalizer to the object
		addFinalizer(object, comp)
		if err := intctrlutil.SetOwnership(comp, object, rscheme, ""); err != nil {
			if _, ok := err.(*controllerutil.AlreadyOwnedError); ok {
				continue
			}
			return err
		}
	}
	return nil
}

// skipSetCompOwnership returns true if the object should not be set ownership to the component
func skipSetCompOwnership(obj client.Object) bool {
	switch obj.(type) {
	case *rbacv1.ClusterRoleBinding, *corev1.PersistentVolume, *corev1.PersistentVolumeClaim, *corev1.Pod:
		return true
	default:
		return false
	}
}

func addFinalizer(obj client.Object, comp *appsv1alpha1.Component) {
	if skipAddCompFinalizer(obj, comp) {
		return
	}
	controllerutil.AddFinalizer(obj, constant.DBComponentFinalizerName)
}

func skipAddCompFinalizer(obj client.Object, comp *appsv1alpha1.Component) bool {
	// Due to compatibility reasons, the component controller creates cluster-scoped RoleBinding and ServiceAccount objects in the following two scenarios:
	// 1. When the user does not specify a ServiceAccount, KubeBlocks automatically creates a ServiceAccount and a RoleBinding with named pattern kb-{cluster.Name}.
	// 2. When the user specifies a ServiceAccount that does not exist, KubeBlocks will automatically create a ServiceAccount and a RoleBinding with the same name.
	// In both cases, the lifecycle of the RoleBinding and ServiceAccount should not be tied to the component.
	skipTypes := []interface{}{
		&rbacv1.RoleBinding{},
		&corev1.ServiceAccount{},
	}

	for _, t := range skipTypes {
		if objType, ok := obj.(interface{ GetName() string }); ok && reflect.TypeOf(obj) == reflect.TypeOf(t) {
			if !strings.HasPrefix(objType.GetName(), constant.GenerateDefaultServiceAccountName(comp.GetName())) {
				return true
			}
		}
	}
	return false
}

// errPrematureStopWithSetCompOwnership is a helper function that sets component ownership and returns graph.ErrPrematureStop
// TODO: remove this function to independent component transformer utils
func errPrematureStopWithSetCompOwnership(comp *appsv1alpha1.Component, dag *graph.DAG, graphCli model.GraphClient) error {
	err := setCompOwnership(comp, dag, graphCli)
	if err != nil {
		return err
	}
	return graph.ErrPrematureStop
}

// errWithSetCompOwnership is a helper function that sets component ownership before returns err
// TODO: refactor to set ownership information when creating each object, instead of setting it uniformly.
func errWithSetCompOwnership(comp *appsv1alpha1.Component, dag *graph.DAG, cli client.Reader, err error) error {
	graphCli, _ := cli.(model.GraphClient)
	_ = setCompOwnership(comp, dag, graphCli)
	return err
}
