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
	"github.com/apecloud/kubeblocks/controllers/monitor/types"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const OteldSecretName = "oteld-secret"

func Secret(reqCtx types.ReconcileCtx, params types.OTeldParams) error {
	if reqCtx.Config.UseConfigMap {
		return nil
	}

	k8sClient := params.Client
	configData, err := reqCtx.GetOteldConfigYaml()
	if err != nil {
		return err
	}

	name := OTeldName
	namespace := viper.GetString(constant.MonitorNamespaceEnvName)
	secret := builder.NewSecretBuilder(namespace, name).
		AddLabels(constant.AppNameLabelKey, name).
		AddLabels(constant.KBManagedByKey, constant.AppName).
		SetStringData(map[string]string{}).PutData("config.yaml", configData).
		GetObject()

	exitingSecret := &corev1.Secret{}
	err = k8sClient.Get(reqCtx.Ctx, client.ObjectKey{Name: name, Namespace: namespace}, exitingSecret)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			reqCtx.Log.Error(err, "Failed to find secret", "secret", exitingSecret.Name)
			params.Recorder.Eventf(exitingSecret, corev1.EventTypeWarning, "Failed to find secret", err.Error())
			return err
		}
		return k8sClient.Create(reqCtx.Ctx, secret)
	}

	updatedSecret := exitingSecret.DeepCopy()
	updatedSecret.Labels = secret.Labels
	updatedSecret.StringData = secret.StringData
	reqCtx.Log.Info("updating existing secret", "secret", client.ObjectKeyFromObject(updatedSecret))
	return k8sClient.Update(reqCtx.Ctx, updatedSecret)

}
