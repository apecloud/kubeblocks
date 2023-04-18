# mongod.conf
# for documentation of all options, see:
#   http://docs.mongodb.org/manual/reference/configuration-options/

{{- $log_root := getVolumePathByName ( index $.podSpec.containers 0 ) "log" }}
{{- $mongodb_root := getVolumePathByName ( index $.podSpec.containers 0 ) "data" }}
{{- $mongodb_port_info := getPortByName ( index $.podSpec.containers 0 ) "mongodb" }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}

# require port
{{- $mongodb_port := 27017 }}
{{- if $mongodb_port_info }}
{{- $mongodb_port = $mongodb_port_info.containerPort }}
{{- end }}

# where and how to store data.
storage:
  dbPath: {{ $mongodb_root }}/db
  journal:
    enabled: true
  directoryPerDB: true

# where to write logging data.
{{ block "logsBlock" . }}
systemLog:
  destination: file
  quiet: false
  logAppend: true
  logRotate: reopen
  path: /data/mongodb/logs/mongodb.log
  verbosity: 0
{{ end }}

# network interfaces
net:
  port: {{ $mongodb_port }}
  unixDomainSocket:
    enabled: false
    pathPrefix: {{ $mongodb_root }}/tmp
  ipv6: false
  bindIpAll: true
  #bindIp:

# replica set options
replication:
  replSetName: replicaset
  enableMajorityReadConcern: true

# sharding options
#sharding:
  #clusterRole:

# process management options
processManagement:
   fork: false
   pidFilePath: {{ $mongodb_root }}/tmp/mongodb.pid

# set parameter options
setParameter:
   enableLocalhostAuthBypass: true

# security options
security:
  authorization: enabled
  keyFile: /etc/mongodb/keyfile
