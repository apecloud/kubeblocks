snapshot_name: string
pvc_name:      string
sts: {
	metadata: {
		labels: [string]: string
		namespace: string
	}
}

snapshot: {
	apiVersion: "snapshot.storage.k8s.io/v1"
	kind:       "VolumeSnapshot"
	metadata: {
		name:      snapshot_name
		namespace: sts.metadata.namespace
		labels:    sts.metadata.labels
	}
	spec: {
		source: {
			persistentVolumeClaimName: pvc_name
		}
	}
}
