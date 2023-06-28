apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name:
spec:
  componentSpecs:
    - name: mysql # user-defined
      componentDefRef: mysql # ref clusterdefinition componentDefs.name
      monitor: false
      replicas: 1
      serviceAccountName: kb-release-name-apecloud-mysql-cluster
      enabledLogs:     ["slow","error"]
      volumeClaimTemplates:
        - name: data # ref clusterdefinition components.containers.volumeMounts.name
          spec:
            storageClassName:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi
    {{- $withProxy := and (eq .mode "raftGroup") .parameters.proxyEnabled -}}
    {{- if $withProxy }}
    - name: etcd
      componentDefRef: etcd # ref clusterdefinition componentDefs.name
      replicas: 1
    - name: vtctld
      componentDefRef: vtctld # ref clusterdefinition componentDefs.name
      replicas: 1
    - name: vtconsensus
      componentDefRef: vtconsensus # ref clusterdefinition componentDefs.name
      replicas: 1
    - name: vtgate
      componentDefRef: vtgate # ref clusterdefinition componentDefs.name
      replicas: 1
    {{- end }}