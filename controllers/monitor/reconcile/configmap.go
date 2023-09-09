/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package reconcile

import (
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/monitor/types"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
)

const OteldConfigMapName = "oteld-configmap"

func ConfigMap(reqCtx types.ReconcileCtx, params types.OTeldParams) error {
	if !reqCtx.Config.UseConfigMap {
		return nil
	}

	k8sClient := params.Client
	configData, err := reqCtx.GetOteldConfigYaml()
	if err != nil {
		return err
	}

	name := OteldConfigMapName
	namespace := viper.GetString(constant.MonitorNamespaceEnvName)
	exitingConfigMap := &corev1.ConfigMap{}
	err = k8sClient.Get(reqCtx.Ctx, client.ObjectKey{Name: name, Namespace: namespace}, exitingConfigMap)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		configmap := builder.NewConfigMapBuilder(namespace, name).
			SetData(map[string]string{"config.yaml": string(configData)}).
			GetObject()
		return k8sClient.Create(reqCtx.Ctx, configmap)
	}

	updatedConfigmap := exitingConfigMap.DeepCopy()
	updatedConfigmap.Data["config.yaml"] = string(configData)
	return k8sClient.Update(reqCtx.Ctx, updatedConfigmap)
}
