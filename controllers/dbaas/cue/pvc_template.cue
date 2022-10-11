sts: {
	metadata: {
		labels: {
			"app.kubernetes.io/instance": string
		}
	}
	spec: {
		volumeClaimTemplates: [
			{
				spec: {
					accessModes: [string]
					resources: {}
				}
			},
		]
	}
}
pvc_key: {
	Name:      string
	Namespace: string
}
pvc: {
	kind:       "PersistentVolumeClaim"
	apiVersion: "v1"
	metadata: {
		name:      pvc_key.Name
		namespace: pvc_key.Namespace
	}
	spec: {
		accessModes: sts.spec.volumeClaimTemplates[0].spec.accessModes
		resources:   sts.spec.volumeClaimTemplates[0].spec.resources
		dataSource: {
			"name":     "\(sts.metadata.labels."app.kubernetes.io/instance")-scaling-auto-generated"
			"kind":     "VolumeSnapshot"
			"apiGroup": "snapshot.storage.k8s.io"
		}
	}
}
