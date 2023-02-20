// Copyright ApeCloud, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

meta: {
	clusterDefinition: {
		name: string
	}

	cluster: {
		namespace: string
		name:      string
	}

	component: {
		name:                  string
		type:                  string
		characterType:         string
		configName:            string
		templateName:          string
		configConstraintsName: string
		configTemplateName:    string
	}
}

config: {
	apiVersion: "v1"
	kind:       "ConfigMap"
	metadata: {
		name:      meta.component.configName
		namespace: meta.cluster.namespace
		labels: {
			if meta.component.characterType != _|_ {
				"app.kubernetes.io/name": "\(meta.component.characterType)-\(meta.clusterDefinition.name)"
			}
			if meta.component.characterType == _|_ {
				"app.kubernetes.io/name": "\(meta.clusterDefinition.name)"
			}

			// cluster name
			"app.kubernetes.io/instance": meta.cluster.name
			// component name
			"app.kubernetes.io/component-name": "\(meta.component.name)"
			"app.kubernetes.io/created-by":     "controller-manager"
			"app.kubernetes.io/managed-by":     "kubeblocks"

			// configmap selector for ConfigureController
			"configuration.kubeblocks.io/configuration-type": "instance"
			// config template name
			"configuration.kubeblocks.io/configuration-tpl-name":         "\(meta.component.templateName)"
			"configuration.kubeblocks.io/configuration-constraints-name": "\(meta.component.configConstraintsName)"
			"configuration.kubeblocks.io/configtemplate-name":            "\(meta.component.configTemplateName)"
		}
		annotations: {
			// enable configmap upgrade
			"configuration.kubeblocks.io/disable-reconfigure": "false"
		}

		data: {
		}
	}
}
