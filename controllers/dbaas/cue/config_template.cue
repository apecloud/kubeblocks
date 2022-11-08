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
	}
}

config: {
	apiVersion: "v1"
	kind:       "ConfigMap"
	metadata: {
		name:      meta.component.configName
		namespace: meta.cluster.namespace
		labels: {
			"app.kubernetes.io/name":     "\(meta.clusterDefinition.type)-\(meta.clusterDefinition.name)"
			"app.kubernetes.io/instance": meta.cluster.name
			// "app.kubernetes.io/version" : # TODO
			"app.kubernetes.io/component": "\(meta.component.type)-\(meta.component.name)"

			// config template name
			"app.kubernetes.io/configtemplate-name": "\(meta.component.templateName)"
			// configmap selector for ConfigureController
			"app.kubernetes.io/ins-configure": "true"
			"app.kubernetes.io/managed-by": "kubeblocks"
		}
		annotations: {
			// enable configmap upgrade
			"app.kubernetes.io/rolling-upgrade": "true"
		}

		data: {
		}
	}
}
