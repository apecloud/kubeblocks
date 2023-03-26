/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
