meta: {
	clusterDefinition: {
		name: string
		type: string
	}

	cluster: {
		namespace: string
		name:      string
	}

	component: {
		name:         string
		type:         string
		configName:   string
		templateName: string
		configConstraintsName: string
	}
}

config: {
	apiVersion: "v1"
	kind:       "ConfigMap"
	metadata: {
		name:      meta.component.configName
		namespace: meta.cluster.namespace
		labels: {
			"app.kubernetes.io/name": "\(meta.clusterDefinition.type)-\(meta.clusterDefinition.name)"
			// cluster name
			"app.kubernetes.io/instance": meta.cluster.name
			// component name
			"app.kubernetes.io/component-name": "\(meta.component.name)"
			"app.kubernetes.io/created-by":     "controller-manager"

			// configmap selector for ConfigureController
			"configuration.kubeblocks.io/ins-configure": "true"
			"app.kubernetes.io/managed-by":    "kubeblocks"
			// config template name
			"configuration.kubeblocks.io/configurationtpl-name": "\(meta.component.templateName)"
			"configuration.kubeblocks.io/configuration-constraints-name": "\(meta.component.configConstraintsName)"
		}
		annotations: {
			// enable configmap upgrade
			"configuration.kubeblocks.io/rolling-upgrade": "true"
		}

		data: {
		}
	}
}
