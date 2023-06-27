// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
		compDefName:           string
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
			"app.kubernetes.io/component":  "\(meta.component.compDefName)"

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
