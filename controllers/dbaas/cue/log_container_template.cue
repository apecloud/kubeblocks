logContainer: {
	"name":  "logName"
	"image": "fluent/fluent-bit:1.8"
	"command": [
		"/bin/bash",
		"-c",
	]
	"args": [
		"mkdir -p /fluent-bit/conf && touch /fluent-bit/conf/fluent-bit.conf;   mkdir -p /var/log/fluent-bit && touch /var/log/fluent-bit/{tail.db,fluent-bit.log};      echo -e \"[SERVICE] \n    Flush        1 \n    Daemon       off \n    Log_File     /var/log/fluent-bit/fluent-bit.log \n[INPUT] \n    Name tail \n    Path \\${FILE_PATH} \n    DB /var/log/fluent-bit/tail.db \n[OUTPUT] \n    Name  stdout \n    Match * \n    Format json_lines \n    Json_date_key   collect_time \n    Json_date_format  iso8601\" > /fluent-bit/conf/fluent-bit.conf; /fluent-bit/bin/fluent-bit -c /fluent-bit/conf/fluent-bit.conf",
	]
	"env": [
		{
			"name":  "Key"
			"value": "Value"
		},
	]
	"resources": {
		"limits": {
			"cpu":    "100m"
			"memory": "100Mi"
		}
		"requests": {
			"cpu":    "50m"
			"memory": "10Mi"
		}
	}
	"volumeMounts": [
		{
			"name":      "pos"
			"mountPath": "/var/log"
		},
	]
	"imagePullPolicy": "IfNotPresent"
}

posVolume: {
	"name": "pos"
	"hostPath": {
		"path": "/var/log"
	}
}
