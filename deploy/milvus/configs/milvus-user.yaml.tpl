{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace   := $.cluster.metadata.namespace }}
{{- $userName := getEnvByName ( index $.podSpec.containers 0 ) "MINIO_ACCESS_KEY" }}
{{- $secret := getEnvByName ( index $.podSpec.containers 0 ) "MINIO_SECRET_KEY" | b64dec }}

etcd:
  endpoints:
  - {{$clusterName}}-etcd-headless.{{$namespace}}.svc.cluster.local:2379
  rootPath: {{$clusterName}}
messageQueue: rocksmq
minio:
  accessKeyID: minioadmin
  address: {{$clusterName}}-minio-headless.{{$namespace}}.svc.cluster.local
  bucketName: {{$clusterName}}
  port: 9000
  secretAccessKey: minioadmin
msgChannel:
  chanNamePrefix:
    cluster: {{$clusterName}}
  rocksmq: {}