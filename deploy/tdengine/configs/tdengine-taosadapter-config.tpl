debug = false
taosConfigDir = ""
port = 6041
logLevel = "info"

[cors]
allowAllOrigins = true

[pool]
maxConnect = 4000
maxIdle = 4000
idleTimeout = "1h"

[ssl]
enable = false
certFile = ""
keyFile = ""

[log]
path = "/var/log/taos"
rotationCount = 30
rotationTime = "24h"
rotationSize = "1GB"
enableRecordHttpSql = false
sqlRotationCount = 2
sqlRotationTime =  "24h"
sqlRotationSize = "1GB"

[monitor]
collectDuration = "3s"
incgroup = false
pauseQueryMemoryThreshold = 70
pauseAllMemoryThreshold = 80
identity = ""
writeToTD = false
user = "root"
password = "taosdata"
writeInterval = "30s"

[prometheus]
enable = true

[opentsdb]
enable = true

[influxdb]
enable = true

[statsd]
enable = false
port = 6044
db = "statsd"
user = "root"
password = "taosdata"
worker = 10
gatherInterval = "5s"
protocol = "udp"
maxTCPConnections = 250
tcpKeepAlive = false
allowPendingMessages = 50000
deleteCounters = true
deleteGauges = true
deleteSets = true
deleteTimings = true

[collectd]
enable = false
port = 6045
db = "collectd"
user = "root"
password = "taosdata"
worker = 10


[opentsdb_telnet]
enable = false
maxTCPConnections = 250
tcpKeepAlive = false
dbs = ["opentsdb_telnet", "collectd", "icinga2", "tcollector"]
ports = [6046, 6047, 6048, 6049]
user = "root"
password = "taosdata"
batchSize = 1
flushInterval = "0s"

[node_exporter]
enable = false
db = "node_exporter"
user = "root"
password = "taosdata"
urls = ["http://localhost:9100"]
responseTimeout = "5s"
httpUsername = ""
httpPassword = ""
httpBearerTokenString = ""
caCertFile = ""
certFile = ""
keyFile = ""
insecureSkipVerify = true
gatherDuration = "5s"
