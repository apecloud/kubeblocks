snapshot_key: {
	Name:      string
	Namespace: string
}
pvc_name: string
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
		labels: {
			"app.kubernetes.io/created-by": "kubeblocks"
			for k, v in sts.metadata.labels {
				"\(k)": "\(v)"
			}
		}
	}
	spec: {
		source: {
			persistentVolumeClaimName: pvc_name
		}
	}
}
