/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package configuration

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// GetParameterFromConfiguration get configure parameter
// ctx: apiserver context
// cli: apiserver client
// cluster: appsv1alpha1.Cluster
// component: component name
func GetParameterFromConfiguration(configMap *corev1.ConfigMap, allFiles bool, fieldPath ...string) ([]string, error) {
	if configMap == nil || len(configMap.Data) == 0 {
		return nil, MakeError("configmap not any configuration files. [%v]", configMap)
	}

	// Load configmap
	wrapCfg, err := NewConfigLoader(CfgOption{
		Type:           CfgCmType,
		Log:            log.FromContext(context.Background()),
		CfgType:        appsv1alpha1.Ini,
		ConfigResource: FromConfigData(configMap.Data, nil),
	})
	if err != nil {
		return nil, WrapError(err, "failed to loader configmap")
	}

	res := make([]string, 0, len(fieldPath))
	option := NewCfgOptions("")
	option.AllSearch = allFiles
	for _, field := range fieldPath {
		if rs, err := wrapCfg.Query(field, option); err != nil {
			return nil, WrapError(err, "failed to get parameter:[%s]", field)
		} else {
			res = append(res, string(rs))
		}
	}

	return res, nil
}
