apiVersion: kubekey.kubesphere.io/v1alpha2
kind: Cluster
metadata:
  name: {{ $.Name }}
spec:
  hosts:
  {{- range $.Hosts }}
  - name: {{ .Name }}
    address: {{ .Address }}
    internalAddress: {{ .InternalAddress }}
    user: {{ $.User }}
    password: {{ $.Password }}
    privateKey: {{ $.PrivateKey | quote }}
    timeout: {{ $.Timeout }}
  {{- end }}
  roleGroups:
  {{- $roles := keys $.RoleGroups }}
  {{- range $roles }}
    {{- $nodes := get $.RoleGroups . }}
    {{ . }}:
    {{- range $nodes }}
      - {{ . }}
    {{- end }}
  {{- end }}
  controlPlaneEndpoint:
    domain: lb.apiservice.local
    {{- $address := ""}}
    {{- if hasKey $.RoleGroups "master" }}
        {{- $mName := index (get $.RoleGroups "master") 0 }}
        {{- range $.Hosts }}
            {{- if eq .Name $mName }}
                {{- $address = .InternalAddress }}
            {{- end }}
        {{- end }}
    {{- end }}
    {{- if eq $address "" }}
        {{- failed "require control address." }}
    {{- end }}
    address: {{ $address }}
    port: 6443
  kubernetes:
    nodelocaldns: false
    dnsDomain: cluster.local
    version: {{ $.Version }}
    clusterName: {{ $.Name }}
    {{- $criType := "containerd" }}
    nodeCidrMaskSize: 24
    proxyMode: ipvs
    {{- if eq $criType "containerd" }}
    containerRuntimeEndpoint: "unix:///run/containerd/containerd.sock"
    {{- end }}
  {{- if $.CRIType }}
  {{ $criType = $.CRIType }}
  {{- end }}
    containerManager: {{ $criType }}
  etcd:
    backupDir: /var/backups/kube_etcd
    backupPeriod: 1800
    keepBackupNumber: 6
    backupScript: /usr/local/bin/kube-scripts
    {{- if hasKey $.RoleGroups "etcd" }}
    type: kubekey
    {{- else }}
    type: kubeadm
    {{- end }}
  network:
    plugin: cilium
    kubePodsCIDR: 10.233.64.0/18
    kubeServiceCIDR: 10.233.0.0/18