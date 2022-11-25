snapshot_key: {
	Name: string
	Namespace: string
}
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
		name:      snapshot_key.Name
		namespace: snapshot_key.Namespace
		labels:    sts.metadata.labels
	}
	spec: {
		source: {
			persistentVolumeClaimName: pvc_name
		}
	}
}
