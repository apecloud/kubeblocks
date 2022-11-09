pod: {
	metadata: {
		labels: {
			"statefulset.kubernetes.io/pod-name": string
			"app.kubernetes.io/name":             string
			"app.kubernetes.io/instance":         string
			"app.kubernetes.io/component-name":   string
		}
		name:      string
		namespace: string
	}
	spec: {
		containers: [...{
			ports: [...{
				containerPort: int
				name:          string
				protocol:      string
			}]
		}]
	}
}

service: {
	"apiVersion": "v1"
	"kind":       "Service"
	"metadata": {
		"name":      pod.metadata.name
		"namespace": pod.metadata.namespace
		labels: {
			"app.kubernetes.io/instance":       pod.metadata.labels."app.kubernetes.io/instance"
			"app.kubernetes.io/name":           pod.metadata.labels."app.kubernetes.io/name"
			"app.kubernetes.io/component-name": pod.metadata.labels."app.kubernetes.io/component-name"
			"app.kubernetes.io/managed-by":     "kubeblocks"
			// "app.kubernetes.io/version" : # TODO
		}
	}
	"spec": {
		"clusterIP": "None"
		"selector": {
			"statefulset.kubernetes.io/pod-name": pod.metadata.labels."statefulset.kubernetes.io/pod-name"
		}
		ports: [
			for _, container in pod.spec.containers
			for _, v in container.ports {
				name:       v.name
				protocol:   v.protocol
				port:       v.containerPort
				targetPort: v.containerPort
			},

		]
	}
}
