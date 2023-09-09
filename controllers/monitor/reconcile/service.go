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
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const OteldServiceName = "oteld-service"

func Service(reqCtx types.ReconcileCtx, params types.OTeldParams) error {
	var (
		k8sClient = params.Client
		namespace = viper.GetString(constant.MonitorNamespaceEnvName)
	)

	svc := buildSvcForOtel(namespace, OTeldName)
	exitingSvc := &corev1.Service{}
	err := k8sClient.Get(reqCtx.Ctx, client.ObjectKey{Name: OTeldName, Namespace: namespace}, exitingSvc)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			reqCtx.Log.Error(err, "failed to find secret", "secret", client.ObjectKeyFromObject(exitingSvc))
			params.Recorder.Eventf(exitingSvc, corev1.EventTypeWarning, "failed to find service", err.Error())
			return err
		}
		return k8sClient.Create(reqCtx.Ctx, svc)
	}

	reqCtx.Log.Info("updating existing secret", "secret", client.ObjectKeyFromObject(exitingSvc))
	return k8sClient.Update(reqCtx.Ctx, svc)
}
