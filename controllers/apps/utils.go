/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func validateComponentTemplate(cli client.Client, rctx intctrlutil.RequestCtx, compd *appsv1.ComponentDefinition) error {
	validateObject := func(objectKey client.ObjectKey) error {
		configObj := &corev1.ConfigMap{}
		return cli.Get(rctx.Ctx, objectKey, configObj)
	}
	validateTemplate := func(tpl appsv1.ComponentTemplateSpec) error {
		if tpl.TemplateRef != "" {
			return validateObject(client.ObjectKey{Namespace: tpl.Namespace, Name: tpl.TemplateRef})
		}
		return nil
	}
	for _, tpls := range [][]appsv1.ComponentTemplateSpec{compd.Spec.Configs, compd.Spec.Scripts} {
		for _, tpl := range tpls {
			if err := validateTemplate(tpl); err != nil {
				return err
			}
		}
	}
	return nil
}
