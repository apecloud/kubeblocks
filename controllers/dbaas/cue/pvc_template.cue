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
			}
		]
	}
}
pvc_key: {
	name: string
	namespace: string
}
pvc: {
  kind: "PersistentVolumeClaim",
  apiVersion: "v1",
  metadata: {
    name: pvc_key.name
    namespace: pvc_key.namespace
  },
  spec: {
    accessModes: sts.spec.volumeClaimTemplates[0].spec.accessModes
    resources: sts.spec.volumeClaimTemplates[0].spec.resources
    dataSource: {
      "name": "\(sts.metadata.labels."app.kubernetes.io/instance")-scaling-auto-generated",
      "kind": "VolumeSnapshot",
      "apiGroup": "snapshot.storage.k8s.io"
    }
  }
}
