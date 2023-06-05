options: {
	backupName:      string
	containerName:   string
	namespace:       string
	image:           string
	imagePullPolicy: string
}

container: {
	image:           options.image
	name:            options.containerName
	imagePullPolicy: options.imagePullPolicy
	command: ["sh", "-c"]
	args: [
		"""
retryTimes=0
oldBackupInfo=
while true; do
  sleep 3;
  if [ ! -f ${BACKUP_INFO_FILE} ]; then
    continue
  fi
  backupInfo=$(cat ${BACKUP_INFO_FILE})
  if [ "${oldBackupInfo}" == "${backupInfo}" ]; then
    continue
  fi
  echo "start to patch backupInfo: ${backupInfo}"
  eval kubectl -n \(options.namespace) patch backup \(options.backupName) --subresource=status --type=merge --patch '{\\\"status\\\":${backupInfo}}'
  if [ $? -ne 0 ]; then
    retryTimes=$(($retryTimes+1))
  else
    echo "update backup status successfully"
    retryTimes=0
    oldBackupInfo=${backupInfo}
  fi
  if [ $retryTimes -ge 3 ]; then
    echo "ERROR: update backup status failed, 3 attempts have been made!"
    exit 1
  fi
done
""",
	]
}
