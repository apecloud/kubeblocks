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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func setCompOwnershipNFinalizer(comp *appsv1.Component, object client.Object) error {
	if skipSetCompOwnershipNFinalizer(object) {
		return nil
	}
	// add finalizer to the object
	addFinalizer(object, comp)
	if err := intctrlutil.SetOwnership(comp, object, rscheme, ""); err != nil {
		if _, ok := err.(*controllerutil.AlreadyOwnedError); ok {
			return nil
		}
		return err
	}
	return nil
}

// skipSetCompOwnershipNFinalizer returns true if the object should not be set ownership to the component
func skipSetCompOwnershipNFinalizer(obj client.Object) bool {
	switch obj.(type) {
	case *corev1.PersistentVolume, *corev1.PersistentVolumeClaim, *corev1.Pod:
		return true
	default:
		return false
	}
}

func addFinalizer(obj client.Object, comp *appsv1.Component) {
	controllerutil.AddFinalizer(obj, constant.DBComponentFinalizerName)
}
