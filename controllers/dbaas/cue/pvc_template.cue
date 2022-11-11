sts: {
	metadata: {
		labels: [string]: string
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
snapshot_name: string
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
		labels:    sts.metadata.labels
	}
	spec: {
		accessModes: sts.spec.volumeClaimTemplates[0].spec.accessModes
		resources:   sts.spec.volumeClaimTemplates[0].spec.resources
		dataSource: {
			"name":     snapshot_name
			"kind":     "VolumeSnapshot"
			"apiGroup": "snapshot.storage.k8s.io"
		}
	}
}
