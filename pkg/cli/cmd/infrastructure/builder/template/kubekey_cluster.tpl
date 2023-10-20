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
    user: {{ $.User.Name }}
    password: {{ $.User.Password }}
    privateKey: {{ $.User.PrivateKey | quote }}
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
    domain: {{ $.Kubernetes.ControlPlaneEndpoint.Domain }}
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
    port: {{ $.Kubernetes.ControlPlaneEndpoint.Port }}
  kubernetes:
    nodelocaldns: false
    dnsDomain: {{ $.Kubernetes.Networking.DNSDomain }}
    version: {{ $.Version }}
    clusterName: {{ $.Name }}
    {{- $criType := "containerd" }}
    nodeCidrMaskSize: 24
    proxyMode: {{ $.Kubernetes.ProxyMode }}
    containerRuntimeEndpoint: {{ $.Kubernetes.CRI.ContainerRuntimeEndpoint }}
    containerManager: {{ $.Kubernetes.CRI.ContainerRuntimeType }}
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
    plugin: {{ $.Kubernetes.Networking.Plugin }}
    kubePodsCIDR: {{ $.Kubernetes.Networking.PodSubnet }}
    kubeServiceCIDR: {{ $.Kubernetes.Networking.ServiceSubnet }}