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
			"app.kubernetes.io/name":       "\(meta.clusterDefinition.name)"
			"app.kubernetes.io/instance":   meta.cluster.name
			"app.kubernetes.io/managed-by": "kubeblocks"

			"apps.kubeblocks.io/component-name": "\(meta.component.name)"
			// configmap selector for ConfigureController
			"config.kubeblocks.io/config-type": "instance"
			// config template name
			"config.kubeblocks.io/template-name": "\(meta.component.templateName)"
		}
		annotations: {
			// enable configmap upgrade
			"config.kubeblocks.io/disable-reconfigure": "false"
		}

		data: {
		}
	}
}
